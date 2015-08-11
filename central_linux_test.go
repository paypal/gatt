package gatt

import (
	"encoding/hex"
	"fmt"
	"io"
	"net"
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
		func(resp ResponseWriter, req *ReadRequest) {
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

	longString := "A really long characteristic"
	svc.AddCharacteristic(MustParseUUID("11fac9e0-c111-11e3-9246-0002a5d5c51c")).HandleReadFunc(
		func(resp ResponseWriter, req *ReadRequest) {
			start := req.Offset
			end := req.Offset + req.Cap
			if len(longString) < start {
				start = len(longString)
			}

			if len(longString) < end {
				end = len(longString)
			}
			io.WriteString(resp, longString[start:end])
		})

	svc.AddCharacteristic(MustParseUUID("11fac9e0-c111-11e3-9246-0002a5d5c51d")).SetValue([]byte(longString))

	gapSvc := NewService(attrGAPUUID)

	gapSvc.AddCharacteristic(attrDeviceNameUUID).SetValue([]byte("Gopher"))
	gapSvc.AddCharacteristic(attrAppearanceUUID).SetValue([]byte{0x00, 0x80})
	gattSvc := NewService(attrGATTUUID)

	a := generateAttributes([]*Service{gapSvc, gattSvc, svc}, uint16(1)) // ble a start at 1
	go newCentral(a, net.HardwareAddr{}, h).loop()

	// 0x0001	0x2800	0x02	0x00	*gatt.Service	[ 00 18 ]
	// 0x0002	0x2803	0x02	0x00	*gatt.Characteristic	[ 02 03 00 00 2A ]
	// 0x0003	0x2a00	0x02	0x00	*gatt.Characteristic	[ 47 6F 70 68 65 72 ]
	// 0x0004	0x2803	0x02	0x00	*gatt.Characteristic	[ 02 05 00 01 2A ]
	// 0x0005	0x2a01	0x02	0x00	*gatt.Characteristic	[ 00 80 ]
	// 0x0006	0x2800	0x02	0x00	*gatt.Service	[ 01 18 ]
	// 0x0007	0x2800	0x02	0x00	*gatt.Service	[ 1B C5 D5 A5 02 00 04 99 E3 11 11 C1 C0 95 FC 09 ]
	// 0x0008	0x2803	0x02	0x00	*gatt.Characteristic	[ 02 09 00 1B C5 D5 A5 02 00 46 92 E3 11 11 C1 E0 C9 FA 11 ]
	// 0x0009	0x11fac9e0c11111e392460002a5d5c51b	0x02	0x00	*gatt.Characteristic	[  ]
	// 0x000A	0x2803	0x0C	0x00	*gatt.Characteristic	[ 0C 0B 00 1B C5 D5 A5 02 00 C8 B8 E3 11 11 C1 80 0D FE 16 ]
	// 0x000B	0x16fe0d80c11111e3b8c80002a5d5c51b	0x0C	0x00	*gatt.Characteristic	[  ]
	// 0x000C	0x2803	0x30	0x00	*gatt.Characteristic	[ 30 0D 00 66 9A 0C 20 00 08 33 8A E3 11 16 C1 50 7B 92 1C ]
	// 0x000D	0x1c927b50c11611e38a330800200c9a66	0x30	0x00	*gatt.Characteristic	[  ]
	// 0x000E	0x2902	0x0E	0x00	*gatt.Descriptor	[ 00 00 ]
	// 0x000F	0x2803	0x02	0x00	*gatt.Characteristic	[ 02 10 00 1C C5 D5 A5 02 00 46 92 E3 11 11 C1 E0 C9 FA 11 ]
	// 0x0010	0x11fac9e0c11111e392460002a5d5c51c	0x02	0x00	*gatt.Characteristic	[  ]
	// 0x0011	0x2803	0x02	0x00	*gatt.Characteristic	[ 02 12 00 1D C5 D5 A5 02 00 46 92 E3 11 11 C1 E0 C9 FA 11 ]
	// 0x0012	0x11fac9e0c11111e392460002a5d5c51d	0x02	0x00	*gatt.Characteristic	[ 41 20 72 65 61 6C 6C 79 20 6C 6F 6E 67 20 63 68 61 72 61 63 74 65 72 69 73 74 69 63 ]
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
			want: "09080300476f70686572",
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
			name: "read long char with handler -- 'A really long characte'",
			send: "0a1000",
			want: "0b41207265616c6c79206c6f6e67206368617261637465",
		},
		{
			name: "finish read long char with handler - '6973746963'",
			send: "0c10001700",
			want: "0d6973746963",
		},
		{
			name: "read long char with value -- 'A really long characte'",
			send: "0a1200",
			want: "0b41207265616c6c79206c6f6e67206368617261637465",
		},
		{
			name: "finish read long char with value - '6973746963'",
			send: "0c12001700",
			want: "0d6973746963",
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
