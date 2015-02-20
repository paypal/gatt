package gatt

import (
	"errors"
	"log"
	"net"
	"time"

	"github.com/acmacalister/gatt/linux"
	"github.com/acmacalister/gatt/linux/internal/cmd"
)

type advertiser interface {
	SetServing(s bool)
	Serving() bool
	Start() error
	Stop() error
	AdvertiseService() error
	Option(...linux.Option) linux.Option
}

// setDefaultAdvertisement builds advertisement data from the
// UUIDs of services.
func (s *Server) setDefaultAdvertisement() error {
	opts := []linux.Option{
		linux.AdvertisingIntervalMax(0x00f4),
		linux.AdvertisingIntervalMin(0x00f4),
		linux.AdvertisingChannelMap(0x7),
	}
	if len(s.advertisingPacket) == 0 {
		u := []UUID{}
		for _, svc := range s.services {
			u = append(u, svc.uuid)
		}
		ad, _ := serviceAdvertisingPacket(u)
		opts = append(opts, linux.AdvertisingPacket(ad))
	}
	if len(s.scanResponsePacket) == 0 {
		opts = append(opts, linux.ScanResponsePacket(nameScanResponsePacket(s.name)))
	}
	s.adv.Option(opts...)
	return s.adv.AdvertiseService()
}

func (s *Server) setAdvertisingServices(u []UUID) {
	ad, _ := serviceAdvertisingPacket(u)
	s.advertisingPacket = ad
	if s.serving {
		s.adv.Option(linux.AdvertisingPacket(ad))
	}
}

func (s *Server) setAdvertisingPacket(b []byte) {
	if s.serving {
		s.adv.Option(linux.AdvertisingPacket(b))
	}
}

func (s *Server) setScanResponsePacket(b []byte) {
	if s.serving {
		s.adv.Option(linux.ScanResponsePacket(b))
	}
}

func (s *Server) setManufacturerData(b []byte) {
	if s.serving {
		s.adv.Option(linux.ManufacturerData(b))
	}
}

func (s *Server) start() error {
	var logger *log.Logger
	h, err := linux.NewHCI(logger, s.maxConnections)

	if err != nil {
		return err
	}

	a := linux.NewAdvertiser(h.Cmd())
	l := h.L2CAP()
	l.Adv = a

	if err := s.setServices(); err != nil {
		return err
	}

	s.quit = make(chan struct{})
	s.adv = a

	go func() {
		for {
			select {
			case l2c := <-l.ConnC():
				remoteAddr := BDAddr{net.HardwareAddr(l2c.Param.PeerAddress[:])}
				c := newConn(s, l2c, remoteAddr)
				go func() {
					if s.connect != nil {
						s.connect(c)
					}
					c.loop()
					if s.disconnect != nil {
						s.disconnect(c)
					}
				}()
			case <-s.quit:
				h.Close()
				if s.closed != nil {
					s.closed(s.err)
				}
				return
			}
		}
	}()
	h.Start()
	// monitor the status of the BLE controller
	go func() {
		// Send a HCI command to controller periodically, if we don't get response
		// for a while, close the server to notify upper layer.
		t := time.AfterFunc(time.Second*30, func() {
			s.err = errors.New("device does not respond")
			s.Close()
		})
		for _ = range time.Tick(time.Second * 10) {
			h.Cmd().SendAndCheckResp(cmd.LEReadBufferSize{}, []byte{0x00})
			t.Reset(time.Second * 30)
		}
	}()
	return s.setDefaultAdvertisement()
}
