// +build

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/paypal/gatt"
	"github.com/paypal/gatt/examples/option"
)

var done = make(chan struct{})

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
	id := flag.Args()[0]
	if p.ID() != id {
		return
	}

	p.Device().StopScanning()
	fmt.Printf("\nPeripheral ID:%s, NAME:(%s)\n", p.ID(), p.Name())
	fmt.Println("  Local Name        =", a.LocalName)
	fmt.Println("  TX Power Level    =", a.TxPowerLevel)
	fmt.Println("  Manufacturer Data =", a.ManufacturerData)
	fmt.Println("  Service Data      =", a.ServiceData)

	p.Device().Connect(p)
}

func periphConnected(p gatt.Peripheral, err error) {
	fmt.Println("Connected")
	defer p.Device().CancelConnection(p)

	// Discovery services
	ss, err := p.DiscoverServices(nil)
	if err != nil {
		fmt.Printf("Failed to discover services, err: %s\n", err)
		return
	}

	for _, s := range ss {
		msg := "Service: " + s.UUID().String()
		if len(s.Name()) > 0 {
			msg += " (" + s.Name() + ")"
		}
		fmt.Println(msg)

		// Discovery characteristics
		cs, err := p.DiscoverCharacteristics(nil, s)
		if err != nil {
			fmt.Printf("Failed to discover characteristics, err: %s\n", err)
			continue
		}

		for _, c := range cs {
			msg := "  Characteristic  " + c.UUID().String()
			if len(c.Name()) > 0 {
				msg += " (" + c.Name() + ")"
			}
			msg += "\n    properties    " + c.Properties().String()
			fmt.Println(msg)

			// Read the characteristic, if possible.
			if (c.Properties() & gatt.CharRead) != 0 {
				b, err := p.ReadCharacteristic(c)
				if err != nil {
					fmt.Printf("Failed to read characteristic, err: %s\n", err)
					continue
				}
				fmt.Printf("    value         %x | %q\n", b, b)
			}

			// Discovery descriptors
			ds, err := p.DiscoverDescriptors(nil, c)
			if err != nil {
				fmt.Printf("Failed to discover descriptors, err: %s\n", err)
				continue
			}

			for _, d := range ds {
				msg := "  Descriptor      " + d.UUID().String()
				if len(d.Name()) > 0 {
					msg += " (" + d.Name() + ")"
				}
				fmt.Println(msg)

				// Read descriptor (could fail, if it's not readable)
				b, err := p.ReadDescriptor(d)
				if err != nil {
					fmt.Printf("Failed to read descriptor, err: %s\n", err)
					continue
				}
				fmt.Printf("    value         %x | %q\n", b, b)
			}
		}
		fmt.Println()
	}
}

func periphDisconnected(p gatt.Peripheral, err error) {
	fmt.Println("Disconnected")
	close(done)
}

func main() {

	flag.Parse()

	if len(flag.Args()) != 1 {
		fmt.Println("usage:", os.Args[0], "[options] peripheral-id")
		os.Exit(1)
	}

	d, err := gatt.NewDevice(option.DefaultClientOptions...)
	if err != nil {
		fmt.Printf("Failed to open device, err: %s\n", err)
		return
	}

	d.Handle(
		gatt.PeripheralDiscovered(periphDiscovered),
		gatt.PeripheralConnected(periphConnected),
		gatt.PeripheralDisconnected(periphDisconnected),
	)

	d.Init(stateChanged)
	fmt.Println("Waiting...")
	<-done
	fmt.Println("Done")
}
