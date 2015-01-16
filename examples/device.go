package main

import (
	"log"

	"../../gatt/linux"
)

func main() {
	res, err := linux.GetDeviceList()
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
