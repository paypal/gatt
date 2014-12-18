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

	advertisingPacket  []byte
	scanResponsePacket []byte
	manufacturerData   []byte

	addr     BDAddr
	services []*Service
	handles  *handleRange
	serving  bool
	quit     chan struct{}
	inited   chan struct{}
	err      error

	adv advertiser
}

// NewServer creates a Server with the specified options.
// See also Server.Options.
// See http://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis for more discussion.
func NewServer(opts ...option) *Server {
	s := &Server{maxConnections: 1, inited: make(chan struct{})}
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

// Advertise starts advertising.
func (s *Server) Advertise() {
	<-s.inited
	s.adv.Start()
}

// StopAdvertising stops advertising.
func (s *Server) StopAdvertising() {
	<-s.inited
	s.adv.Stop()
}

// Advertising reports whether the server is advertising.
func (s *Server) Advertising() bool {
	<-s.inited
	return s.adv.Serving()
}

// SetAdvertisement sets advertisement data to the specified
// UUIDs and optional manufacture data. If the UUIDs is set to
// nil, the UUIDs of added services will be used instead.
func (s *Server) SetAdvertisement(u []UUID, m []byte) error {
	return s.setAdvertisement(u, m)
}

// AdvertiseAndServe starts the server and advertises the UUIDs of its services.
func (s *Server) AdvertiseAndServe() error {
	if s.serving {
		return errors.New("a server is already running")
	}
	if err := s.start(); err != nil {
		return err
	}
	s.serving = true
	s.adv.Start()
	close(s.inited)
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
	close(s.inited)
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
// It returns an option to restore the last arg's previous value.
// Some options can only be set while the server is not running;
// they are best used with NewServer instead of Option.
// See http://commandcenter.blogspot.com.au/2014/01/self-referential-functions-and-design.html for more discussion.
func (s *Server) Option(opts ...option) (prev option) {
	for _, opt := range opts {
		prev = opt(s)
	}
	return prev
}

// Name sets the device name, exposed via the Generic Access Service (0x1800).
// Name cannot be called while serving.
// See also Server.NewServer and Server.Option.
func Name(n string) option {
	return func(s *Server) option {
		prev := s.name
		s.name = n
		return Name(prev)
	}
}

// HCI sets the hci device to use, e.g. "hci1".
// To automatically select an hci device, use "".
// HCI cannot be called while serving.
// See also Server.NewServer and Server.Option.
func HCI(hci string) option {
	return func(s *Server) option {
		if s.serving {
			panic("cannot set HCI while server is running")
		}
		prev := s.hci
		s.hci = hci
		return HCI(prev)
	}
}

// Connect sets a function to be called when a device connects to the server.
// See also Server.NewServer and Server.Option.
func Connect(f func(c Conn)) option {
	return func(s *Server) option {
		prev := s.connect
		s.connect = f
		return Connect(prev)
	}
}

// Disconnect sets a function to be called when a device disconnects from the server.
// See also Server.NewServer and Server.Option.
func Disconnect(f func(c Conn)) option {
	return func(s *Server) option {
		prev := s.disconnect
		s.disconnect = f
		return Disconnect(prev)
	}
}

// ReceiveRSSI sets a function to be called when an RSSI measurement is received for a connection.
// See also Server.NewServer and Server.Option.
func ReceiveRSSI(f func(c Conn, rssi int)) option {
	return func(s *Server) option {
		prev := s.receiveRSSI
		s.receiveRSSI = f
		return ReceiveRSSI(prev)
	}
}

// Closed sets a function to be called when a server is closed.
// err will be any associated error.
// If the server was closed by calling Close, err may be nil.
// See also Server.NewServer and Server.Option.
func Closed(f func(err error)) option {
	return func(s *Server) option {
		prev := s.closed
		s.closed = f
		return Closed(prev)
	}
}

// StateChange sets a function to be called when the server changes states.
// See also Server.NewServer and Server.Option.
// TODO: Break these states out into separate, meaningful methods?
// TODO: Document the set of states.
func StateChange(f func(newState string)) option {
	return func(s *Server) option {
		prev := s.stateChange
		s.stateChange = f
		return StateChange(prev)
	}
}

// MaxConnections sets the maximum number of allowed concurrent connections.
// Not all HCI devices support multiple connections.
// See also Server.NewServer.
// MaxConnections cannot be used with Server.Option.
func MaxConnections(n int) option {
	return func(s *Server) option {
		prev := s.maxConnections
		s.maxConnections = n
		return MaxConnections(prev)
	}
}

// AdvertisingPacket sets a custom advertising packet.
// If nil, the advertising data will constructed to advertise
// as many services as possible. The AdvertisingPacket must be no
// longer than MaxAdvertisingPacketLength.
// If ManufacturerData is also set, their total length must be no
// longer than MaxAdvertisingPacketLength.
// See also Server.NewServer and Server.Option.
func AdvertisingPacket(b []byte) option {
	return func(s *Server) option {
		s.setAdvertisingPacket(b)
		prev := s.advertisingPacket
		s.advertisingPacket = b
		return AdvertisingPacket(prev)
	}
}

// ScanResponsePacket sets a custom scan response packet.
// If nil, the scan response packet will set to return the server
// name, truncated if necessary. The ScanResponsePacket must be no
// longer than MaxAdvertisingPacketLength.
// See also Server.NewServer and Server.Option.
func ScanResponsePacket(b []byte) option {
	return func(s *Server) option {
		s.setScanResponsePacket(b)
		prev := s.scanResponsePacket
		s.scanResponsePacket = b
		return ScanResponsePacket(prev)
	}
}

// ManufacturerData sets custom manufacturer data.
// If set, it will be appended to the advertising data.
// The combined length of the AdvertisingPacket and ManufactureData
// must be no longer than MaxAdvertisingPacketLength .
// See also Server.NewServer and Server.Option.
func ManufacturerData(b []byte) option {
	return func(s *Server) option {
		s.setManufacturerData(b)
		prev := s.manufacturerData
		s.manufacturerData = b
		return ManufacturerData(prev)
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
