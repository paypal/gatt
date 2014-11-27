package gatt

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
)

type security int

const (
	securityLow = iota
	securityMed
	securityHigh
)

type conn struct {
	server      *Server
	localAddr   BDAddr
	remoteAddr  BDAddr
	rssi        int
	mtu         uint16
	security    security
	l2conn      io.ReadWriteCloser
	notifiers   map[*Characteristic]*notifier
	notifiersmu *sync.Mutex
}

func newConn(server *Server, l2conn io.ReadWriteCloser, addr BDAddr) *conn {
	return &conn{
		server:      server,
		rssi:        -1,
		localAddr:   server.addr,
		remoteAddr:  addr,
		mtu:         23,
		security:    securityLow,
		l2conn:      l2conn,
		notifiers:   make(map[*Characteristic]*notifier),
		notifiersmu: &sync.Mutex{},
	}
}

func (c *conn) String() string     { return c.remoteAddr.String() }
func (c *conn) LocalAddr() BDAddr  { return c.localAddr }
func (c *conn) RemoteAddr() BDAddr { return c.remoteAddr }
func (c *conn) Close() error {
	if err := c.close(); err != nil {
		return err
	}
	if err := c.l2conn.Close(); err != nil {
		return err
	}
	return nil
}
func (c *conn) RSSI() int { return c.rssi }
func (c *conn) MTU() int  { return int(c.mtu) }
func (c *conn) UpdateRSSI() (rssi int, err error) {
	// TODO
	return 0, errors.New("not implemented yet")
}
func (c *conn) close() error {
	// Stop all notifiers
	// TODO: Clear all descriptor CCC values?
	c.notifiersmu.Lock()
	defer c.notifiersmu.Unlock()
	for _, n := range c.notifiers {
		n.stop()
	}
	return nil
}

func (c *conn) loop() {
	// TODO: rework the usage io.ReadWriterCloser to conform the semantic.
	// Or, alternatively, cook a more stiuable interface between L2CAP layer.
	for {
		// L2CAP implementations shall support a minimum MTU size of 48 bytes.
		// The default value is 672 bytes
		b := make([]byte, 672)
		n, err := c.l2conn.Read(b)
		if n == 0 || err != nil {
			break
		}
		if rsp := c.handleReq(b[:n]); rsp != nil {
			c.l2conn.Write(rsp)
		}
	}
	c.close()
}

// handleReq dispatches a raw request from the conn shim
// to an appropriate handler, based on its type.
// It panics if len(b) == 0.
func (c *conn) handleReq(b []byte) []byte {
	var resp []byte

	switch reqType, req := b[0], b[1:]; reqType {
	case attOpMtuReq:
		resp = c.handleMTU(req)
	case attOpFindInfoReq:
		resp = c.handleFindInfo(req)
	case attOpFindByTypeReq:
		resp = c.handleFindByType(req)
	case attOpReadByTypeReq:
		resp = c.handleReadByType(req)
	case attOpReadReq, attOpReadBlobReq:
		resp = c.handleRead(reqType, req)
	case attOpReadByGroupReq:
		resp = c.handleReadByGroup(req)
	case attOpWriteReq, attOpWriteCmd:
		resp = c.handleWrite(reqType, req)
	case attOpReadMultiReq, attOpPrepWriteReq, attOpExecWriteReq, attOpSignedWriteCmd:
		fallthrough
	default:
		resp = attErrorResp(reqType, 0x0000, attEcodeReqNotSupp)
	}

	return resp
}

func (c *conn) handleMTU(b []byte) []byte {
	c.mtu = binary.LittleEndian.Uint16(b)
	// This sanity check helps keep the response
	// writing code easier, since you don't have
	// to double-check that the response headers
	// will fit in the MTU. This is also the min
	// allowed by the BLE spec; we're just
	// enforcing it.
	if c.mtu < 23 {
		c.mtu = 23
	}
	// Clip the value to a reasonably safe value.
	// TODO: make this value configurable.
	if c.mtu >= 256 {
		c.mtu = 256
	}
	return []byte{attOpMtuResp, uint8(c.mtu), uint8(c.mtu >> 8)}
}

func (c *conn) handleFindInfo(b []byte) []byte {
	start, end := readHandleRange(b)

	w := newL2capWriter(c.mtu)
	w.WriteByteFit(attOpFindInfoResp)
	uuidLen := -1
	for _, h := range c.server.l2cap.handles.Subrange(start, end) {
		var uuid UUID
		switch h.typ {
		case typService:
			uuid = gattAttrPrimaryServiceUUID
		case typIncludedService:
			uuid = gattAttrSecondaryServiceUUID
		case typCharacteristic:
			uuid = gattAttrCharacteristicUUID
		case typCharacteristicValue, typDescriptor:
			uuid = h.uuid
		default:
			continue
		}

		if uuidLen == -1 {
			uuidLen = uuid.Len()
			if uuidLen == 2 {
				w.WriteByteFit(0x01) // TODO: constants for 16bit vs 128bit uuid magic numbers here
			} else {
				w.WriteByteFit(0x02)
			}
		}
		if uuid.Len() != uuidLen {
			break
		}

		w.Chunk()
		w.WriteUint16Fit(h.n)
		w.WriteUUIDFit(uuid)
		if ok := w.Commit(); !ok {
			break
		}
	}

	if uuidLen == -1 {
		return attErrorResp(attOpFindInfoReq, start, attEcodeAttrNotFound)
	}
	return w.Bytes()
}

func (c *conn) handleFindByType(b []byte) []byte {
	start, end := readHandleRange(b)

	if uuid := (UUID{reverse(b[4:6])}); !uuidEqual(uuid, gattAttrPrimaryServiceUUID) {
		return attErrorResp(attOpFindByTypeReq, start, attEcodeAttrNotFound)
	}

	uuid := UUID{reverse(b[6:])}

	w := newL2capWriter(c.mtu)
	w.WriteByteFit(attOpFindByTypeResp)

	var wrote bool
	for _, h := range c.server.l2cap.handles.Subrange(start, end) {
		if !h.isPrimaryService(uuid) {
			continue
		}
		w.Chunk()
		w.WriteUint16Fit(h.startn)
		w.WriteUint16Fit(h.endn)
		if ok := w.Commit(); !ok {
			break
		}
		wrote = true
	}

	if !wrote {
		return attErrorResp(attOpFindByTypeReq, start, attEcodeAttrNotFound)
	}

	return w.Bytes()
}

func (c *conn) handleReadByType(b []byte) []byte {
	start, end := readHandleRange(b)
	uuid := UUID{reverse(b[4:])}

	// TODO: Refactor out into two extra helper handle* functions?
	if uuidEqual(uuid, gattAttrCharacteristicUUID) {
		w := newL2capWriter(c.mtu)
		w.WriteByteFit(attOpReadByTypeResp)
		uuidLen := -1
		for _, h := range c.server.l2cap.handles.Subrange(start, end) {
			if h.typ != typCharacteristic {
				continue
			}
			if uuidLen == -1 {
				uuidLen = h.uuid.Len()
				w.WriteByteFit(byte(uuidLen + 5))
			}
			if h.uuid.Len() != uuidLen {
				break
			}
			w.Chunk()
			w.WriteUint16Fit(h.startn)
			w.WriteByteFit(byte(h.props))
			w.WriteUint16Fit(h.valuen)
			w.WriteUUIDFit(h.uuid)
			if ok := w.Commit(); !ok {
				break
			}
		}
		if uuidLen == -1 {
			return attErrorResp(attOpReadByTypeReq, start, attEcodeAttrNotFound)
		}
		return w.Bytes()
	}

	// TODO: Refactor out into two extra helper handle* functions?
	// !bytes.Equal(uuid, gattAttrCharacteristicUUID)
	var valuen uint16
	var found bool
	var secure bool

	for _, h := range c.server.l2cap.handles.Subrange(start, end) {
		if h.isCharacteristic(uuid) {
			valuen = h.valuen
			secure = h.secure&charRead != 0
			found = true
			break
		}
		if h.isDescriptor(uuid) {
			valuen = h.n
			secure = h.secure&charRead != 0
			found = true
			break
		}
	}

	if !found {
		return attErrorResp(attOpReadByTypeReq, start, attEcodeAttrNotFound)
	}
	if secure && c.security > securityLow {
		return attErrorResp(attOpReadByTypeReq, start, attEcodeAuthentication)
	}

	valueh, ok := c.server.l2cap.handles.At(valuen)
	if !ok {
		// This can only happen (I think) if we've done
		// a bad job constructing our handles.
		panic(fmt.Errorf("bad value handle reading %x: %v\n\nHandles: %#v", uuid, valuen, c.server.l2cap.handles))
	}
	w := newL2capWriter(c.mtu)
	datalen := w.Writeable(4, valueh.value)
	w.WriteByteFit(attOpReadByTypeResp)
	w.WriteByteFit(byte(datalen + 2))
	w.WriteUint16Fit(valuen)
	w.WriteFit(valueh.value)

	return w.Bytes()
}

func (c *conn) handleRead(reqType byte, b []byte) []byte {
	valuen := binary.LittleEndian.Uint16(b)
	var offset uint16
	if reqType == attOpReadBlobReq {
		offset = binary.LittleEndian.Uint16(b[2:])
	}
	respType := attRespFor[reqType]

	h, ok := c.server.l2cap.handles.At(valuen)
	if !ok {
		return attErrorResp(reqType, valuen, attEcodeInvalidHandle)
	}

	w := newL2capWriter(c.mtu)
	w.WriteByteFit(respType)
	w.Chunk()

	switch h.typ {
	case typService, typIncludedService:
		w.WriteUUIDFit(h.uuid)
	case typCharacteristic:
		w.WriteByteFit(byte(h.props))
		w.WriteUint16Fit(h.valuen)
		w.WriteUUIDFit(h.uuid)
	case typCharacteristicValue, typDescriptor:
		valueh := h
		if h.typ == typCharacteristicValue {
			vh, ok := c.server.l2cap.handles.At(valuen - 1) // TODO: Store a cross-reference explicitly instead of this -1 nonsense.
			if !ok {
				panic(fmt.Errorf("invalid handle reference reading characteristicValue handle %d:\n\nHandles: %#v", valuen-1, c.server.l2cap.handles))
			}
			valueh = vh
		}
		if valueh.props&charRead == 0 {
			return attErrorResp(reqType, valuen, attEcodeReadNotPerm)
		}
		if valueh.secure&charRead != 0 && c.security > securityLow {
			return attErrorResp(reqType, valuen, attEcodeAuthentication)
		}
		if h.value != nil {
			w.WriteFit(h.value)
		} else {
			// Ask server for data
			char := valueh.attr.(*Characteristic) // TODO: Rethink attr being interface{}
			data, status := c.readChar(char, int(c.mtu-1), int(offset))
			if status != StatusSuccess {
				return attErrorResp(reqType, valuen, status)
			}
			w.WriteFit(data)
			offset = 0 // the server has already adjusted for the offset
		}
	default:
		// Shouldn't happen?
		return attErrorResp(reqType, valuen, attEcodeInvalidHandle)
	}

	if ok := w.ChunkSeek(offset); !ok {
		return attErrorResp(reqType, valuen, attEcodeInvalidOffset)
	}

	w.CommitFit()
	return w.Bytes()
}

func (c *conn) handleReadByGroup(b []byte) []byte {
	start, end := readHandleRange(b)
	uuid := UUID{reverse(b[4:])}

	var typ handleType
	switch {
	case uuidEqual(uuid, gattAttrPrimaryServiceUUID):
		typ = typService
	case uuidEqual(uuid, gattAttrIncludeUUID):
		typ = typIncludedService
	default:
		return attErrorResp(attOpReadByGroupReq, start, attEcodeUnsuppGrpType)
	}

	w := newL2capWriter(c.mtu)
	w.WriteByteFit(attOpReadByGroupResp)
	uuidLen := -1
	for _, h := range c.server.l2cap.handles.Subrange(start, end) {
		if h.typ != typ {
			continue
		}
		if uuidLen == -1 {
			uuidLen = h.uuid.Len()
			w.WriteByteFit(byte(uuidLen + 4))
		}
		if uuidLen != h.uuid.Len() {
			break
		}
		w.Chunk()
		w.WriteUint16Fit(h.startn)
		w.WriteUint16Fit(h.endn)
		w.WriteUUIDFit(h.uuid)
		if ok := w.Commit(); !ok {
			break
		}
	}
	if uuidLen == -1 {
		return attErrorResp(attOpReadByGroupReq, start, attEcodeAttrNotFound)
	}

	return w.Bytes()
}

func (c *conn) handleWrite(reqType byte, b []byte) []byte {
	valuen := binary.LittleEndian.Uint16(b)
	data := b[2:]

	h, ok := c.server.l2cap.handles.At(valuen)
	if !ok {
		return attErrorResp(reqType, valuen, attEcodeInvalidHandle)
	}

	if h.typ == typCharacteristicValue {
		vh, ok := c.server.l2cap.handles.At(valuen - 1) // TODO: Clean this up somehow by storing a better ref explicitly.
		if !ok {
			panic(fmt.Errorf("invalid handle reference writing characteristicValue handle %d: \n\nHandles: %#v", valuen-1, c.server.l2cap.handles))
		}
		h = vh
	}

	noResp := reqType == attOpWriteCmd
	charFlag := uint(charWrite)
	if noResp {
		charFlag = charWriteNR
	}

	if h.props&charFlag == 0 {
		return attErrorResp(reqType, valuen, attEcodeWriteNotPerm)
	}
	if h.secure&charFlag == 0 && c.security > securityLow {
		return attErrorResp(reqType, valuen, attEcodeAuthentication)
	}

	if h.typ != typDescriptor && !uuidEqual(h.uuid, gattAttrClientCharacteristicConfigUUID) {
		// Regular write, not CCC
		result := c.writeChar(h.attr.(*Characteristic), data, noResp)
		if noResp {
			return nil
		}
		if result != StatusSuccess {
			return attErrorResp(reqType, valuen, result)
		}
		return []byte{attOpWriteResp}
	}

	// CCC/descriptor write
	if len(data) != 2 {
		return attErrorResp(reqType, valuen, attEcodeInvalAttrValueLen)
	}

	ccc := binary.LittleEndian.Uint16(data)
	char := h.attr.(*Characteristic)
	h.value = data

	if ccc&gattCCCNotifyFlag == 0 {
		// TODO: Suppress these calls if the notification state hasn't actually changed
		c.stopNotify(char)
		if noResp {
			return nil
		}
		return []byte{attOpWriteResp}
	}

	c.startNotify(char, int(c.mtu-3))
	if noResp {
		return nil
	}
	return []byte{attOpWriteResp}
}

func (c *conn) sendNotification(char *Characteristic, data []byte) (int, error) {
	w := newL2capWriter(c.mtu)
	w.WriteByteFit(attOpHandleNotify)
	w.WriteUint16Fit(char.valuen)
	w.WriteFit(data)
	b := w.Bytes()
	return c.l2conn.Write(b)
}

func readHandleRange(b []byte) (start, end uint16) {
	return binary.LittleEndian.Uint16(b), binary.LittleEndian.Uint16(b[2:])
}

func (c *conn) request(char *Characteristic) Request {
	return Request{
		Server:         c.server,
		Service:        char.service,
		Characteristic: char,
		Conn:           c,
	}
}

func (c *conn) readChar(char *Characteristic, maxlen int, offset int) (data []byte, status byte) {
	req := &ReadRequest{Request: c.request(char), Cap: maxlen, Offset: offset}
	resp := newReadResponseWriter(maxlen)
	char.rhandler.ServeRead(resp, req)
	return resp.bytes(), resp.status
}

func (c *conn) writeChar(char *Characteristic, data []byte, noResponse bool) (status byte) {
	return char.whandler.ServeWrite(c.request(char), data)
}

func (c *conn) startNotify(char *Characteristic, maxlen int) {
	c.notifiersmu.Lock()
	defer c.notifiersmu.Unlock()
	if _, found := c.notifiers[char]; found {
		return
	}
	n := newNotifier(c, char, maxlen)
	c.notifiers[char] = n
	char.nhandler.ServeNotify(c.request(char), n)
}

func (c *conn) stopNotify(char *Characteristic) {
	c.notifiersmu.Lock()
	defer c.notifiersmu.Unlock()
	if n, found := c.notifiers[char]; found {
		n.stop()
		delete(c.notifiers, char)
	}
}
