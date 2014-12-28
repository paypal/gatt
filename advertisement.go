package gatt

import (
	"errors"
	"log"
)

// TODO: The whole file needs to be refined, merge the advPkt into the Advertisement.
// And flesh out the implementation details.

// MaxEIRPacketLength is the maximum allowed AdvertisingPacket
// and ScanResponsePacket length.
const MaxEIRPacketLength = 31

// ErrEIRPacketTooLong is the error returned when an AdvertisingPacket
// or ScanResponsePacket is too long.
var ErrEIRPacketTooLong = errors.New("max packet length is 31")

// advertising data field types
// TODO: refine naming, check spelling.
const (
	typeFlags             = 0x01 // Flags
	typeSomeUUID16        = 0x02 // Incomplete List of 16-bit Service Class UUIDs
	typeAllUUID16         = 0x03 // Complete List of 16-bit Service Class UUIDs
	typeSomeUUID32        = 0x04 // Incomplete List of 32-bit Service Class UUIDs
	typeAllUUID32         = 0x05 // Complete List of 32-bit Service Class UUIDs
	typeSomeUUID128       = 0x06 // Incomplete List of 128-bit Service Class UUIDs
	typeAllUUID128        = 0x07 // Complete List of 128-bit Service Class UUIDs
	typeShortName         = 0x08 // Shortened Local Name
	typeCompleteName      = 0x09 // Complete Local Name
	typeTxPower           = 0x0A // Tx Power Level
	typeClassOfDevice     = 0x0D // Class of Device
	typeSimplePairingC192 = 0x0E // Simple Pairing Hash C-192
	typeSimplePairingR192 = 0x0F // Simple Pairing Randomizer R-192
	typeSecManagerTK      = 0x10 // Security Manager TK Value
	typeSecManagerOOB     = 0x11 // Security Manager Out of Band Flags
	typeSlaveConnInt      = 0x12 // Slave Connection Interval Range
	typeServiceSol16      = 0x14 // List of 16-bit Service Solicitation UUIDs
	typeServiceSol128     = 0x15 // List of 128-bit Service Solicitation UUIDs
	typeServiceData16     = 0x16 // Service Data - 16-bit UUID
	typePubTargetAddr     = 0x17 // Public Target Address
	typeRandTargetAddr    = 0x18 // Random Target Address
	typeAppearance        = 0x19 // Appearance
	typeAdvInterval       = 0x1A // Advertising Interval
	typeLEDeviceAddr      = 0x1B // LE Bluetooth Device Address
	typeLERole            = 0x1C // LE Role
	typeServiceSol32      = 0x1F // List of 32-bit Service Solicitation UUIDs
	typeServiceData32     = 0x20 // Service Data - 32-bit UUID
	typeServiceData128    = 0x21 // Service Data - 128-bit UUID
	typeLESecConfirm      = 0x22 // LE Secure Connections Confirmation Value
	typeLESecRandom       = 0x23 // LE Secure Connections Random Value
	typeManufacturerData  = 0xFF // Manufacturer Specific Data
)

// flag bits
const (
	flagLimitedDiscoverable = 1 << iota // LE Limited Discoverable Mode
	flagGeneralDiscoverable             // LE General Discoverable Mode
	flagLEOnly                          // BR/EDR Not Supported. Bit 37 of LMP Feature Mask Definitions (Page 0)
	flagBothController                  // Simultaneous LE and BR/EDR to Same Device Capable (Controller).
	flagBothHost                        // Simultaneous LE and BR/EDR to Same Device Capable (Host).
)

// TODO: this is borrowed from core bluetooth.
// Embedded folks might be interested in more details.
// Should be done with a more flexible way without losing platform generality.
type Advertisement struct {
	LocalName        string
	ManufacturerData []byte
	ServiceData      []byte
	Services         []UUID
	OverflowService  []UUID
	TxPowerLevel     int
	Connectable      bool
	SolicitedService []UUID
}

// TODO:
// func (a *Advertisement) Marshall() ([]byte, error) {
// }

// FIXME: this serves well as a placeholder. Need to check the correctness and refine it.
func (a *Advertisement) Unmarshall(b []byte) error {
	for len(b) > 0 {
		if len(b) < 2 {
			return errors.New("invalid advertise data")
		}
		l, t := b[0], b[1]
		if len(b) < int(1+l) {
			return errors.New("invalid advertise data")
		}
		d := b[2 : 1+l]
		switch t {
		case typeFlags:
			a.Connectable = d[0] == 0x01 || d[0] == 0x02
		case typeSomeUUID16:
			a.Services = uuidList(a.Services, d, 2)
		case typeAllUUID16:
			a.Services = uuidList(a.Services, d, 2)
		case typeSomeUUID32:
			a.Services = uuidList(a.Services, d, 4)
		case typeAllUUID32:
			a.Services = uuidList(a.Services, d, 4)
		case typeSomeUUID128:
			a.Services = uuidList(a.Services, d, 16)
		case typeAllUUID128:
			a.Services = uuidList(a.Services, d, 16)
		case typeShortName:
			a.LocalName = string(d)
		case typeCompleteName:
			a.LocalName = string(d)
		case typeTxPower:
			a.TxPowerLevel = int(d[0])
		case typeServiceSol16:
			a.SolicitedService = uuidList(a.SolicitedService, d, 2)
		case typeServiceSol128:
			a.SolicitedService = uuidList(a.SolicitedService, d, 16)
		case typeServiceData16:
			a.ServiceData = make([]byte, len(d))
			copy(a.ServiceData, d)
		case typeServiceSol32:
			a.SolicitedService = uuidList(a.SolicitedService, d, 4)
		case typeServiceData32:
			a.ServiceData = make([]byte, len(d))
			copy(a.ServiceData, d)
		case typeServiceData128:
			a.ServiceData = make([]byte, len(d))
			copy(a.ServiceData, d)
		case typeManufacturerData:
			a.ManufacturerData = make([]byte, len(d))
			copy(a.ManufacturerData, d)
		default:
			log.Printf("DATA: [ % X ]", d)
		}
		b = b[1+l:]
	}
	return nil
}

func uuidList(u []UUID, d []byte, w int) []UUID {
	for len(d) > 0 {
		u = append(u, UUID{d[:w]})
		d = d[w:]
	}
	return u
}

// nameScanResponsePacket constructs a scan response packet with
// the given name, truncated as necessary.
func nameScanResponsePacket(name string) []byte {
	typ := byte(typeCompleteName)
	if max := MaxEIRPacketLength - 2; len(name) > max {
		name = name[:max]
		typ = typeShortName
	}
	scan := new(advPacket)
	scan.appendField(typ, []byte(name))
	return scan.data
}

// serviceAdvertisingPacket constructs an advertising packet that
// advertises as many of the provided service uuids as possible.
// It returns the advertising packet and the contained uuids.
// Most clients do not need to call serviceAdvertisingPacket; the
// server will automatically advertise as many of its services as possible.
func serviceAdvertisingPacket(uu []UUID) ([]byte, []UUID) {
	fit := make([]UUID, 0, len(uu))
	adv := new(advPacket)
	adv.appendField(typeFlags, []byte{flagGeneralDiscoverable | flagLEOnly})
	for _, u := range uu {
		if ok := adv.appendUUIDFit(u); ok {
			fit = append(fit, u)
		}
	}
	return adv.data, fit
}

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
	p.appendField(typeManufacturerData, d)
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
