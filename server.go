package gatt

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
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
	// Name is the device name, exposed via the Generic Access Service (0x1800).
	// Name may not be changed while serving.
	Name string

	// HCI is the hci device to use, e.g. "hci1".
	// If HCI is "", an hci device will be selected
	// automatically.
	HCI string

	// AdvertisingPacket is an optional custom advertising packet.
	// If nil, the advertising packet will constructed to advertise
	// as many services as possible. AdvertisingPacket must be set,
	// if at all, before starting the server. The AdvertisingPacket
	// must be no longer than MaxAdvertisingPacketLength.
	AdvertisingPacket []byte

	// ScanResponsePacket is an optional custom scan response packet.
	// If nil, the scan response packet will set to return the server
	// name, truncated if necessary. ScanResponsePacket must be set,
	// if at all, before starting the server. The ScanResponsePacket
	// must be no longer than MaxAdvertisingPacketLength.
	ScanResponsePacket []byte

	// TODO: Add a way to disable connections? The iBeacon advertising
	// packet will advertise that the device is not connectable. Do
	// we also need to enforce that?
	// AdvertiseOnly bool

	// Connect is an optional callback function that will be called
	// when a device has connected to the server.
	Connect func(c Conn)

	// Disconnect is an optional callback function that will be called
	// when a device has disconnected from the server.
	Disconnect func(c Conn)

	// ReceiveRSSI is an optional callback function that will be called
	// when an RSSI measurement has been received for a connection.
	ReceiveRSSI func(c Conn, rssi int)

	// Closed is an optional callback function that will be called
	// when the server is closed. err will be any associated error.
	// If the server was closed by calling Close, err may be nil.
	Closed func(error)

	// StateChange is an optional callback function that will be called
	// when the server changes states.
	// TODO: Break these states out into separate, meaningful methods?
	// At least document them.
	StateChange func(newState string)

	Serving bool

	hci   *hci
	l2cap *l2cap

	addr BDAddr

	// For now, there is one active conn per server; stash it here.
	// The conn part of the API is for forward-compatibility.
	// When Bluetooth 4.1 hits, there may be multiple active
	// connections per server, at which point, we'll need to
	// thread the connection through at each event. We won't
	// be able to do that without l2cap/BlueZ support, though.
	connmu sync.RWMutex
	conn   *conn

	services []*Service
	handles  *handleRange

	quitonce sync.Once
	quit     chan struct{}
	err      error
}

// AddService registers a new Service with the server.
// All services must be added before starting the server.
func (s *Server) AddService(u UUID) *Service {
	if s.Serving {
		return nil
	}
	svc := &Service{uuid: u}
	s.services = append(s.services, svc)
	return svc
}

// TODO: Helper function to construct iBeacon advertising packet.
// See e.g. http://stackoverflow.com/questions/18906988.

func (s *Server) startAdvertising() error {
	return s.hci.advertiseEIR(s.AdvertisingPacket, s.ScanResponsePacket)
}

func (s *Server) AdvertiseAndServe() error {
	if len(s.AdvertisingPacket) > MaxEIRPacketLength || len(s.ScanResponsePacket) > MaxEIRPacketLength {
		return ErrEIRPacketTooLong
	}

	if s.ScanResponsePacket == nil && s.Name != "" {
		s.ScanResponsePacket = nameScanResponsePacket(s.Name)
	}

	if s.AdvertisingPacket == nil {
		uuids := make([]UUID, len(s.services))
		for i, svc := range s.services {
			uuids[i] = svc.UUID()
		}
		s.AdvertisingPacket, _ = serviceAdvertisingPacket(uuids)
	}

	if err := s.start(); err != nil {
		return err
	}

	select {
	case <-s.quit:
		return s.err
	default:
	}

	s.Serving = true

	if err := s.setServices(s.Name, s.services); err != nil {
		return err
	}
	if err := s.startAdvertising(); err != nil {
		return err
	}

	return s.l2cap.listenAndServe()
}

// cleanHCIDevice converts hci (user-provided)
// into a format safe to pass to the c shims.
func cleanHCIDevice(hci string) string {
	if hci == "" {
		return ""
	}
	if strings.HasPrefix(hci, "hci") {
		hci = hci[len("hci"):]
	}
	if n, err := strconv.Atoi(hci); err != nil || n < 0 {
		return ""
	}
	return hci
}

func (s *Server) setServices(name string, svcs []*Service) error {
	// cannot be called while serving
	if s.Serving {
		return errors.New("cannot set services while serving")
	}
	s.handles = generateHandles(name, svcs, uint16(1)) // ble handles start at 1
	// log.Println("Generated handles: ", s.handles)
	return nil
}

func (s *Server) start() error {
	hciDevice := cleanHCIDevice(s.HCI)

	hciShim, err := newCShim("hci-ble", hciDevice)
	if err != nil {
		return err
	}

	s.quit = make(chan struct{})

	s.hci = newHCI(hciShim)
	event, err := s.hci.event()
	if err != nil {
		return err
	}
	if event == "unauthorized" {
		return errors.New("unauthorized; does l2cap-ble have the correct permissions?")
	}
	if event != "poweredOn" {
		return fmt.Errorf("unexpected hci event: %q", event)
	}
	// TODO: If you kill and restart the server quickly, you get event
	// "unsupported". Waiting and then starting again fixes it.
	// Figure out why, and handle it automatically.

	go func() {
		for {
			// No need to check s.quit here; if the users closes the server,
			// hci will get killed, which'll cause an error to be returned here.
			event, err := s.hci.event()
			if err != nil {
				break
			}
			if s.StateChange != nil {
				s.StateChange(event)
			}
		}
		s.close(err)
	}()

	if s.Closed != nil {
		go func() {
			<-s.quit
			s.Closed(s.err)
		}()
	}

	l2capShim, err := newCShim("l2cap-ble", hciDevice)
	if err != nil {
		s.close(err)
		return err
	}

	s.l2cap = newL2cap(l2capShim, s)
	return nil
}

// Close stops a Server.
func (s *Server) Close() error {
	if !s.Serving {
		return errors.New("not serving")
	}
	err := s.hci.Close()
	l2caperr := s.l2cap.close()
	if err == nil {
		err = l2caperr
	}
	s.close(err)
	return err
}

// A BDAddr (Bluetooth Device Address) is a
// hardware-addressed-based net.Addr.
type BDAddr struct {
	net.HardwareAddr
}

func (a BDAddr) Network() string { return "BLE" }

// Conn is a BLE connection. Due to the limitations of Bluetooth 4.0,
// there is only one active connection at a time; this will change in
// Bluetooth 4.1.
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

func (s *Server) close(err error) {
	s.quitonce.Do(func() {
		s.err = err
		close(s.quit)
	})
}
