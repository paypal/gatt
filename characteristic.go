package gatt

import (
	"bytes"
	"fmt"
)

// Do not re-order the bit flags below;
// they are organized to match the BLE spec.

// Characteristic property flags.
const (
	charRead    = 1 << (iota + 1) // the characteristic may be read
	charWriteNR                   // the characteristic may be written to, with no reply
	charWrite                     // the characteristic may be written to, with a reply
	charNotify                    // the characteristic supports notifications
)

// Supported statuses for GATT characteristic read/write operations.
const (
	StatusSuccess         = attEcodeSuccess
	StatusInvalidOffset   = attEcodeInvalidOffset
	StatusUnexpectedError = attEcodeUnlikely
)

// A Request is the context for a request from a connected device.
type Request struct {
	Conn           Conn
	Service        *Service
	Characteristic *Characteristic
}

// A ReadRequest is a characteristic read request from a connected device.
type ReadRequest struct {
	Request
	Cap    int // maximum allowed reply length
	Offset int // request value offset
}

type ReadResponseWriter interface {
	// Write writes data to return as the characteristic value.
	Write([]byte) (int, error)
	// SetStatus reports the result of the read operation. See the Status* constants.
	SetStatus(byte)
}

// A ReadHandler handles GATT read requests.
type ReadHandler interface {
	ServeRead(resp ReadResponseWriter, req *ReadRequest)
}

// ReadHandlerFunc is an adapter to allow the use of
// ordinary functions as ReadHandlers. If f is a function
// with the appropriate signature, ReadHandlerFunc(f) is a
// ReadHandler that calls f.
type ReadHandlerFunc func(resp ReadResponseWriter, req *ReadRequest)

// ServeRead returns f(r, maxlen, offset).
func (f ReadHandlerFunc) ServeRead(resp ReadResponseWriter, req *ReadRequest) {
	f(resp, req)
}

// A WriteHandler handles GATT write requests.
// Write and WriteNR requests are presented identically;
// the server will ensure that a response is sent if appropriate.
type WriteHandler interface {
	ServeWrite(r Request, data []byte) (status byte)
}

// WriteHandlerFunc is an adapter to allow the use of
// ordinary functions as WriteHandlers. If f is a function
// with the appropriate signature, WriteHandlerFunc(f) is a
// WriteHandler that calls f.
type WriteHandlerFunc func(r Request, data []byte) byte

// ServeWrite returns f(r, data).
func (f WriteHandlerFunc) ServeWrite(r Request, data []byte) byte {
	return f(r, data)
}

// A NotifyHandler handles GATT notification requests.
// Notifications can be sent using the provided notifier.
type NotifyHandler interface {
	ServeNotify(r Request, n Notifier)
}

// NotifyHandlerFunc is an adapter to allow the use of
// ordinary functions as NotifyHandlers. If f is a function
// with the appropriate signature, NotifyHandlerFunc(f) is a
// NotifyHandler that calls f.
type NotifyHandlerFunc func(r Request, n Notifier)

// ServeNotify calls f(r, n).
func (f NotifyHandlerFunc) ServeNotify(r Request, n Notifier) {
	f(r, n)
}

// A Notifier provides a means for a GATT server to send
// notifications about value changes to a connected device.
// Notifiers are provided by NotifyHandlers.
type Notifier interface {
	// Write sends data to the central.
	Write(data []byte) (int, error)

	// Done reports whether the central has requested not to
	// receive any more notifications with this notifier.
	Done() bool

	// Cap returns the maximum number of bytes that may be sent
	// in a single notification.
	Cap() int
}

// A Characteristic is a BLE characteristic.
type Characteristic struct {
	uuid     UUID
	props    uint   // enabled properties
	secure   uint   // security enabled properties
	value    []byte // static value; internal use only; TODO: replace with "ValueHandler" instead
	descs    []*desc
	valuen   uint16 // handle; set during generateHandles, needed when notifying
	rhandler ReadHandler
	whandler WriteHandler
	nhandler NotifyHandler

	// storage used by other types
	service *Service
}

// HandleRead makes the characteristic support read requests,
// and routes read requests to h. HandleRead must be called
// before any server using c has been started.
func (c *Characteristic) HandleRead(h ReadHandler) {
	c.props |= charRead
	c.secure |= charRead
	c.rhandler = h
}

// HandleReadFunc calls HandleRead(ReadHandlerFunc(f)).
func (c *Characteristic) HandleReadFunc(f func(resp ReadResponseWriter, req *ReadRequest)) {
	c.HandleRead(ReadHandlerFunc(f))
}

// HandleWrite makes the characteristic support write and
// write-no-response requests, and routes write requests to h.
// The WriteHandler does not differentiate between write and
// write-no-response requests; it is handled automatically.
// HandleWrite must be called before any server using c has been started.
func (c *Characteristic) HandleWrite(h WriteHandler) {
	c.props |= charWrite | charWriteNR
	c.secure |= charWrite | charWriteNR
	c.whandler = h
}

// HandleWriteFunc calls HandleWrite(WriteHandlerFunc(f)).
func (c *Characteristic) HandleWriteFunc(f func(r Request, data []byte) (status byte)) {
	c.HandleWrite(WriteHandlerFunc(f))
}

// HandleNotify makes the characteristic support notify requests,
// and routes notification requests to h. HandleNotify must be called
// before any server using c has been started.
func (c *Characteristic) HandleNotify(h NotifyHandler) {
	c.props |= charNotify
	c.secure |= charNotify
	c.nhandler = h
}

// HandleNotifyFunc calls HandleNotify(NotifyHandlerFunc(f)).
func (c *Characteristic) HandleNotifyFunc(f func(r Request, n Notifier)) {
	c.HandleNotify(NotifyHandlerFunc(f))
}

// TODO: Add Indication support. It should be transparent and appear
// as a Notify, the way that Write and WriteNR are handled.

// UUID returns the characteristic's UUID
func (c *Characteristic) UUID() UUID {
	return c.uuid
}

// readResponseWriter is the default implementation of ReadResponseWriter.
type readResponseWriter struct {
	capacity int
	buf      *bytes.Buffer
	status   byte
}

func newReadResponseWriter(c int) *readResponseWriter {
	return &readResponseWriter{
		capacity: c,
		buf:      new(bytes.Buffer),
		status:   StatusSuccess,
	}
}

func (w *readResponseWriter) Write(b []byte) (int, error) {
	if avail := w.capacity - w.buf.Len(); avail < len(b) {
		return 0, fmt.Errorf("requested write %d bytes, %d available", len(b), avail)
	}
	return w.buf.Write(b)
}

func (w *readResponseWriter) SetStatus(status byte) { w.status = status }
func (w *readResponseWriter) bytes() []byte         { return w.buf.Bytes() }
