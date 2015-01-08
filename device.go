package gatt

import "errors"

var notImplemented = errors.New("not implemented")

type option func(*Device) option

type peripheralManagerHandler struct {
	// Name is the device name, exposed via the Generic Access Service (0x1800).
	Name string

	// Connect is called when a device connects to the server.
	Connected func(c Conn)

	// Disconnect is called when a device disconnects from the server.
	Disconnected func(c Conn)

	// ReceiveRSSI is called when an RSSI measurement is received for a connection.
	ReceiveRSSI func(c Conn, rssi int)

	// Closed is called when a server is closed.
	// err will be any associated error.
	// If the server was closed by calling Close, err may be nil.
	Closed func(error)

	// StateChange is called when the server changes states.
	// TODO: Break these states out into separate, meaningful methods?
	// TODO: Document the set of states.
	StateChange func(newState string)

	// TODO: Helper function to construct iBeacon advertising packet.
	// See e.g. http://stackoverflow.com/questions/18906988.
}

type centralManagerHandler struct {
	PeripheralConnected          func(*Peripheral)
	PeripheralDisconnected       func(*Peripheral, error)
	PeripheralFailToConnect      func(*Peripheral, error)
	PeripheralDiscovered         func(*Peripheral)
	RetrieveConnectedPeripherals func([]*Peripheral)
	RetrievePeripherals          func([]*Peripheral)
}
