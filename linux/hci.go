package linux

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type HCI struct {
	AcceptMasterHandler  func(l2c io.ReadWriteCloser, addr net.HardwareAddr)
	AcceptSlaveHandler   func(l2c io.ReadWriteCloser, pd *PlatData)
	AdvertisementHandler func(pd *PlatData)

	d io.ReadWriteCloser
	c *cmd
	e *event
	l *l2cap

	plist   map[bdaddr]*PlatData
	plistmu *sync.Mutex
}

type bdaddr [6]byte

type PlatData struct {
	Name        string
	AddressType uint8
	Address     [6]byte
	Data        []byte
	Connectable bool
	RSSI        int8

	conn io.ReadWriteCloser
	ts   time.Time
}

func NewHCI(maxConn int) (*HCI, error) {
	d, err := newSocket(1)
	if err != nil {
		d, err = newSocket(0)
		if err != nil {
			return nil, err
		}
	}
	c := newCmd(d)
	l := newL2CAP(maxConn)
	e := newEvent()

	e.handleEvent(leMeta, handlerFunc(l.handleLEMeta))
	e.handleEvent(disconnectionComplete, handlerFunc(l.handleDisconnectionComplete))
	e.handleEvent(numberOfCompletedPkts, handlerFunc(l.handleNumberOfCompletedPkts))
	e.handleEvent(commandComplete, handlerFunc(c.handleComplete))
	e.handleEvent(commandStatus, handlerFunc(c.handleStatus))

	h := &HCI{
		d:       d,
		c:       c,
		e:       e,
		l:       l,
		plist:   make(map[bdaddr]*PlatData),
		plistmu: &sync.Mutex{},
	}
	l.hci = h

	go h.mainLoop()
	h.resetDevice()
	return h, nil
}

func (h *HCI) Close() error { return h.d.Close() }

func (h *HCI) Advertise() error {
	return h.c.sendAndCheckResp(leSetAdvertiseEnable{advertisingEnable: 1}, []byte{0x00})
}

func (h *HCI) StopAdvertising() error {
	return h.c.sendAndCheckResp(leSetAdvertiseEnable{advertisingEnable: 0}, []byte{0x00})
}

func (h *HCI) SetAdvertisingParameters(intMin, intMax uint16, chnlMap uint8) error {
	return h.c.sendAndCheckResp(
		leSetAdvertisingParameters{
			advertisingIntervalMin: intMin,
			advertisingIntervalMax: intMax,
			advertisingChannelMap:  chnlMap,
		}, []byte{0x00})
}

func (h *HCI) SetScanResponsePacket(n uint8, data [31]byte) error {
	return h.c.sendAndCheckResp(
		leSetScanResponseData{
			scanResponseDataLength: n,
			scanResponseData:       data,
		}, []byte{0x00})
}

func (h *HCI) SetAdvertisingData(n uint8, data [31]byte) error {
	return h.c.sendAndCheckResp(
		leSetAdvertisingData{
			advertisingDataLength: n,
			advertisingData:       data,
		}, []byte{0x00})
}

func (h *HCI) Ping() error { return h.c.sendAndCheckResp(leReadBufferSize{}, []byte{0x00}) }

func (h *HCI) Scan() error {
	// TODO: move to a separate function.
	h.c.sendAndCheckResp(
		leSetScanParameters{
			leScanType:           0x01,   // [0x00]: passive, 0x01: active
			leScanInterval:       0x0010, // [0x10]: 0.625ms * 16
			leScanWindow:         0x0010, // [0x10]: 0.625ms *16
			ownAddressType:       0x00,   // [0x00]: public, 0x01: random
			scanningFilterPolicy: 0x00,   // [0x00]: accept all, 0x01: ignore non-white-listed.
		}, []byte{0x00})

	return h.c.sendAndCheckResp(
		leSetScanEnable{
			leScanEnable:     1,
			filterDuplicates: 1,
		}, []byte{0x00})
}

func (h *HCI) StopScan() error {
	return h.c.sendAndCheckResp(
		leSetScanEnable{
			leScanEnable:     0,
			filterDuplicates: 1,
		}, []byte{0x00})
}

func (h *HCI) Connect(pd *PlatData) error {
	h.c.send(
		leCreateConn{
			leScanInterval:        0x0004,     // N x 0.625ms
			leScanWindow:          0x0004,     // N x 0.625ms
			initiatorFilterPolicy: 0x00,       // white list not used
			peerAddressType:       0x00,       // public
			peerAddress:           pd.Address, //
			ownAddressType:        0x00,       // public
			connIntervalMin:       0x0006,     // N x 0.125ms
			connIntervalMax:       0x0006,     // N x 0.125ms
			connLatency:           0x0000,     //
			supervisionTimeout:    0x000A,     // N x 10ms
			minimumCELength:       0x0000,     // N x 0.625ms
			maximumCELength:       0x0000,     // N x 0.625ms
		})
	return nil
}

func (h *HCI) CancelConnection(pd *PlatData) error {
	return pd.conn.Close()
}

func (h *HCI) mainLoop() {
	b := make([]byte, 4096)
	for {
		n, err := h.d.Read(b)
		if err != nil {
			return
		}
		if n == 0 {
			return
		}
		p := make([]byte, n)
		copy(p, b)
		go h.handlePacket(p)
	}
}

func (h *HCI) handlePacket(b []byte) {
	t, b := packetType(b[0]), b[1:]
	var err error
	switch t {
	case typCommandPkt:
		op := uint16(b[0]) | uint16(b[1])<<8
		log.Printf("unmanaged cmd: %s(0x%04X)\n", opcode(op), op)
	case typACLDataPkt:
		err = h.l.handleL2CAP(b)
	case typSCODataPkt:
		err = fmt.Errorf("SCO packet not supported")
	case typEventPkt:
		err = h.e.dispatch(b)
	case typVendorPkt:
		err = fmt.Errorf("Vendor packet not supported")
	default:
		log.Fatalf("Unknown event: 0x%02X [ % X ]\n", t, b)
	}
	if err != nil {
		log.Printf("hci: %s, [ % X]", err, b)
	}
}

func (h *HCI) resetDevice() error {
	seq := []cmdParam{
		reset{},
		setEventMask{eventMask: 0x3dbff807fffbffff},
		leSetEventMask{leEventMask: 0x000000000000001F},
		writeSimplePairingMode{simplePairingMode: 1},
		writeLEHostSupported{leSupportedHost: 1, simultaneousLEHost: 0},
		writeInquiryMode{inquiryMode: 2},
		writePageScanType{pageScanType: 1},
		writeInquiryScanType{scanType: 1},
		writeClassOfDevice{classOfDevice: [3]byte{0x40, 0x02, 0x04}},
		writePageTimeout{pageTimeout: 0x2000},
		writeDefaultLinkPolicy{defaultLinkPolicySettings: 0x5},
		hostBufferSize{
			hostACLDataPacketLength:            0x1000,
			hostSynchronousDataPacketLength:    0xff,
			hostTotalNumACLDataPackets:         0x0014,
			hostTotalNumSynchronousDataPackets: 0x000a},
	}
	for _, s := range seq {
		if err := h.c.sendAndCheckResp(s, []byte{0x00}); err != nil {
			return err
		}
	}
	return nil
}

func (h *HCI) handleAdvertisement(b []byte) {
	// If no one is interested, don't bother.
	if h.AdvertisementHandler == nil {
		return
	}
	ep := &leAdvertisingReportEP{}
	if err := ep.unmarshal(b); err != nil {
		return
	}
	for i := 0; i < int(ep.numReports); i++ {
		addr := bdaddr(ep.address[i])
		et := ep.eventType[i]
		connectable := et == advInd || et == advDirectInd
		scannable := et == advInd || et == advScanInd

		if et == scanRsp {
			h.plistmu.Lock()
			pd, ok := h.plist[addr]
			h.plistmu.Unlock()
			if ok {
				pd.Data = append(pd.Data, ep.data[i]...)
				h.AdvertisementHandler(pd)
			}
			continue
		}

		pd := &PlatData{
			AddressType: ep.addressType[i],
			Address:     ep.address[i],
			Data:        ep.data[i],
			Connectable: connectable,
			RSSI:        ep.rssi[i],
			ts:          time.Now(),
		}
		h.plistmu.Lock()
		h.plist[addr] = pd
		h.plistmu.Unlock()
		if scannable {
			continue
		}
		h.AdvertisementHandler(pd)
	}
}

func (h *HCI) handleConnection(c io.ReadWriteCloser, addr bdaddr, master bool) {
	if master {
		if h.AcceptMasterHandler == nil {
			return
		}
		h.AcceptMasterHandler(c, net.HardwareAddr(addr[:]))
		return
	}
	if h.AcceptSlaveHandler == nil {
		return
	}
	h.plistmu.Lock()
	pd := h.plist[addr]
	h.plistmu.Unlock()
	pd.conn = c
	h.AcceptSlaveHandler(c, pd)
}
