package gatt

import (
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

type testL2CShim struct {
	readc  chan []byte
	writec chan []byte
}

func (t *testL2CShim) Read(b []byte) (int, error) {
	r := <-t.readc
	if len(r) > len(b) {
		panic("fix this annoyance properly")
	}
	n := copy(b, r)
	return n, nil
}

func (t *testL2CShim) Write(b []byte) (int, error) {
	t.writec <- b
	return len(b), nil
}

func (t *testL2CShim) Close() error           { return nil }
func (t *testL2CShim) Wait() error            { return nil }
func (t *testL2CShim) Signal(os.Signal) error { return nil }

type testL2CapHandler struct {
	l2c *l2cap
}

func (testL2CapHandler) readChar(c *Characteristic, maxlen int, offset int) ([]byte, byte) {
	resp := newReadResponseWriter(maxlen)
	c.rhandler.ServeRead(resp, new(ReadRequest))
	return resp.bytes(), resp.status
}

func (testL2CapHandler) writeChar(c *Characteristic, data []byte, noResponse bool) byte {
	return c.whandler.ServeWrite(Request{}, data)
}

func (t *testL2CapHandler) startNotify(c *Characteristic, maxlen int) {
	if c.notifier != nil {
		return
	}
	c.notifier = newNotifier(t.l2c, c, maxlen)
	c.nhandler.ServeNotify(Request{}, c.notifier)
}

func (testL2CapHandler) stopNotify(c *Characteristic) {
	c.notifier.stop()
	c.notifier = nil
}

func (testL2CapHandler) connected(hw net.HardwareAddr)    {}
func (testL2CapHandler) disconnected(hw net.HardwareAddr) {}
func (testL2CapHandler) receivedRSSI(rssi int)            {}
func (testL2CapHandler) receivedBDAddr(bdaddr string)     {}

func TestServing(t *testing.T) {
	h := new(testL2CapHandler)
	shim := &testL2CShim{readc: make(chan []byte), writec: make(chan []byte)}
	l2c := newL2cap(shim, h)
	h.l2c = l2c

	var wrote []byte

	svc := &Service{
		uuid: MustParseUUID("09fc95c0-c111-11e3-9904-0002a5d5c51b"),
	}
	svc.chars = []*Characteristic{
		&Characteristic{
			service: svc,
			uuid:    MustParseUUID("11fac9e0-c111-11e3-9246-0002a5d5c51b"),
			props:   charRead,
			secure:  charRead,
			rhandler: ReadHandlerFunc(func(resp ReadResponseWriter, req *ReadRequest) {
				io.WriteString(resp, "count: 1")
			}),
		},
		&Characteristic{
			service: svc,
			uuid:    MustParseUUID("16fe0d80-c111-11e3-b8c8-0002a5d5c51b"),
			props:   charWrite | charWriteNR,
			secure:  charWrite | charWriteNR,
			whandler: WriteHandlerFunc(func(r Request, data []byte) (status byte) {
				wrote = data
				return StatusSuccess
			}),
		},
		&Characteristic{
			service: svc,
			uuid:    MustParseUUID("1c927b50-c116-11e3-8a33-0800200c9a66"),
			props:   charNotify,
			secure:  charNotify,
			nhandler: NotifyHandlerFunc(func(r Request, n Notifier) {
				go func() {
					count := 0
					for !n.Done() {
						data := []byte(fmt.Sprintf("Count: %d", count))
						_, err := n.Write(data)
						if err != nil {
							panic(err)
						}
						count++
						time.Sleep(10 * time.Millisecond)
					}
				}()
			}),
		},
	}

	l2c.setServices("", []*Service{svc})

	// Generated handles:
	//   {1 1 0 5 service [24 0] <ptr> 0 0 []}
	//   {2 2 3 0 characteristic [42 0] <ptr> 2 2 []}
	//   {3 0 0 0 characteristicValue [42 0] <nil> 0 0 []}
	//   {4 4 5 0 characteristic [42 1] <ptr> 2 2 []}
	//   {5 0 0 0 characteristicValue [42 1] <nil> 0 0 [0 128]}
	//   {6 6 0 6 service [24 1] <ptr> 0 0 []}
	//   {7 7 0 14 service [9 252 149 192 193 17 17 227 153 4 0 2 165 213 197 27] <ptr> 0 0 []}
	//   {8 8 9 0 characteristic [17 250 201 224 193 17 17 227 146 70 0 2 165 213 197 27] <ptr> 2 2 []}
	//   {9 0 0 0 characteristicValue [17 250 201 224 193 17 17 227 146 70 0 2 165 213 197 27] <nil> 0 0 []}
	//   {10 10 11 0 characteristic [22 254 13 128 193 17 17 227 184 200 0 2 165 213 197 27] <ptr> 12 12 []}
	//   {11 0 0 0 characteristicValue [22 254 13 128 193 17 17 227 184 200 0 2 165 213 197 27] <nil> 0 0 []}
	//   {12 12 13 0 characteristic [28 146 123 80 193 22 17 227 138 51 8 0 32 12 154 102] <ptr> 16 16 []}
	//   {13 0 0 0 characteristicValue [28 146 123 80 193 22 17 227 138 51 8 0 32 12 154 102] <nil> 0 0 []}
	//   {14 0 0 0 descriptor [41 2] <ptr> 10 10 [0 0]}] 1}

	go l2c.listenAndServe()

	rxtx := []struct {
		name  string
		send  string
		want  string
		after func()
	}{
		{
			name: "set mtu to 135 -- mtu is 135",
			send: "028700",
			want: "038700",
		},
		{
			name: "set mtu to 23 -- mtu is 23", // keep later req/resp small!
			send: "021700",
			want: "031700",
		},
		{
			name: "bad req -- unsupported",
			send: "FF1234567890",
			want: "01ff000006",
		},
		{
			name: "find info [1,10] -- 1: 0x2800, 2: 0x2803, 3: 0x2a00, 4: 0x2803, 5: 0x2a01",
			send: "0401000A00",
			want: "050101000028020003280300002a040003280500012a",
		},
		{
			name: "find info [1,2] -- 1: 0x2800, 2: 0x2803",
			send: "0401000200",
			want: "05010100002802000328",
		},
		{
			name: "find by type [1,11] svc uuid -- handle range [7,14]",
			send: "0601000B0000281bc5d5a502000499e31111c1c095fc09",
			want: "0707000e00",
		},
		{
			name: "read by group [1,3] svc uuid -- unsupported group type at handle 1",
			send: "10010003001bc5d5a502000499e31111c1c095fc09",
			want: "0110010010",
		},
		{
			name: "read by group [1,3] 0x2800 -- group at [1,5]: 0x1800",
			send: "10010003000028",
			want: "1106010005000018",
		},
		{
			name: "read by group [1,14] 0x2800 -- group at [1,5]: 0x1800, [6,6]: 0x1801",
			send: "1001000E000028",
			want: "1106010005000018060006000118",
		},
		{
			name: "read by type [1,5] 0x2a00 (device name) -- found 2, 3",
			send: "0801000500002a",
			want: "09020300",
		},
		{
			name: "read by type [4,5] 0x2a00 (device name) -- not found",
			send: "0804000500002a",
			want: "010804000a",
		},
		{
			name: "read by type [6,6] 0x2803 (attr char) -- not found",
			send: "08060006000328",
			want: "010806000a",
		},
		{
			name: "read char -- 'count: 1'",
			send: "0a0900",
			want: "0b636f756e743a2031",
		},
		{
			name: "write char 'abcdef' -- ok",
			send: "120b00616263646566",
			want: "13",
			after: func() {
				if string(wrote) != "abcdef" {
					t.Errorf("wrote: got %q want %q", wrote, "abcdef")
				}
			},
		},
		{
			name: "start notify -- ok",
			send: "120e000100",
			want: "13",
		},
		{
			name: "-- notified 'Count: 0'",
			want: "1b0d00436f756e743a2030",
		},
		{
			name: "-- notified 'Count: 1'",
			want: "1b0d00436f756e743a2031",
		},
		{
			name: "-- notified 'Count: 2'",
			want: "1b0d00436f756e743a2032",
		},
		{
			name: "-- notified 'Count: 3'",
			want: "1b0d00436f756e743a2033",
		},
		{
			name: "stop notify -- ok",
			send: "120e000000",
			want: "13",
		},
	}

	for _, tt := range rxtx {
		if tt.send != "" {
			shim.readc <- []byte("data " + tt.send + "\n")
		}
		resp := <-shim.writec
		if resp[len(resp)-1] != '\n' {
			t.Errorf("%s: sent %q, response %q does not end in \\n", tt.name, tt.send, resp)
			continue
		}
		got := string(resp[:len(resp)-1]) // trim \n
		if got != tt.want {
			t.Errorf("%s: sent %q got %q want %q", tt.name, tt.send, got, tt.want)
			continue
		}
		if tt.after != nil {
			tt.after()
		}
	}
}
