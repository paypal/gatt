package gatt

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
