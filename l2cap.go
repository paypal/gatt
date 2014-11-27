// TODO: Figure out about how to structure things for multiple
// OS / BLE interface configurations. Build tags? Subpackages?

package gatt

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// newL2cap uses s to provide l2cap access.
func newL2cap(s shim, server *Server) *l2cap {
	c := &l2cap{
		shim:    s,
		readbuf: bufio.NewReader(s),
		server:  server,
		readc:   make(chan []byte),
	}
	return c
}

type l2cap struct {
	shim    shim
	readbuf *bufio.Reader
	sendmu  sync.Mutex // serializes writes to the shim
	handles *handleRange
	server  *Server
	serving bool
	quit    chan struct{}
	readc   chan []byte
}

func (c *l2cap) Read(b []byte) (int, error) {
	d, ok := <-c.readc
	if !ok {
		return 0, nil
	}
	if len(d) > len(b) {
		return 0, errors.New("buffer too small")
	}
	copy(b, d)
	return len(d), nil
}

func (c *l2cap) Write(b []byte) (int, error) {
	if len(b) > int(c.server.conn.mtu) {
		panic(fmt.Errorf("cannot send %x: mtu %d", b, c.server.conn.mtu))
	}

	// log.Printf("L2CAP: Sending %x", b)
	c.sendmu.Lock()
	_, err := fmt.Fprintf(c.shim, "%x\n", b)
	c.sendmu.Unlock()
	return len(b), err
}

func (c *l2cap) Close() error {
	if !c.serving {
		return errors.New("not serving")
	}
	c.serving = false
	close(c.quit)
	return nil
}

func (c *l2cap) listenAndServe() error {
	if c.serving {
		return errors.New("already serving")
	}
	c.serving = true
	c.quit = make(chan struct{})
	return c.eventloop()
}

func (c *l2cap) setServices(name string, svcs []*Service) error {
	// cannot be called while serving
	if c.serving {
		return errors.New("cannot set services while serving")
	}
	c.handles = generateHandles(name, svcs, uint16(1)) // ble handles start at 1
	// log.Println("Generated handles: ", c.handles)
	return nil
}

func (c *l2cap) close() error {
	if !c.serving {
		return errors.New("not serving")
	}
	c.serving = false
	close(c.quit)
	return nil
}

func (c *l2cap) eventloop() error {
	for {
		// TODO: Check c.quit *concurrently* with other
		// blocking activities.
		select {
		case <-c.quit:
			return nil
		default:
		}

		s, err := c.readbuf.ReadString('\n')
		// log.Printf("L2CAP: Received %s", s)
		if err != nil {
			return err
		}
		f := strings.Fields(s)
		if len(f) < 2 {
			continue
		}

		// TODO: Think about concurrency here. Do we want to spawn
		// new goroutines to not block this core loop?

		switch f[0] {
		case "accept":
			hw, err := net.ParseMAC(f[1])
			if err != nil {
				return errors.New("failed to parse accepted addr " + f[1] + ": " + err.Error())
			}
			c.server.connected(hw)
		case "disconnect":
			hw, err := net.ParseMAC(f[1])
			if err != nil {
				return errors.New("failed to parse disconnected addr " + f[1] + ": " + err.Error())
			}
			c.server.disconnected(hw)
		case "rssi":
			n, err := strconv.Atoi(f[1])
			if err != nil {
				return errors.New("failed to parse rssi " + f[1] + ": " + err.Error())
			}
			c.server.receivedRSSI(n)
		case "security":
			switch f[1] {
			case "low":
				c.server.conn.security = securityLow
			case "medium":
				c.server.conn.security = securityMed
			case "high":
				c.server.conn.security = securityHigh
			default:
				return errors.New("unexpected security change: " + f[1])
			}
			// TODO: notify l2capHandler about security change
		case "bdaddr":
			c.server.receivedBDAddr(f[1])
		case "hciDeviceId":
			// log.Printf("l2cap hci device: %s", f[1])
		case "data":
			req, err := hex.DecodeString(f[1])
			if err != nil {
				return err
			}
			c.readc <- req
		}
	}
}

func (c *l2cap) disconnect() error {
	return c.shim.Signal(syscall.SIGHUP)
}

func (c *l2cap) updateRSSI() error {
	return c.shim.Signal(syscall.SIGUSR1)
}
