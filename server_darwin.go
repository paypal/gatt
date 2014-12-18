package gatt

// This is just a placeholder for the gatt package to pass
// some intergraion systems (builders), when they try to build
// native executable on OSX.

type advertiser interface {
	SetServing(s bool)
	Serving() bool
	Start() error
	Stop() error
	AdvertiseService() error
	// Option(...linux.Option) linux.Option
}

func (s *Server) setDefaultAdvertisement() error            { return nil }
func (s *Server) setAdvertisement(u []UUID, m []byte) error { return nil }
func (s *Server) setAdvertisingPacket(b []byte)             {}
func (s *Server) setScanResponsePacket(b []byte)            {}
func (s *Server) setManufacturerData(b []byte)              {}
func (s *Server) start() error                              { return nil }
