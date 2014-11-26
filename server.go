package gatt

import (
	"errors"
	"net"
)

// MaxEIRPacketLength is the maximum allowed AdvertisingPacket
// and ScanResponsePacket length.
const MaxEIRPacketLength = 31

// ErrEIRPacketTooLong is the error returned when an AdvertisingPacket
// or ScanResponsePacket is too long.
var ErrEIRPacketTooLong = errors.New("max packet length is 31")

// A Server is a GATT server. Servers are single-shot types; once
// a Server has been closed, it cannot be restarted. Instead, create
// a new Server. Only one server may be running at a time.
type Server struct {
	name           string
	hci            string
	connect        func(c Conn)
	disconnect     func(c Conn)
	receiveRSSI    func(c Conn, rssi int)
	closed         func(error)
	stateChange    func(newState string)
	maxConnections int

	addr     BDAddr
	services []*Service
	handles  *handleRange
	serving  bool
	quit     chan struct{}
	err      error

	adv advertiser
}

func NewServer(opts ...option) *Server {
	s := &Server{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// AddService registers a new Service with the server.
// All services must be added before starting the server.
func (s *Server) AddService(u UUID) *Service {
	if s.serving {
		return nil
	}
	svc := &Service{uuid: u}
	s.services = append(s.services, svc)
	return svc
}

// SetAdvertisement builds advertisement data from the specified
// UUIDs and optional manufacture data. If the UUIDs is set to
// nil, the UUIDs of added services will be used instead.
func (s *Server) SetAdvertisement(u []UUID, m []byte) error {
	return s.setAdvertisement(u, m)
}

// StartAdvertising starts advertising.
func (s *Server) StartAdvertising() {
	s.adv.Start()
}

// StopAdvertising stops advertising.
func (s *Server) StopAdvertising() {
	s.adv.Stop()
}

// Advertising return the status of advertising.
func (s *Server) Advertising() bool {
	return s.adv.Serving()
}

// AdvetiseAndServe builds the advertising data from the UUIDs of
// added services, starts the server, and starts advertising.
func (s *Server) AdvertiseAndServe() error {
	if s.serving {
		return errors.New("a server is already running")
	}
	go func() {
		if err := s.SetAdvertisement(nil, nil); err != nil {
			s.err = err
			s.Close()
		}
	}()
	if err := s.start(); err != nil {
		return err
	}
	s.serving = true
	s.adv.Start()
	<-s.quit
	return s.err
}

// Serve starts the server.
func (s *Server) Serve() error {
	if s.serving {
		return errors.New("a server is already running")
	}
	if err := s.start(); err != nil {
		return err
	}
	s.serving = true
	<-s.quit
	return s.err
}

func (s *Server) setServices() error {
	// cannot be called while serving
	if s.serving {
		return errors.New("cannot set services while serving")
	}
	s.handles = generateHandles(s.name, s.services, uint16(1)) // ble handles start at 1
	return nil
}

// Close stops a Server.
func (s *Server) Close() error {
	if !s.serving {
		return errors.New("not serving")
	}
	s.adv.Stop()
	s.serving = false
	close(s.quit)
	return nil
}

type option func(*Server) option

// Option sets the options specified.
func (s *Server) Option(opts ...option) (prev option) {
	for _, opt := range opts {
		prev = opt(s)
	}
	return prev
}

// Name is the device name, exposed via the Generic Access Service (0x1800).
// Name may not be changed while serving.
func Name(n string) option {
	return func(s *Server) option {
		prev := s.name
		s.name = n
		return Name(prev)
	}
}

// HCI is the hci device to use, e.g. "hci1".
// If HCI is "", an hci device will be selected
// automatically.
func HCI(n string) option {
	return func(s *Server) option {
		prev := s.hci
		s.hci = n
		return HCI(prev)
	}
}

// Connect is an optional callback function that will be called
// when a device has connected to the server.
func Connect(f func(c Conn)) option {
	return func(s *Server) option {
		prev := s.connect
		s.connect = f
		return Connect(prev)
	}
}

// Disconnect is an optional callback function that will be called
// when a device has disconnected from the server.
func Disconnect(f func(c Conn)) option {
	return func(s *Server) option {
		prev := s.disconnect
		s.disconnect = f
		return Disconnect(prev)
	}
}

// ReceiveRSSI is an optional callback function that will be called
// when an RSSI measurement has been received for a connection.
func ReceiveRSSI(f func(c Conn, rssi int)) option {
	return func(s *Server) option {
		prev := s.receiveRSSI
		s.receiveRSSI = f
		return ReceiveRSSI(prev)
	}
}

// Closed is an optional callback function that will be called
// when the server is closed. err will be any associated error.
// If the server was closed by calling Close, err may be nil.
func Closed(f func(error)) option {
	return func(s *Server) option {
		prev := s.closed
		s.closed = f
		return Closed(prev)
	}
}

// StateChange is an optional callback function that will be called
// when the server changes states.
// TODO: Break these states out into separate, meaningful methods?
// At least document them.
func StateChange(f func(newState string)) option {
	return func(s *Server) option {
		prev := s.stateChange
		s.stateChange = f
		return StateChange(prev)
	}
}

// MaxConnections sets the maximum connections supported by the device.
// TODO: Extend the semantic to cover the AdvertiseOnly?
func MaxConnections(n int) option {
	return func(s *Server) option {
		prev := s.maxConnections
		s.maxConnections = n
		return MaxConnections(prev)
	}
}

// TODO: Helper function to construct iBeacon advertising packet.
// See e.g. http://stackoverflow.com/questions/18906988.

// A BDAddr (Bluetooth Device Address) is a
// hardware-addressed-based net.Addr.
type BDAddr struct{ net.HardwareAddr }

func (a BDAddr) Network() string { return "BLE" }

type Conn interface {
	// LocalAddr returns the address of the connected device (central).
	LocalAddr() BDAddr

	// LocalAddr returns the address of the local device (peripheral).
	RemoteAddr() BDAddr

	// Close disconnects the connection.
	Close() error

	// RSSI returns the last RSSI measurement, or -1 if there have not been any.
	RSSI() int

	// UpdateRSSI requests an RSSI update and blocks until one has been received.
	// TODO: Implement.
	UpdateRSSI() (rssi int, err error)

	// MTU returns the current connection mtu.
	MTU() int
}
