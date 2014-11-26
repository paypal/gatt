package gatt

import "errors"

type conn struct {
	server     *Server
	localAddr  BDAddr
	remoteAddr BDAddr
	rssi       int
}

func newConn(server *Server, addr BDAddr) *conn {
	return &conn{
		server:     server,
		rssi:       -1,
		localAddr:  server.addr,
		remoteAddr: addr,
	}
}

func (c *conn) String() string     { return c.remoteAddr.String() }
func (c *conn) LocalAddr() BDAddr  { return c.localAddr }
func (c *conn) RemoteAddr() BDAddr { return c.remoteAddr }
func (c *conn) Close() error       { return c.server.disconnect(c) }
func (c *conn) RSSI() int          { return c.rssi }
func (c *conn) MTU() int           { return int(c.server.l2cap.mtu) }

func (c *conn) UpdateRSSI() (rssi int, err error) {
	// TODO
	return 0, errors.New("not implemented yet")
}
