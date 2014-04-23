package gatt

import (
	"bufio"
	"errors"
	"fmt"
	"strings"
)

func newHCI(s shim) *hci {
	c := &hci{
		shim:    s,
		readbuf: bufio.NewReader(s),
	}
	return c
}

type hci struct {
	shim
	readbuf *bufio.Reader
}

// advertiseEIR instructs hci to begin advertising. adv and scan
// must have maximum length 31.
//
// TODO: Support setting advertising params. Here's a relevant extract from
//
// http://stackoverflow.com/questions/21124993/is-there-a-way-to-increase-ble-advertisement-frequency-in-bluez
//
// sudo hcitool -i hci0 cmd 0x08 0x0006 A0 00 A0 00 03 00 00 00 00 00 00 00 00 07 00
// sudo hcitool -i hci0 cmd 0x08 0x000a 01
//
// The first hcitool command (0x08 0x0006) is "LE Set Advertising Parameters. The
// first two bytes A0 00 are the "min interval". The second two bytes A0 00 are the
// "max interval". In this example, it sets the time between advertisements to 100ms.
// The granularity of this setting is 0.625ms, so setting the interval to 01 00 sets
// the advertisement to go every 0.625ms. Setting it to A0 00 sets the advertisement
// to go every 0xA0*0.625ms = 100ms. Setting it to 40 06 sets the advertisement to go
// every 0x0640*0.625ms = 1000ms. The fifth byte, 03, sets the advertising mode to
// non-connectable. With a non-connectable advertisement, the fastest you can advertise
// is 100ms, with a connectable advertisment (0x00) you can advertise much faster.
//
// The second hcitool command (0x08 0x000a) is "LE Set Advertise Enable". It is necessary
// to issue this command with hcitool instead of hciconfig, because
// "hciconfig hci0 leadv 3" will automatically set the advertising rate to the slower
// default of 1280ms.
//
func (c *hci) advertiseEIR(adv []byte, scan []byte) error {
	switch {
	case len(adv) > MaxEIRPacketLength:
		return ErrEIRPacketTooLong
	case len(scan) > MaxEIRPacketLength:
		return ErrEIRPacketTooLong
	}
	// log.Printf("HCI: Sending %x %x", adv, scan)
	_, err := fmt.Fprintf(c.shim, "%x %x\n", adv, scan)
	return err
}

// event returns the next available HCI event, blocking if needed.
func (c *hci) event() (string, error) {
	for {
		s, err := c.readbuf.ReadString('\n')
		if err != nil {
			return "", err
		}
		f := strings.Fields(s)
		if len(f) < 2 {
			return "", errors.New("badly formed event: " + s)
		}
		switch f[0] {
		case "adapterState":
			return f[1], nil
		case "hciDeviceId":
			// log.Printf("HCI device id %s", f[1])
			continue
		default:
			return "", errors.New("unexpected event type: " + s)
		}
	}
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
	typeFlags        = 1 // flags
	typeSomeUUID16   = 2 // more 16-bit UUIDs available
	typeAllUUID16    = 3 // complete list of 16-bit UUIDs available
	typeSomeUUID32   = 4 // more 32-bit UUIDs available
	typeAllUUID32    = 5 // complete list of 32-bit UUIDs available
	typeSomeUUID128  = 6 // more 128-bit UUIDs available
	typeAllUUID128   = 7 // complete list of 128-bit UUIDs available
	typeShortName    = 8 // shortened local name
	typeCompleteName = 9 // complete local name
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
