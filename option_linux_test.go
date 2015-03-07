package gatt

import (
	"bytes"

	"github.com/paypal/gatt/linux/cmd"
)

func ExampleLnxDeviceID() {
	NewDevice(LnxDeviceID(-1, true)) // Can only be used with NewDevice.
}

func ExampleLnxMaxConnections() {
	NewDevice(LnxMaxConnections(1)) // Can only be used with NewDevice.
}

func ExampleLnxSetAdvertisingEnable() {
	d, _ := NewDevice()
	d.Option(LnxSetAdvertisingEnable(true)) // Can only be used with Option.
}

func ExampleSetAdvertisingData() {
	// Manually crafting an advertising packet with a type field, and a service uuid - 0xFE01.
	o := LnxSetAdvertisingData(&cmd.LESetAdvertisingData{
		AdvertisingDataLength: 6,
		AdvertisingData:       [31]byte{0x02, 0x01, 0x06, 0x03, 0x01, 0xFE},
	})
	d, _ := NewDevice(o) // Can be used with NewDevice.
	d.Option(o)          // Or dynamically with Option.
}

func ExampleLnxSetScanResponseData() {
	// Manually crafting a scan response data packet with a name field.
	o := LnxSetScanResponseData(&cmd.LESetScanResponseData{
		ScanResponseDataLength: 8,
		ScanResponseData:       [31]byte{0x07, 0x09, 'G', 'o', 'p', 'h', 'e', 'r'},
	})
	d, _ := NewDevice(o)
	d.Option(o)
}

func ExampleLnxSetAdvertisingParameters() {
	o := LnxSetAdvertisingParameters(&cmd.LESetAdvertisingParameters{
		AdvertisingIntervalMin:  0x800,     // [0x0800]: 0.625 ms * 0x0800 = 1280.0 ms
		AdvertisingIntervalMax:  0x800,     // [0x0800]: 0.625 ms * 0x0800 = 1280.0 ms
		AdvertisingType:         0x00,      // [0x00]: ADV_IND, 0x01: DIRECT(HIGH), 0x02: SCAN, 0x03: NONCONN, 0x04: DIRECT(LOW)
		OwnAddressType:          0x00,      // [0x00]: public, 0x01: random
		DirectAddressType:       0x00,      // [0x00]: public, 0x01: random
		DirectAddress:           [6]byte{}, // Public or Random Address of the device to be connected
		AdvertisingChannelMap:   0x7,       // [0x07] 0x01: ch37, 0x02: ch38, 0x04: ch39
		AdvertisingFilterPolicy: 0x00,
	})
	d, _ := NewDevice(o) // Can be used with NewDevice.
	d.Option(o)          // Or dynamically with Option.
}

func ExampleLnxSendHCIRawCommand_predefinedCommand() {
	// Send a predefined command of cmd package.
	c := &cmd.LESetScanResponseData{
		ScanResponseDataLength: 8,
		ScanResponseData:       [31]byte{0x07, 0x09, 'G', 'o', 'p', 'h', 'e', 'r'},
	}
	rsp := bytes.NewBuffer(nil)
	d, _ := NewDevice()
	d.Option(LnxSendHCIRawCommand(c, rsp)) // Can only be used with Option
	// Check the return status
	if rsp.Bytes()[0] != 0x00 {
		// Handle errors
	}
}

// customCmd implements cmd.CmdParam as a fake vendor command.
type customCmd struct{ ConnectionHandle uint16 }

func (c customCmd) Opcode() int { return 0xFC01 }
func (c customCmd) Len() int    { return 3 }
func (c customCmd) Marshal(b []byte) {
	b[0], b[1], b[2] = byte(c.ConnectionHandle), byte(c.ConnectionHandle>>8), 0xff
}

func ExampleLnxSendHCIRawCommand_customCommand() {
	// customCmd implements cmd.CmdParam as a fake vendor command.
	//
	//  type customCmd struct{ ConnectionHandle uint16 }
	//
	//  func (c customCmd) Opcode() int { return 0xFC01 }
	//  func (c customCmd) Len() int    { return 3 }
	//  func (c customCmd) Marshal(b []byte) {
	//  	[]byte{
	// 	 	byte(c.ConnectionHandle),
	//  		byte(c.ConnectionHandle >> 8),
	//  		0xff,
	//  	}
	//  }
	// Send a custom vendor command without checking response.
	c := &customCmd{ConnectionHandle: 0x40}
	d, _ := NewDevice()
	d.Option(LnxSendHCIRawCommand(c, nil)) // Can only be used with Option
}
