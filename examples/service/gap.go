package service

import "github.com/paypal/gatt"

// NOTE: OSX provides GAP and GATT services, and they can't be customized.
// For Linux/Embedded, however, this is something we want to fully control.
func NewGapService() *gatt.Service {
	s := gatt.NewService(gatt.AttrGAPUUID)
	s.AddCharacteristic(gatt.AttrDeviceNameUUID).SetValue([]byte("gopher"))
	s.AddCharacteristic(gatt.AttrAppearanceUUID).SetValue([]byte{0x00, 0x80})
	s.AddCharacteristic(gatt.AttrPeripheralPrivacyUUID).SetValue([]byte{0x00})
	s.AddCharacteristic(gatt.AttrReconnectionAddrUUID).SetValue([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	s.AddCharacteristic(gatt.AttrPeferredParamsUUID).SetValue([]byte{0x06, 0x00, 0x06, 0x00, 0x00, 0x00, 0xd0, 0x07})
	return s
}
