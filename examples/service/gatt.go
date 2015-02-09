package service

import (
	"log"

	"github.com/paypal/gatt"
)

// NOTE: OSX provides GAP and GATT services, and they can't be customized.
// For Linux/Embedded, however, this is something we want to fully control.
func NewGattService() *gatt.Service {
	s := gatt.NewService(gatt.AttrGATTUUID)
	s.AddCharacteristic(gatt.AttrServiceChangedUUID).HandleNotifyFunc(
		func(r gatt.Request, n gatt.Notifier) {
			go func() {
				log.Printf("TODO: indicate client when the services are changed")
			}()
		})
	return s
}
