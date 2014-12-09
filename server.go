package gatt

import (
	"errors"
	"log"
	"net"
	"time"

	"github.com/paypal/gatt/hci"
	"github.com/paypal/gatt/hci/device"
)

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

	ManufacturerData []byte

	// MaxConnections Set the maximum connections supported by the device.
	// TODO: Extend the semantic to cover the AdvertiseOnly?
	MaxConnections int

	adv      *Advertiser
	addr     BDAddr
	services []*Service
	handles  *handleRange
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

func (s *Server) AdvertiseAndServe() error {
	go s.SetAdvertisement(nil, nil)
	return s.Serve()
}

func (s *Server) Serve() error {
	if s.Serving {
		return errors.New("a server is already running")
	}
	if err := s.start(); err != nil {
		return err
	}

	s.Serving = true
	<-s.quit
	return s.err
}

func (s *Server) setServices() error {
	// cannot be called while serving
	if s.Serving {
		return errors.New("cannot set services while serving")
	}
	s.handles = generateHandles(s.Name, s.services, uint16(1)) // ble handles start at 1
	return nil
}

func (s *Server) start() error {
	// logger := log.New(os.Stderr, "", log.LstdFlags)
	var logger *log.Logger
	// FIXME: fix the sloppiness here
	d, err := device.NewSocket(1)
	if err != nil {
		d, err = device.NewSocket(0)
		if err != nil {
			return err
		}
	}
	h := hci.NewHCI(d, logger, s.MaxConnections)
	a := NewAdvertiser(h)
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
				gc := newConn(s, l2c, remoteAddr)
				go func() {
					if s.Connect != nil {
						s.Connect(gc)
					}
					gc.loop()
					if s.Disconnect != nil {
						s.Disconnect(gc)
					}
				}()
			case <-s.quit:
				h.Close()
				if s.Closed != nil {
					s.Closed(s.err)
				}
				return
			}
		}
	}()
	h.Start()
	return nil
}

// Close stops a Server.
func (s *Server) Close() error {
	if !s.Serving {
		return errors.New("not serving")
	}
	s.adv.Stop()
	s.Serving = false
	close(s.quit)
	return nil
}

func (s *Server) SetAdvertisement(u []UUID, m []byte) {
	if len(u) == 0 {
		for _, svc := range s.services {
			u = append(u, svc.uuid)
		}
	}
	// Wait until server is intitalized, or stopped
	for !s.Serving {
		select {
		case <-s.quit:
			return
		case <-time.After(time.Second):
		}
	}
	s.adv.mu.Lock()
	s.adv.AdvertisingPacket, _ = ServiceAdvertisingPacket(u)
	s.adv.ScanResponsePacket = NameScanResponsePacket(s.Name)
	s.adv.ManufacturerData = m
	s.adv.mu.Unlock()
	s.adv.AdvertiseService()
	s.adv.Start()
}

// A BDAddr (Bluetooth Device Address) is a
// hardware-addressed-based net.Addr.
type BDAddr struct {
	net.HardwareAddr
}

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

	// SetPrivateData sets an optional user defined data.
	// TODO: rework the interfaces to leverage the context package.
	SetPrivateData(interface{})

	// PrivateData returns the user defined data, if assigned.
	PrivateData() interface{}
}
