package main

import (
	"log"

	"github.com/paypal/gatt/linux/internal/hci"
)

func main() {
	res, err := hci.GetDeviceList()
	if err != nil {
		log.Fatalf("error retrieving bluetooth device list - %v", err)
	}
	log.Printf("count = %d", len(res))

	for _, dev := range res {
		log.Printf("device name %v", dev.Name())
		log.Printf("device addr %v", dev.Addr())
		log.Printf("device stats %+v", dev.Stats)
	}
}
