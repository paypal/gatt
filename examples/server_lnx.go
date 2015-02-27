// +build

package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/paypal/gatt"
	"github.com/paypal/gatt/examples/service"
	"github.com/paypal/gatt/linux/cmd"
)

// server_lnx implements a GATT server.
// It uses some linux specific options for more finer control over the device.

var (
	mc    = flag.Int("mc", 1, "Maximum concurrent connections")
	id    = flag.Duration("id", 0, "ibeacon duration")
	ii    = flag.Duration("ii", 5*time.Second, "ibeacon interval")
	name  = flag.String("name", "Gopher", "Device Name")
	chmap = flag.Int("chmap", 0x7, "Advertising channel map")
	dev   = flag.Int("dev", -1, "HCI device ID")
	chk   = flag.Bool("chk", true, "Check device LE support")
)

// cmdReadBDAddr implements cmd.CmdParam for demostrating LnxSendHCIRawCommand()
type cmdReadBDAddr struct{}

func (c cmdReadBDAddr) Marshal(b []byte) {}
func (c cmdReadBDAddr) Opcode() int      { return 0x1009 }
func (c cmdReadBDAddr) Len() int         { return 0 }

// Get bdaddr with LnxSendHCIRawCommand() for demo purpose
func bdaddr(d gatt.Device) {
	rsp := bytes.NewBuffer(nil)
	if err := d.Option(gatt.LnxSendHCIRawCommand(&cmdReadBDAddr{}, rsp)); err != nil {
		fmt.Printf("Failed to send HCI raw command, err: %s", err)
	}
	b := rsp.Bytes()
	if b[0] != 0 {
		fmt.Printf("Failed to get bdaddr with HCI Raw command, status: %d", b[0])
	}
	log.Printf("BD Addr: %02X:%02X:%02X:%02X:%02X:%02X", b[6], b[5], b[4], b[3], b[2], b[1])
}

func main() {
	flag.Parse()
	d, err := gatt.NewDevice(
		gatt.LnxMaxConnections(*mc),
		gatt.LnxDeviceID(*dev, *chk),
		gatt.LnxSetAdvertisingParameters(&cmd.LESetAdvertisingParameters{
			AdvertisingIntervalMin: 0x00f4,
			AdvertisingIntervalMax: 0x00f4,
			AdvertisingChannelMap:  0x07,
		}),
	)

	if err != nil {
		log.Printf("Failed to open device, err: %s", err)
		return
	}

	// Register optional handlers.
	d.Handle(
		gatt.CentralConnected(func(c gatt.Central) { log.Println("Connect: ", c.ID()) }),
		gatt.CentralDisconnected(func(c gatt.Central) { log.Println("Disconnect: ", c.ID()) }),
	)

	// A mandatory handler for monitoring device state.
	onStateChanged := func(d gatt.Device, s gatt.State) {
		fmt.Printf("State: %s\n", s)
		switch s {
		case gatt.StatePoweredOn:
			// Get bdaddr with LnxSendHCIRawCommand()
			bdaddr(d)

			// Setup GAP and GATT services.
			d.AddService(service.NewGapService(*name))
			d.AddService(service.NewGattService())

			// Add a simple counter service.
			s1 := service.NewCountService()
			d.AddService(s1)

			// Add a simple counter service.
			s2 := service.NewBatteryService()
			d.AddService(s2)
			uuids := []gatt.UUID{s1.UUID(), s2.UUID()}

			// If id is zero, advertise name and services statically.
			if *id == time.Duration(0) {
				d.AdvertiseNameAndServices(*name, uuids)
				break
			}

			// If id is non-zero, advertise name and services and iBeacon alternately.
			go func() {
				for {
					// Advertise as a RedBear Labs iBeacon.
					d.AdvertiseIBeacon(gatt.MustParseUUID("5AFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"), 1, 2, -59)
					time.Sleep(*id)

					// Advertise name and services.
					d.AdvertiseNameAndServices(*name, uuids)
					time.Sleep(*ii)
				}
			}()

		default:
		}
	}

	d.Init(onStateChanged)
	select {}
}
