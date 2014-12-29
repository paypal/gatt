// +build ignore

// This corresponds to the sample code found in doc.go.
// TODO: Clean this up and turn it into proper examples.

package main

import (
	"fmt"
	"log"
	"time"

	"github.com/paypal/gatt"
)

func main() {
	s := gatt.NewDevice()
	s.Name = "gophers"
	s.Connected = func(c gatt.Conn) { log.Println("Connect: ", c) }
	s.Disconnected = func(c gatt.Conn) { log.Println("Disconnect: ", c) }
	s.ReceiveRSSI = func(c gatt.Conn, rssi int) { log.Println("RSSI: ", c, " ", rssi) }
	s.Closed = func(err error) { log.Println("Server closed: ", err) }
	s.StateChange = func(newState string) { log.Println("Server state change: ", newState) }

	svc := gatt.NewService(gatt.MustParseUUID("09fc95c0-c111-11e3-9904-0002a5d5c51b"))

	n := 0
	rchar := svc.AddCharacteristic(gatt.MustParseUUID("11fac9e0-c111-11e3-9246-0002a5d5c51b"))
	rchar.HandleRead(
		gatt.ReadHandlerFunc(
			func(resp gatt.ReadResponseWriter, req *gatt.ReadRequest) {
				fmt.Fprintf(resp, "count: %d", n)
				n++
			}),
	)

	wchar := svc.AddCharacteristic(gatt.MustParseUUID("16fe0d80-c111-11e3-b8c8-0002a5d5c51b"))
	wchar.HandleWriteFunc(
		func(r gatt.Request, data []byte) (status byte) {
			log.Println("Wrote:", string(data))
			// return gatt.StatusSuccess
			return 0
		})

	nchar := svc.AddCharacteristic(gatt.MustParseUUID("1c927b50-c116-11e3-8a33-0800200c9a66"))
	nchar.HandleNotifyFunc(
		func(r gatt.Request, n gatt.Notifier) {
			go func() {
				count := 0
				for !n.Done() {
					fmt.Fprintf(n, "Count: %d", count)
					count++
					time.Sleep(time.Second)
				}
			}()
		})

	s.AddService(svc)
	s.Advertise()

	select {}
}
