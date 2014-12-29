package gatt

import "net"

// A BDAddr (Bluetooth Device Address) is a hardware-addressed-based net.Addr.
type BDAddr struct{ net.HardwareAddr }

func (a BDAddr) Network() string { return "BLE" }

type Conn interface {
	// LocalAddr returns the address of the connected device (central).
	LocalAddr() BDAddr

	// LocalAddr returns the address of the local device (peripheral).
	RemoteAddr() BDAddr

	// Close disconnects the connection.
	Close() error

	// RSSI returns the last RSSI measurement, or -1 if there have not been any.
	RSSI() int

	// UpdateRSSI requests an RSSI update and blocks until one has been received.
	// TODO: Implement.
	UpdateRSSI() (rssi int, err error)

	// MTU returns the current connection mtu.
	MTU() int
}
