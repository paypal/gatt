package gatt

const (
	CentrallManager   = 0 // Default
	PeripheralManager = 1
)

// MacDeviceRole specify the XPC connection type to connect blued.
// THis option can only be used with NewDevice on Mac implementation.
// CentralManager (client functions)
// PeripheralManager (server functions)
func MacDeviceRole(r int) Option {
	return func(d Device) { d.(*device).role = r }
}
