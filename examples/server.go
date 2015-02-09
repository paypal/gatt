// +build

package main

import (
	"fmt"
	"log"

	"github.com/paypal/gatt"
	"github.com/paypal/gatt/examples/option"
	"github.com/paypal/gatt/examples/service"
)

func main() {
	d, err := gatt.NewDevice(option.DefaultServerOptions...)
	if err != nil {
		log.Printf("Failed to open device, err: %s", err)
		return
	}

	onStateChanged := func(d gatt.Device, s gatt.State) {
		fmt.Printf("State: %s\n", s)
		switch s {
		case gatt.StatePoweredOn:
			s1 := service.NewCountService()
			s2 := service.NewBatteryService()
			d.AddService(service.NewGapService())  // no effect on OSX
			d.AddService(service.NewGattService()) // no effect on OSX
			d.AddService(s1)
			d.AddService(s2)
			d.AdvertiseNameAndServices("gopher", []gatt.UUID{s1.UUID(), s2.UUID()})
		default:
		}
	}

	d.Handle(
		gatt.CentralConnected(func(c gatt.Central) { log.Println("Connect: ", c) }),
		gatt.CentralDisconnected(func(c gatt.Central) { log.Println("Disconnect: ", c) }),
	)

	d.Init(onStateChanged)

	select {}
}
