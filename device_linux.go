package gatt

import (
	"encoding/binary"
	"time"

	"github.com/paypal/gatt/linux"
)

type device struct {
	deviceHandler

	hci   *linux.HCI
	hb    bool // heartbeat, ping HCI device periodically
	state State

	// All the following fields are only used peripheralManager (server) implementation.
	svcs  []*Service
	attrs *attrRange

	devID   int
	chkLE   bool
	maxConn int

	advPkt      []byte
	scanRespPkt []byte
	advMfData   []byte

	advIntMin  uint16
	advIntMax  uint16
	advChnlMap uint8
}

func NewDevice(opts ...Option) (Device, error) {
	d := &device{
		advIntMin:  0x0800, // Spec default: 0x0800.
		advIntMax:  0x0800, // Spec default: 0x0800.
		advChnlMap: 7,      // Broadcast to all broadcasting channels.
		maxConn:    1,      // Support 1 connection at a time.
		devID:      -1,     // Try detect an available one.
		chkLE:      true,   // Check if the device supports LE.
	}

	d.Option(opts...)
	h, err := linux.NewHCI(d.devID, d.chkLE, d.maxConn)
	if err != nil {
		return nil, err
	}

	d.hci = h
	return d, nil
}

func (d *device) Init(f func(Device, State)) error {
	d.hci.AcceptMasterHandler = func(pd *linux.PlatData) {
		c := newCentral(d, d.attrs, pd.Conn, true)
		if d.centralConnected != nil {
			d.centralConnected(c)
		}
		c.loop()
		if d.centralDisconnected != nil {
			d.centralDisconnected(c)
		}
	}
	d.hci.AcceptSlaveHandler = func(pd *linux.PlatData) {
		p := &peripheral{
			d:     d,
			pd:    pd,
			l2c:   pd.Conn,
			reqc:  make(chan message),
			quitc: make(chan struct{}),
			sub:   newSubscriber(),
		}
		if d.peripheralConnected != nil {
			go d.peripheralConnected(p, nil)
		}
		p.loop()
		if d.peripheralDisconnected != nil {
			d.peripheralDisconnected(p, nil)
		}
	}
	d.hci.AdvertisementHandler = func(pd *linux.PlatData) {
		a := &Advertisement{}
		a.Unmarshall(pd.Data)
		a.Connectable = pd.Connectable
		p := &peripheral{pd: pd, d: d}
		if d.peripheralDiscovered != nil {
			pd.Name = a.LocalName
			d.peripheralDiscovered(p, a, int(pd.RSSI))
		}
	}
	if d.hb {
		go d.heartbeat()
	}
	d.state = StatePoweredOn
	d.stateChanged = f
	go d.stateChanged(d, d.state)
	return nil
}

func (d *device) Stop() error {
	d.state = StatePoweredOff
	defer d.stateChanged(d, d.state)
	return d.hci.Close()
}

func (d *device) AddService(s *Service) error {
	d.svcs = append(d.svcs, s)
	d.attrs = generateAttributes(d.svcs, uint16(1)) // ble attrs start at 1
	return nil
}

func (d *device) RemoveAllServices() error {
	d.svcs = nil
	d.attrs = nil
	return nil
}

func (d *device) SetServices(s []*Service) error {
	d.RemoveAllServices()
	d.svcs = append(d.svcs, s...)
	d.attrs = generateAttributes(d.svcs, uint16(1)) // ble attrs start at 1
	return nil
}

func (d *device) AdvertiseNameAndServices(name string, uu []UUID) error {
	a := &advPacket{}
	a.appendField(typeFlags, []byte{flagGeneralDiscoverable | flagLEOnly})
	for _, u := range uu {
		if u.Equal(AttrGAPUUID) || u.Equal(AttrGATTUUID) {
			continue
		}
		if ok := a.appendUUIDFit(u); !ok {
			break
		}
	}
	if len(a.data)+len(name)+2 < MaxEIRPacketLength {
		a.appendName(name)
		d.scanRespPkt = nil
	} else {
		a := &advPacket{}
		d.scanRespPkt = a.appendName(name).data
	}
	d.advPkt = a.data

	return d.advertise()
}

func (d *device) AdvertiseIBeaconData(b []byte) error {
	a := &advPacket{}
	a.appendFlags(flagGeneralDiscoverable | flagLEOnly)
	a.appendManufactureData(0x004C, b)
	d.advPkt = a.data
	return d.advertise()
}

func (d *device) AdvertiseIBeacon(u UUID, major, minor uint16, pwr int8) error {
	b := make([]byte, 23)
	b[0] = 0x02                               // Data type: iBeacon
	b[1] = 0x15                               // Data length: 21 bytes
	copy(b[2:], reverse(u.b))                 // Big endian
	binary.BigEndian.PutUint16(b[18:], major) // Big endian
	binary.BigEndian.PutUint16(b[20:], minor) // Big endian
	b[22] = uint8(pwr)                        // Measured Tx Power
	return d.AdvertiseIBeaconData(b)
}

func (d *device) StopAdvertising() error {
	return d.hci.SetAdvertiseEnable(false)
}

func (d *device) Scan(ss []UUID, dup bool) {
	// TODO: filter
	d.hci.SetScanEnable(true, dup)
}

func (d *device) StopScanning() {
	d.hci.SetScanEnable(false, true)
}

func (d *device) Connect(p Peripheral) {
	d.hci.Connect(p.(*peripheral).pd)
}

func (d *device) CancelConnection(p Peripheral) {
	d.hci.CancelConnection(p.(*peripheral).pd)
}

// FIXME: this got one of my BLE dongle mad sometimes. Need to figure out why.
// heartbeat monitors the status of the BLE controller
func (d *device) heartbeat() {
	// Send a HCI command to controller periodically, if we don't get response
	// for a while, close the server to notify upper layer.
	t := time.AfterFunc(time.Second*30, func() {
		d.hci.Close()
		d.stateChanged(d, StateUnknown)
	})
	for _ = range time.Tick(time.Second * 15) {
		d.hci.Ping()
		t.Reset(time.Second * 30)
	}
}

func (d *device) advertise() error {
	d.hci.SetAdvertiseEnable(false)
	defer d.hci.SetAdvertiseEnable(true)

	if err := d.hci.SetAdvertisingParameters(
		d.advIntMin,
		d.advIntMax,
		d.advChnlMap); err != nil {
		return err
	}

	if len(d.scanRespPkt) > 0 {
		data := [31]byte{}
		n := copy(data[:31], d.scanRespPkt)
		if err := d.hci.SetScanResponsePacket(uint8(n), data); err != nil {
			return err
		}
	}

	if len(d.advPkt) > 0 {
		data := [31]byte{}
		n := copy(data[:31], append(d.advPkt, d.advMfData...))
		if err := d.hci.SetAdvertisingData(uint8(n), data); err != nil {
			return err
		}
	}

	return nil
}
