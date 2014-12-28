package l2cap

import (
	"fmt"
	"io"
	"sync"

	"github.com/paypal/gatt/linux/internal/cmd"
	"github.com/paypal/gatt/linux/internal/event"
)

type l2adv interface {
	Start() error
	Stop() error
	Serving() bool
	SetServing(bool)
}

type L2CAP struct {
	dev     io.ReadWriter
	cmd     *cmd.Cmd
	acceptc chan *Conn

	maxConn int
	bufCnt  chan struct{}
	bufSize int
	Adv     l2adv

	connsmu  *sync.Mutex
	connsSeq int
	conns    map[uint16]*Conn
}

func NewL2CAP(cmd *cmd.Cmd, d io.ReadWriter, maxConn int) *L2CAP {
	return &L2CAP{
		cmd:     cmd,
		dev:     d,
		acceptc: make(chan *Conn),

		// TODO: should be quired from controller, or specified by user.
		maxConn: maxConn,
		bufCnt:  make(chan struct{}, 15-1),
		bufSize: 27,

		connsmu:  &sync.Mutex{},
		connsSeq: 0,
		conns:    map[uint16]*Conn{},
	}
}

type aclData struct {
	handle uint16
	flags  uint8
	dlen   uint16
	b      []byte
}

func (h *aclData) Unmarshal(b []byte) error {
	if len(b) < 4 {
		return fmt.Errorf("malformed acl packet")
	}
	handle := uint16(b[0]) | (uint16(b[1]&0x0f) << 8)
	flags := b[1] >> 4
	dlen := uint16(b[2]) | (uint16(b[3]) << 8)
	if len(b) != 4+int(dlen) {
		return fmt.Errorf("malformed acl packet")
	}

	*h = aclData{handle: handle, flags: flags, dlen: dlen, b: b[4:]}
	return nil
}

func (h *aclData) String() string {
	return fmt.Sprintf("ACL Data: handle %d flags 0x%02X dlen 0x%04X", h.handle, h.flags, h.dlen)
}

func (l *L2CAP) HandleLEMeta(b []byte) error {
	code := event.LEEventCode(b[0])
	switch code {
	case event.LEConnectionComplete:
		l.Adv.SetServing(false)
		ep := &event.LEConnectionCompleteEP{}
		if err := ep.Unmarshal(b); err != nil {
			return err
		}
		h := ep.ConnectionHandle
		c := newConn(l, h, ep, l.connsSeq)
		l.connsSeq++
		l.connsmu.Lock()
		defer l.connsmu.Unlock()
		if c, found := l.conns[h]; found {
			l.trace("l2cap: handle 0x%04X is still alived (seq: %d)", h, c.seq)
		}

		l.conns[h] = c
		l.acceptc <- c
		if len(l.conns) < l.maxConn {
			l.Adv.Start()
		}

		// FIXME: sloppiness. This call should be called by the package user once we
		// flesh out the support of L2CAP signaling packets (CID:0x0001,0x0005)
		if ep.ConnLatency != 0 || ep.ConnInterval > 0x18 {
			c.UpdateConnection()
		}

	case event.LEConnectionUpdateComplete:

	case event.LEAdvertisingReport,
		event.LEReadRemoteUsedFeaturesComplete,
		event.LELTKRequest,
		event.LERemoteConnectionParameterRequest:
		return fmt.Errorf("Unhandled LE event: %s", code)
	}
	return nil
}

func (l *L2CAP) HandleDisconnectionComplete(b []byte) error {
	ep := &event.DisconnectionCompleteEP{}
	if err := ep.Unmarshal(b); err != nil {
		return err
	}
	h := ep.ConnectionHandle
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
		l.Adv.Start()
	}
	return nil
}

func (l *L2CAP) HandleNumberOfCompletedPkts(b []byte) error {
	ep := &event.NumberOfCompletedPktsEP{}
	if err := ep.Unmarshal(b); err != nil {
		return err
	}
	for _, r := range ep.Packets {
		for i := 0; i < int(r.NumOfCompletedPkts); i++ {
			<-l.bufCnt
		}
	}
	return nil
}

func (l *L2CAP) HandleL2CAP(b []byte) error {
	a := &aclData{}
	if err := a.Unmarshal(b); err != nil {
		return err
	}
	l.connsmu.Lock()
	defer l.connsmu.Unlock()
	if c, found := l.conns[a.handle]; found {
		c.aclc <- a
		return nil
	}
	return nil
}

func (l *L2CAP) ConnC() chan *Conn {
	return l.acceptc
}

func (l *L2CAP) Close() error {
	l.trace("l2cap: Close()")
	close(l.acceptc)
	for _, c := range l.conns {
		c.Close()
	}
	return nil
}

func (l *L2CAP) trace(fmt string, v ...interface{}) {}

type Conn struct {
	l2c    *L2CAP
	handle uint16
	aclc   chan *aclData
	Param  *event.LEConnectionCompleteEP
	seq    int
}

func newConn(l *L2CAP, h uint16, ep *event.LEConnectionCompleteEP, seq int) *Conn {
	l.trace("l2conn: 0x%04X connected, seq :%d", h, seq)
	return &Conn{
		l2c:    l,
		handle: h,
		Param:  ep,
		aclc:   make(chan *aclData),
		seq:    seq,
	}
}

// write writes the L2CAP payload to the controller.
// It first prepend the L2CAP header (4-bytes), and diassemble the payload
// if it is larger than the HCI LE buffer size that the conntroller can support.
func (c *Conn) write(cid int, b []byte) (int, error) {
	flag := uint8(0) // ACL data continuation flag
	tlen := len(b)   // Total length of the L2CAP payload

	w := append(
		[]byte{
			0,    // packet type
			0, 0, // handle
			0, 0, // dlen
			uint8(tlen), uint8(tlen >> 8), // L2CAP header
			uint8(cid), uint8(cid >> 8), // L2CAP header
		}, b...)

	n := 4 + tlen // L2CAP header + L2CAP payload
	for n > 0 {
		dlen := n
		if dlen > c.l2c.bufSize {
			dlen = c.l2c.bufSize
		}
		w[0] = 0x02 // packetTypeACL
		w[1] = uint8(c.handle)
		w[2] = uint8(c.handle>>8) | flag
		w[3] = uint8(dlen)
		w[4] = uint8(dlen >> 8)

		// make sure we don't send more buffers than the controller can handdle
		c.l2c.bufCnt <- struct{}{}

		c.l2c.dev.Write(w[:5+dlen])
		w = w[dlen:] // advance the pointer to the next segment, if any.
		flag = 0x10  // the rest of iterations handle continued segments, if any.
		n -= dlen
	}

	return len(b), nil
}

func (c *Conn) UpdateConnection() (int, error) {
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

func (c *Conn) Read(b []byte) (int, error) {
	a, ok := <-c.aclc
	if !ok {
		return 0, io.EOF
	}

	tlen := int(uint16(a.b[0]) | uint16(a.b[1])<<8)
	if tlen > len(b) {
		return 0, io.ErrShortBuffer
	}
	d := a.b[4:] // skip L2CAP header
	copy(b, d)
	n := len(d)

	// Keep receiving and reassemble continued L2CAP segments
	for n != tlen {
		if a, ok = <-c.aclc; !ok || (a.flags&0x1) == 0 {
			return n, io.ErrUnexpectedEOF
		}
		copy(b[n:], a.b)
		n += len(a.b)
	}
	return n, nil
}

func (c *Conn) Write(b []byte) (int, error) {
	return c.write(0x04, b)
}

// Close disconnects the connection by sending HCI disconnect command to the device.
func (c *Conn) Close() error {
	l := c.l2c
	h := c.handle
	l.trace("l2conn: disconnct 0x%04X, seq: %d", c.handle, c.seq)
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
	if err, _ := l.cmd.Send(cmd.Disconnect{ConnectionHandle: h, Reason: 0x13}); err != nil {
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
