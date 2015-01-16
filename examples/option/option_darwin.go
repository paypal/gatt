package option

import "github.com/paypal/gatt"

var DefaultClientOptions = []gatt.Option{
	gatt.MacDeviceRole(gatt.CentrallManager),
}

var DefaultServerOptions = []gatt.Option{
	gatt.MacDeviceRole(gatt.PeripheralManager),
}
