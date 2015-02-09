package linux

import (
	"fmt"
	"io"
	"log"
	"sync"
)

type l2cap struct {
	hci *HCI

	maxConn int
	bufCnt  chan struct{}
	bufSize int

	connsmu  *sync.Mutex
	connsSeq int
	conns    map[uint16]*conn
}

func newL2CAP(maxConn int) *l2cap {
	return &l2cap{
		// TODO: should be quired from controller, or specified by user.
		maxConn: maxConn,
		bufCnt:  make(chan struct{}, 15-1),
		bufSize: 27,

		connsmu:  &sync.Mutex{},
		connsSeq: 0,
		conns:    map[uint16]*conn{},
	}
}

type aclData struct {
	attr  uint16
	flags uint8
	dlen  uint16
	b     []byte
}

func (h *aclData) unmarshal(b []byte) error {
	if len(b) < 4 {
		return fmt.Errorf("malformed acl packet")
	}
	attr := uint16(b[0]) | (uint16(b[1]&0x0f) << 8)
	flags := b[1] >> 4
	dlen := uint16(b[2]) | (uint16(b[3]) << 8)
	if len(b) != 4+int(dlen) {
		return fmt.Errorf("malformed acl packet")
	}

	*h = aclData{attr: attr, flags: flags, dlen: dlen, b: b[4:]}
	return nil
}

func (h *aclData) String() string {
	return fmt.Sprintf("ACL Data: attr %d flags 0x%02X dlen 0x%04X", h.attr, h.flags, h.dlen)
}

func (l *l2cap) handleLEMeta(b []byte) error {
	code := leEventCode(b[0])
	switch code {
	case leConnectionComplete:
		l.hci.SetAdvertiseEnable(false)
		ep := &leConnectionCompleteEP{}
		if err := ep.unmarshal(b); err != nil {
			return err
		}
		h := ep.connectionHandle
		c := newConn(l, h, ep, l.connsSeq)
		l.connsSeq++
		l.connsmu.Lock()
		defer l.connsmu.Unlock()
		if c, found := l.conns[h]; found {
			l.trace("l2cap: attr 0x%04X is still alived (seq: %d)", h, c.seq)
		}
		l.conns[h] = c
		go l.hci.handleConnection(c, c.param.peerAddress, ep.role == 0x01)
		if len(l.conns) < l.maxConn {
			l.hci.SetAdvertiseEnable(true)
		}

		// FIXME: sloppiness. This call should be called by the package user once we
		// flesh out the support of l2cap signaling packets (CID:0x0001,0x0005)
		if ep.connLatency != 0 || ep.connInterval > 0x18 {
			c.updateConnection()
		}

	case leConnectionUpdateComplete:
		// anything to do here?

	case leAdvertisingReport:
		l.hci.handleAdvertisement(b)

	// case leReadRemoteUsedFeaturesComplete:
	// case leLTKRequest:
	// case leRemoteConnectionParameterRequest:
	default:
		return fmt.Errorf("Unhandled LE event: %s", code)
	}
	return nil
}

func (l *l2cap) handleDisconnectionComplete(b []byte) error {
	ep := &disconnectionCompleteEP{}
	if err := ep.unmarshal(b); err != nil {
		return err
	}
	h := ep.connectionHandle
	l.connsmu.Lock()
	defer l.connsmu.Unlock()
	c, found := l.conns[h]
	if !found {
		l.trace("l2conn: disconnecting a disconnected 0x%04X connection", h)
		return nil
	}
	delete(l.conns, h)
	l.trace("l2conn: 0x%04X disconnected, seq: %d", h, c.seq)
	close(c.aclc)
	if len(l.conns) == l.maxConn-1 {
		l.hci.SetAdvertiseEnable(true)
	}
	return nil
}

func (l *l2cap) handleNumberOfCompletedPkts(b []byte) error {
	ep := &numberOfCompletedPktsEP{}
	if err := ep.unmarshal(b); err != nil {
		return err
	}
	for _, r := range ep.packets {
		for i := 0; i < int(r.numOfCompletedPkts); i++ {
			<-l.bufCnt
		}
	}
	return nil
}

func (l *l2cap) handleL2CAP(b []byte) error {
	a := &aclData{}
	if err := a.unmarshal(b); err != nil {
		return err
	}
	l.connsmu.Lock()
	defer l.connsmu.Unlock()
	if c, found := l.conns[a.attr]; found {
		c.aclc <- a
		return nil
	}
	return nil
}

func (l *l2cap) Close() error {
	l.trace("l2cap: Close()")
	for _, c := range l.conns {
		c.Close()
	}
	return nil
}

func (l *l2cap) trace(fmt string, v ...interface{}) {}

type conn struct {
	l2c   *l2cap
	attr  uint16
	aclc  chan *aclData
	param *leConnectionCompleteEP
	seq   int
}

func newConn(l *l2cap, h uint16, ep *leConnectionCompleteEP, seq int) *conn {
	l.trace("l2conn: 0x%04X connected, seq :%d", h, seq)
	return &conn{
		l2c:   l,
		attr:  h,
		param: ep,
		aclc:  make(chan *aclData),
		seq:   seq,
	}
}

func (c *conn) updateConnection() (int, error) {
	b := []byte{
		0x12,       // Code (Connection Param Update)
		0x02,       // ID
		0x08, 0x00, // DataLength
		0x08, 0x00, // IntervalMin
		0x18, 0x00, // IntervalMax
		0x00, 0x00, // SlaveLatency
		0xC8, 0x00} // TimeoutMultiplier
	return c.write(0x05, b)
}

// write writes the l2cap payload to the controller.
// It first prepend the l2cap header (4-bytes), and diassemble the payload
// if it is larger than the HCI LE buffer size that the conntroller can support.
func (c *conn) write(cid int, b []byte) (int, error) {
	flag := uint8(0) // ACL data continuation flag
	tlen := len(b)   // Total length of the l2cap payload

	log.Printf("W: [ % X ]", b)
	w := append(
		[]byte{
			0,    // packet type
			0, 0, // attr
			0, 0, // dlen
			uint8(tlen), uint8(tlen >> 8), // l2cap header
			uint8(cid), uint8(cid >> 8), // l2cap header
		}, b...)

	n := 4 + tlen // l2cap header + l2cap payload
	for n > 0 {
		dlen := n
		if dlen > c.l2c.bufSize {
			dlen = c.l2c.bufSize
		}
		w[0] = 0x02 // packetTypeACL
		w[1] = uint8(c.attr)
		w[2] = uint8(c.attr>>8) | flag
		w[3] = uint8(dlen)
		w[4] = uint8(dlen >> 8)

		// make sure we don't send more buffers than the controller can handdle
		c.l2c.bufCnt <- struct{}{}

		c.l2c.hci.d.Write(w[:5+dlen])
		w = w[dlen:] // advance the pointer to the next segment, if any.
		flag = 0x10  // the rest of iterations attr continued segments, if any.
		n -= dlen
	}

	return len(b), nil
}

func (c *conn) Read(b []byte) (int, error) {
	a, ok := <-c.aclc
	if !ok {
		return 0, io.EOF
	}

	tlen := int(uint16(a.b[0]) | uint16(a.b[1])<<8)
	if tlen > len(b) {
		return 0, io.ErrShortBuffer
	}
	d := a.b[4:] // skip l2cap header
	copy(b, d)
	n := len(d)

	// Keep receiving and reassemble continued l2cap segments
	for n != tlen {
		if a, ok = <-c.aclc; !ok || (a.flags&0x1) == 0 {
			return n, io.ErrUnexpectedEOF
		}
		copy(b[n:], a.b)
		n += len(a.b)
	}
	log.Printf("R: [ % X ]", b[:n])
	return n, nil
}

func (c *conn) Write(b []byte) (int, error) {
	return c.write(0x04, b)
}

// Close disconnects the connection by sending HCI disconnect command to the device.
func (c *conn) Close() error {
	l := c.l2c
	h := c.attr
	l.trace("l2conn: disconnct 0x%04X, seq: %d", c.attr, c.seq)
	l.connsmu.Lock()
	defer l.connsmu.Unlock()
	cc, found := l.conns[h]
	if !found {
		l.trace("l2conn: 0x%04X already disconnected", h)
		return nil
	} else if c != cc {
		l.trace("l2conn: 0x%04X seq mismatch %d/%d", h, c.seq, cc.seq)
		return nil
	}
	if err, _ := l.hci.c.send(disconnect{connectionHandle: h, reason: 0x13}); err != nil {
		l.trace("l2conn: failed to disconnect, %s", err)
	}
	return nil
}

// Signal Packets
// 0x00 Reserved								Any
// 0x01 Command reject							0x0001 and 0x0005
// 0x02 Connection request						0x0001
// 0x03 Connection response 					0x0001
// 0x04 Configure request						0x0001
// 0x05 Configure response						0x0001
// 0x06 Disconnection request					0x0001 and 0x0005
// 0x07 Disconnection response					0x0001 and 0x0005
// 0x08 Echo request							0x0001
// 0x09 Echo response							0x0001
// 0x0A Information request						0x0001
// 0x0B Information response					0x0001
// 0x0C Create Channel request					0x0001
// 0x0D Create Channel response					0x0001
// 0x0E Move Channel request					0x0001
// 0x0F Move Channel response					0x0001
// 0x10 Move Channel Confirmation				0x0001
// 0x11 Move Channel Confirmation response		0x0001
// 0x12 Connection Parameter Update request		0x0005
// 0x13 Connection Parameter Update response	0x0005
// 0x14 LE Credit Based Connection request		0x0005
// 0x15 LE Credit Based Connection response		0x0005
// 0x16 LE Flow Control Credit					0x0005
