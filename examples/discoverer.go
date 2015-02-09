// +build

package main

import (
	"fmt"

	"github.com/paypal/gatt"
	"github.com/paypal/gatt/examples/option"
)

func stateChanged(d gatt.Device, s gatt.State) {
	fmt.Println("State:", s)
	switch s {
	case gatt.StatePoweredOn:
		fmt.Println("scaning...")
		d.Scan([]gatt.UUID{}, false)
		return
	default:
		d.StopScanning()
	}
}

func periphDiscovered(p gatt.Peripheral, a *gatt.Advertisement, rssi int) {
	fmt.Printf("\nPeripheral ID:%s, NAME:(%s)\n", p.ID(), p.Name())
	fmt.Println("  Local Name        =", a.LocalName)
	fmt.Println("  TX Power Level    =", a.TxPowerLevel)
	fmt.Println("  Manufacturer Data =", a.ManufacturerData)
	fmt.Println("  Service Data      =", a.ServiceData)
}

func main() {
	d, err := gatt.NewDevice(option.DefaultClientOptions...)
	if err != nil {
		fmt.Printf("Failed to open device, err: %s", err)
		return
	}

	d.Handle(
		gatt.PeripheralDiscovered(periphDiscovered),
	)
	d.Init(stateChanged)
	select {}
}
