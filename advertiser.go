package gatt

import (
	"errors"
	"sync"

	"github.com/roylee17/hci"
	"github.com/roylee17/hci/cmd"
)

// MaxEIRPacketLength is the maximum allowed AdvertisingPacket
// and ScanResponsePacket length.
const MaxEIRPacketLength = 31

// ErrEIRPacketTooLong is the error returned when an AdvertisingPacket
// or ScanResponsePacket is too long.
var ErrEIRPacketTooLong = errors.New("max packet length is 31")

type Advertiser struct {
	// AdvertisingPacket is an optional custom advertising packet.
	// If nil, the advertising packet will constructed to advertise
	// as many services as possible. AdvertisingPacket must be set,
	// if at all, before starting the server. The AdvertisingPacket
	// must be no longer than MaxAdvertisingPacketLength.
	AdvertisingPacket []byte

	// ScanResponsePacket is an optional custom scan response packet.
	// If nil, the scan response packet will set to return the server
	// name, truncated if necessary. ScanResponsePacket must be set,
	// if at all, before starting the server. The ScanResponsePacket
	// must be no longer than MaxAdvertisingPacketLength.
	ScanResponsePacket []byte

	ManufactureData []byte

	AdvertisingIntervalMin uint16
	AdvertisingIntervalMax uint16
	AdvertisingChannelMap  uint8

	serving bool

	hci *hci.HCI

	mu sync.RWMutex
}

func NewAdvertiser(h *hci.HCI) *Advertiser {
	return &Advertiser{
		AdvertisingPacket:      nil,
		ScanResponsePacket:     nil,
		AdvertisingIntervalMin: 0x00f4,
		AdvertisingIntervalMax: 0x00f4,
		AdvertisingChannelMap:  7,

		serving: false,

		hci: h,
	}
}

func (a *Advertiser) SetServing(s bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.serving = s
}

func (a *Advertiser) Serving() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.serving
}

func (a *Advertiser) Start() error {
	a.SetServing(true)
	return a.hci.Cmd().SendAndCheckResp(cmd.LESetAdvertiseEnable{AdvertisingEnable: 1}, []byte{0x00})
}

func (a *Advertiser) Stop() error {
	a.SetServing(false)
	return a.hci.Cmd().SendAndCheckResp(cmd.LESetAdvertiseEnable{AdvertisingEnable: 0}, []byte{0x00})
}

func (a *Advertiser) AdvertiseService() error {
	if a.Serving() {
		a.Stop()
		defer a.Start()
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	c := a.hci.Cmd()
	if len(a.AdvertisingPacket) > MaxEIRPacketLength || len(a.ScanResponsePacket) > MaxEIRPacketLength {
		return ErrEIRPacketTooLong
	}

	if err := c.SendAndCheckResp(
		cmd.LESetAdvertisingParameters{
			AdvertisingIntervalMin: a.AdvertisingIntervalMin,
			AdvertisingIntervalMax: a.AdvertisingIntervalMax,
			AdvertisingChannelMap:  a.AdvertisingChannelMap,
		}, []byte{0x00}); err != nil {
		return err
	}

	// Both Scan and Advertising take exactly 31 bytes data(and a length indicating the significant part of the data)
	if len(a.ScanResponsePacket) > 0 {
		data := [31]byte{}
		copy(data[:31], a.ScanResponsePacket)
		if err := c.SendAndCheckResp(
			cmd.LESetScanResponseData{
				ScanResponseDataLength: uint8(len(a.ScanResponsePacket)),
				ScanResponseData:       data,
			}, []byte{0x00}); err != nil {
			return err
		}
	}

	if len(a.AdvertisingPacket) > 0 {
		data := [31]byte{}
		copy(data[:31], append(a.AdvertisingPacket, a.ManufactureData...))
		l := len(a.AdvertisingPacket) + len(a.ManufactureData)

		if err := c.SendAndCheckResp(
			cmd.LESetAdvertisingData{
				AdvertisingDataLength: uint8(l),
				AdvertisingData:       data,
			}, []byte{0x00}); err != nil {
			return err
		}
	}
	return nil
}

// NameScanResponsePacket constructs a scan response packet with
// the given name, truncated as necessary.
func NameScanResponsePacket(name string) []byte {
	typ := byte(typeCompleteName)
	if max := MaxEIRPacketLength - 2; len(name) > max {
		name = name[:max]
		typ = typeShortName
	}
	scan := new(advPacket)
	scan.appendField(typ, []byte(name))
	return scan.data
}

// ServiceAdvertisingPacket constructs an advertising packet that
// advertises as many of the provided service uuids as possible.
// It returns the advertising packet and the contained uuids.
// Most clients do not need to call serviceAdvertisingPacket; the
// server will automatically advertise as many of its services as possible.
func ServiceAdvertisingPacket(uu []UUID) ([]byte, []UUID) {
	fit := make([]UUID, 0, len(uu))
	adv := new(advPacket)
	adv.appendField(typeFlags, []byte{flagGenerallyDiscoverable | flagLEOnly})
	for _, u := range uu {
		if ok := adv.appendUUIDFit(u); ok {
			fit = append(fit, u)
		}
	}
	return adv.data, fit
}

// advertising data field types
const (
	typeFlags           = 0x01 // flags
	typeSomeUUID16      = 0x02 // more 16-bit UUIDs available
	typeAllUUID16       = 0x03 // complete list of 16-bit UUIDs available
	typeSomeUUID32      = 0x04 // more 32-bit UUIDs available
	typeAllUUID32       = 0x05 // complete list of 32-bit UUIDs available
	typeSomeUUID128     = 0x06 // more 128-bit UUIDs available
	typeAllUUID128      = 0x07 // complete list of 128-bit UUIDs available
	typeShortName       = 0x08 // shortened local name
	typeCompleteName    = 0x09 // complete local name
	typeManufactureData = 0xFF // manufacture specific data
)

// flag bits
const (
	flagGenerallyDiscoverable = 1 << 1
	flagLEOnly                = 1 << 2
)

// TODO: tests
type advPacket struct {
	data []byte
}

// appendField appends a BLE advertising packet field.
// TODO: refuse to append field if it'd make the packet too long.
func (p *advPacket) appendField(typ byte, data []byte) {
	// A field consists of len, typ, data.
	// Len is 1 byte for typ plus len(data).
	p.data = append(p.data, byte(len(data)+1))
	p.data = append(p.data, typ)
	p.data = append(p.data, data...)
}

func (p *advPacket) appendManufactureDataFit(cid uint16, data []byte) bool {
	if len(p.data)+1+2+len(data) > MaxEIRPacketLength {
		return false
	}
	d := append([]byte{uint8(cid), uint8(cid >> 8)}, data...)
	p.appendField(typeManufactureData, d)
	return true
}

// appendUUIDFit appends a BLE advertised service UUID
// packet field if it fits in the packet, and reports
// whether the UUID fit.
func (p *advPacket) appendUUIDFit(u UUID) bool {
	if len(p.data)+u.Len()+2 > MaxEIRPacketLength {
		return false
	}
	// Err on the side of safety and assume that there might be
	// other services available: Use typeSomeUUID instead
	// of typeAllUUID.
	// TODO: When we know the full set of services,
	// calculate this exactly, instead of hedging.
	switch u.Len() {
	case 2:
		p.appendField(typeSomeUUID16, u.reverseBytes())
	case 16:
		p.appendField(typeSomeUUID128, u.reverseBytes())
	}
	return true
}
