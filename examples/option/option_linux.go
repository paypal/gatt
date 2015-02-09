package option

import "github.com/paypal/gatt"

var DefaultClientOptions = []gatt.Option{
// gatt.LnxDeviceID(0, false),
}

var DefaultServerOptions = []gatt.Option{
	// gatt.LnxDeviceID(0, false),
	gatt.LnxAdvertisingIntervalMin(0x00f4),
	gatt.LnxAdvertisingIntervalMax(0x00f4),
}
