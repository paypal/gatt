package gatt

import (
	"encoding/hex"
	"fmt"
	"io"
	"testing"
	"time"
)

type testHandler struct {
	readc  chan []byte
	writec chan []byte
}

func (t *testHandler) Read(b []byte) (int, error) {
	r := <-t.readc
	if len(r) > len(b) {
		panic("fix this annoyance properly")
	}
	n := copy(b, r)
	return n, nil
}

func (t *testHandler) Write(b []byte) (int, error) {
	t.writec <- b
	return len(b), nil
}

func (t *testHandler) Close() error { return nil }

func TestServing(t *testing.T) {
	h := &testHandler{readc: make(chan []byte), writec: make(chan []byte)}

	var wrote []byte

	svc := &Service{uuid: MustParseUUID("09fc95c0-c111-11e3-9904-0002a5d5c51b")}

	svc.AddCharacteristic(MustParseUUID("11fac9e0-c111-11e3-9246-0002a5d5c51b")).HandleReadFunc(
		func(resp ReadResponseWriter, req *ReadRequest) {
			io.WriteString(resp, "count: 1")
		})

	svc.AddCharacteristic(MustParseUUID("16fe0d80-c111-11e3-b8c8-0002a5d5c51b")).HandleWriteFunc(
		func(r Request, data []byte) (status byte) {
			wrote = data
			return StatusSuccess
		})

	svc.AddCharacteristic(MustParseUUID("1c927b50-c116-11e3-8a33-0800200c9a66")).HandleNotifyFunc(
		func(r Request, n Notifier) {
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
		})

	handles := generateHandles("", []*Service{svc}, uint16(1)) // ble handles start at 1
	go newConn(handles, h, BDAddr{}).loop()

	// Generated handles:
	//   {1 1 0 5 service [24 0] <ptr> 0 0 []}
	//   {2 2 3 0 characteristic [42 0] <ptr> 2 2 []}
	//   {3 0 0 0 characteristicValue [42 0] <nil> 0 0 []}
	//   {4 4 5 0 characteristic [42 1] <ptr> 2 2 []}
	//   {5 0 0 0 characteristicValue [42 1] <nil> 0 0 [0 128]}
	//   {6 6 0 6 service [24 1] <ptr> 0 0 []}
	//   {7 7 0 65535 service [9 252 149 192 193 17 17 227 153 4 0 2 165 213 197 27] <ptr> 0 0 []}
	//   {8 8 9 0 characteristic [17 250 201 224 193 17 17 227 146 70 0 2 165 213 197 27] <ptr> 2 2 []}
	//   {9 0 0 0 characteristicValue [17 250 201 224 193 17 17 227 146 70 0 2 165 213 197 27] <nil> 0 0 []}
	//   {10 10 11 0 characteristic [22 254 13 128 193 17 17 227 184 200 0 2 165 213 197 27] <ptr> 12 12 []}
	//   {11 0 0 0 characteristicValue [22 254 13 128 193 17 17 227 184 200 0 2 165 213 197 27] <nil> 0 0 []}
	//   {12 12 13 0 characteristic [28 146 123 80 193 22 17 227 138 51 8 0 32 12 154 102] <ptr> 16 16 []}
	//   {13 0 0 0 characteristicValue [28 146 123 80 193 22 17 227 138 51 8 0 32 12 154 102] <nil> 0 0 []}
	//   {14 0 0 0 descriptor [41 2] <ptr> 10 10 [0 0]}] 1}

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
			want: "070700ffff",
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
		s, _ := hex.DecodeString(tt.send)
		if tt.send != "" {
			h.readc <- s
		}
		got := hex.EncodeToString(<-h.writec)
		if got != tt.want {
			t.Errorf("%s: sent %s got %s want %s", tt.name, tt.send, got, tt.want)
			continue
		}
		if tt.after != nil {
			tt.after()
		}
	}
}
