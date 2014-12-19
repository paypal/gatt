package gatt

import "errors"

// This is a placeholder so that gatt can build on OS X.

type advertiser interface {
	SetServing(s bool)
	Serving() bool
	Start() error
	Stop() error
	AdvertiseService() error
	// Option(...linux.Option) linux.Option
}

var notImplemented = errors.New("not implemented")

func (s *Server) setDefaultAdvertisement() error            { return notImplemented }
func (s *Server) setAdvertisement(u []UUID, m []byte) error { return notImplemented }
func (s *Server) setAdvertisingServices(u []UUID)           {}
func (s *Server) setAdvertisingPacket(b []byte)             {}
func (s *Server) setScanResponsePacket(b []byte)            {}
func (s *Server) setManufacturerData(b []byte)              {}
func (s *Server) start() error                              { return notImplemented }
