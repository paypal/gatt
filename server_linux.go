package gatt

import (
	"log"
	"net"
	"time"

	"github.com/paypal/gatt/linux"
)

type advertiser interface {
	SetServing(s bool)
	Serving() bool
	Start() error
	Stop() error
	AdvertiseService() error
	Option(...linux.AdvertisingOption) linux.AdvertisingOption
}

func (s *Server) setAdvertisement(u []UUID, m []byte) error {
	if len(u) == 0 {
		for _, svc := range s.services {
			u = append(u, svc.uuid)
		}
	}

	// Wait until server is intitalized, or stopped
	for !s.serving {
		select {
		case <-s.quit:
			return nil
		case <-time.After(time.Second):
		}
	}

	ad, _ := ServiceAdvertisingPacket(u)
	s.adv.Option(linux.AdvertisingIntervalMax(0x00f4),
		linux.AdvertisingIntervalMin(0x00f4),
		linux.AdvertisingChannelMap(0x7),
		linux.AdvertisingPacket(ad),
		linux.ScanResponse(NameScanResponsePacket(s.name)))
	return s.adv.AdvertiseService()
}

func (s *Server) start() error {
	var logger *log.Logger
	h := linux.NewHCI(logger, s.maxConnections)
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
	return nil
}
