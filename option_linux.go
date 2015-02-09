package gatt

// LnxAdvertisingPacket is an optional custom advertising packet.
// If nil, the advertising data will constructed to advertise
// as many services as possible. The AdvertisingPacket must be no
// longer than MaxAdvertisingPacketLength.
// If ManufacturerData is also set, their total length must be no
// longer than MaxAdvertisingPacketLength.
func LnxAdvertisingPacket(b []byte) Option {
	return func(d Device) { d.(*device).advPkt = b }
}

// LnxScanResponsePacket is an optional custom scan response packet.
// If nil, the scan response packet will set to return the server
// name, truncated if necessary. The ScanResponsePacket must be no
// longer than MaxAdvertisingPacketLength.
func LnxScanResponsePacket(b []byte) Option {
	return func(d Device) { d.(*device).scanRespPkt = b }
}

// LnxManufacturerData is an optional custom data.
// If set, it will be appended in the advertising data.
// The length of AdvertisingPacket ManufactureData must be no longer
// than MaxAdvertisingPacketLength .
func LnxManufacturerData(b []byte) Option {
	return func(d Device) { d.(*device).advMfData = b }
}

// LnxAdvertisingIntervalMin is an optional parameter.
// If set, it overrides the default minimum advertising interval for
// undirected and low duty cycle directed advertising.
func LnxAdvertisingIntervalMin(n uint16) Option {
	return func(d Device) { d.(*device).advIntMin = n }
}

// LnxAdvertisingIntervalMax is an optional parameter.
// If set, it overrides the default maximum advertising interval for
// undirected and low duty cycle directed advertising.
func LnxAdvertisingIntervalMax(n uint16) Option {
	return func(d Device) { d.(*device).advIntMax = n }
}

// LnxAdvertisingChannelMap is an optional parameter.
// If set, it overrides the default advertising channel map.
func LnxAdvertisingChannelMap(n uint8) Option {
	return func(d Device) { d.(*device).advChnlMap = n }
}

// LnxDeviceID specifies which HCI device to use.
// This option can only be used with NewDevice on Linux implementation.
// If n is set to -1, we'll try all the available HCI devices.
// If chk is set to true, an additional check for LE support in device's
// feature list be checked. This is to filter devices that does not
// support LE. However, in case some LE driver that doesn't set the LE
// support in its feature list, user can turn off the check.
func LnxDeviceID(n int, chk bool) Option {
	return func(d Device) {
		d.(*device).devID = n
		d.(*device).chkLE = chk
	}
}

// LnxMaxConnections is an optional parameter.
// If set, it overrides the default max connections supported.
// This option can only be used with NewDevice on Linux implementation.
func LnxMaxConnections(n int) Option {
	return func(d Device) { d.(*device).maxConn = n }
}
