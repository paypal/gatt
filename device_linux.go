package gatt

import (
	"errors"
	"net"
	"time"

	"github.com/paypal/gatt/linux"
)

type Device struct {
	peripheralManagerHandler

	svcs    []*Service
	hci     *linux.HCI
	handles *handleRange

	maxConn int

	quit chan struct{}
	err  error

	advSvcs     []UUID
	advPkt      []byte
	scanRespPkt []byte
	mfData      []byte

	advIntMin  uint16
	advIntMax  uint16
	advChnlMap uint8
}

func NewDevice() *Device {
	d := &Device{
		advIntMin:  0x00F4, // Spec default: 0x0800
		advIntMax:  0x00F4, // Spec default: 0x0800
		advChnlMap: 7,
		maxConn:    1,

		quit: make(chan struct{}),
	}
	d.hci = linux.NewHCI(d, d.maxConn)
	d.hci.Start()
	d.Start()
	return d
}

func (d *Device) Start() error {
	go func() {
		for {
			select {
			case l2c := <-d.hci.ConnC():
				remoteAddr := BDAddr{net.HardwareAddr(l2c.Param.PeerAddress[:])}
				c := newConn(d.handles, l2c, remoteAddr)
				go func() {
					if d.Connected != nil {
						d.Connected(c)
					}
					c.loop()
					if d.Disconnected != nil {
						d.Disconnected(c)
					}
				}()
			case <-d.quit:
				d.hci.Close()
				if d.Closed != nil {
					d.Closed(d.err)
				}
				return
			}
		}
	}()
	go d.heartbeat()
	return nil
}

func (d *Device) Stop() error { return d.hci.Close() }

func (d *Device) AddService(svc *Service) *Service {
	d.svcs = append(d.svcs, svc)
	d.handles = generateHandles(d.Name, d.svcs, uint16(1)) // ble handles start at 1
	return svc
}

func (d *Device) RemoveService(svc Service) error { return notImplemented }
func (d *Device) RemoveAllServices() error        { return notImplemented }
func (d *Device) Advertise() error {
	if len(d.advPkt) == 0 {
		d.setDefaultAdvertisement(d.Name)
	}
	return d.hci.Advertise()
}
func (d *Device) StopAdvertising() error { return d.hci.StopAdvertising() }

// heartbeat monitors the status of the BLE controller
func (d *Device) heartbeat() {
	// Send d HCI command to controller periodically, if we don't get response
	// for d while, close the server to notify upper layer.
	t := time.AfterFunc(time.Second*30, func() {
		d.err = errors.New("Device does not respond")
		close(d.quit)
	})
	for _ = range time.Tick(time.Second * 10) {
		d.hci.Ping()
		t.Reset(time.Second * 30)
	}
}

func (d *Device) setDefaultAdvertisement(name string) error {
	opts := []option{}
	if len(d.scanRespPkt) == 0 {
		opts = append(opts, ScanResponsePacket(nameScanResponsePacket(name)))
	}

	if len(d.advPkt) == 0 {
		u := []UUID{}
		for _, svc := range d.svcs {
			u = append(u, svc.UUID())
		}
		ad, _ := serviceAdvertisingPacket(u)
		opts = append(opts, AdvertisingPacket(ad))
	}

	for _, opt := range opts {
		opt(d)
	}
	return d.advertiseService()
}

func (d *Device) advertiseService() error {
	d.StopAdvertising()
	defer d.Advertise()

	if err := d.hci.SetAdvertisingParameters(
		d.advIntMin,
		d.advIntMax,
		d.advChnlMap); err != nil {
		return err
	}

	if len(d.scanRespPkt) > 0 {
		// Scan response command takes exactly 31 bytes data
		// The length indicating the significant part of the data.
		data := [31]byte{}
		n := copy(data[:31], d.scanRespPkt)
		if err := d.hci.SetScanResponsePacket(uint8(n), data); err != nil {
			return err
		}
	}

	if len(d.advPkt) > 0 {
		// Advertising data command takes exactly 31 bytes data, including manufacture data.
		// The length indicating the significant part of the data.
		data := [31]byte{}
		n := copy(data[:31], append(d.advPkt, d.mfData...))
		if err := d.hci.SetAdvertisingData(uint8(n), data); err != nil {
			return err
		}
	}

	return nil
}

// Option sets the options specified.
func (d *Device) Option(opts ...option) (prev option) {
	for _, opt := range opts {
		prev = opt(d)
	}
	d.advertiseService()
	return prev
}

// AdvertisingPacket is an optional custom advertising packet.
// If nil, the advertising data will constructed to advertise
// as many services as possible. The AdvertisingPacket must be no
// longer than MaxAdvertisingPacketLength.
// If ManufacturerData is also set, their total length must be no
// longer than MaxAdvertisingPacketLength.
func AdvertisingPacket(b []byte) option {
	return func(d *Device) option {
		prev := d.advPkt
		d.advPkt = b
		return AdvertisingPacket(prev)
	}
}

// ScanResponsePacket is an optional custom scan response packet.
// If nil, the scan response packet will set to return the server
// name, truncated if necessary. The ScanResponsePacket must be no
// longer than MaxAdvertisingPacketLength.
func ScanResponsePacket(b []byte) option {
	return func(d *Device) option {
		prev := d.scanRespPkt
		d.scanRespPkt = b
		return ScanResponsePacket(prev)
	}
}

// ManufacturerData is an optional custom data.
// If set, it will be appended in the advertising data.
// The length of AdvertisingPacket ManufactureData must be no longer
// than MaxAdvertisingPacketLength .
func ManufacturerData(b []byte) option {
	return func(d *Device) option {
		prev := d.mfData
		d.mfData = b
		return ManufacturerData(prev)
	}
}

// AdvertisingIntervalMin is an optional parameter.
// If set, it overrides the default minimum advertising interval for
// undirected and low duty cycle directed advertising.
func AdvertisingIntervalMin(n uint16) option {
	return func(d *Device) option {
		prev := d.advIntMin
		d.advIntMin = n
		return AdvertisingIntervalMin(prev)
	}
}

// AdvertisingIntervalMax is an optional parameter.
// If set, it overrides the default maximum advertising interval for
// undirected and low duty cycle directed advertising.
func AdvertisingIntervalMax(n uint16) option {
	return func(d *Device) option {
		prev := d.advIntMax
		d.advIntMax = n
		return AdvertisingIntervalMax(prev)
	}
}

// AdvertisingChannelMap is an optional parameter.
// If set, it overrides the default advertising channel map.
func AdvertisingChannelMap(n uint8) option {
	return func(d *Device) option {
		prev := d.advChnlMap
		d.advChnlMap = n
		return AdvertisingChannelMap(prev)
	}
}

// AdvertisingServices is an optional parameter.
// If set, it overrides the default advertising services.
func AdvertisingServices(u []UUID) option {
	return func(d *Device) option {
		prev := d.advSvcs
		d.advSvcs = u
		d.advPkt, _ = serviceAdvertisingPacket(u)
		return AdvertisingServices(prev)
	}
}
