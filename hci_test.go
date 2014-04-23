package gatt

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type testshim struct {
	bytes.Buffer
}

func (t *testshim) Close() error           { return nil }
func (t *testshim) Wait() error            { return nil }
func (t *testshim) Signal(os.Signal) error { return nil }

func TestAdvertiseEIR(t *testing.T) {
	cases := []struct {
		adv     []byte
		scan    []byte
		want    string
		wanterr bool
	}{
		{adv: []byte{0x12, 0x34}, scan: []byte{0xAB, 0xCD}, want: "1234 abcd\n"},
		{scan: []byte{0xAB, 0xCD}, want: " abcd\n"},
		{adv: []byte{0x12, 0x34}, want: "1234 \n"},
		// data too long
		{adv: bytes.Repeat([]byte{0}, 32), wanterr: true},
		{scan: bytes.Repeat([]byte{0}, 32), wanterr: true},
	}

	shim := new(testshim)
	hci := newHCI(shim)
	for _, tt := range cases {
		shim.Buffer.Reset()
		err := hci.advertiseEIR(tt.adv, tt.scan)
		if tt.wanterr {
			if err != ErrEIRPacketTooLong {
				t.Errorf("AdvertiseEIR(%x, %x) got %v want ErrEIRPacketTooLong", tt.adv, tt.scan, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("AdvertiseEIR(%x, %x) unexpected error: %v", tt.adv, tt.scan, err)
		}
		if got := shim.Buffer.String(); got != tt.want {
			t.Errorf("AdvertiseEIR(%x, %x): got %q want %q", tt.adv, tt.scan, got, tt.want)
		}
	}
}

func TestNameScanResponsePacket(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{
			name: "gopher",
			want: "0709676f70686572",
		},
		{
			name: "gophergophergophergophergophergopher",
			want: "1e08676f70686572676f70686572676f70686572676f70686572676f706865",
		},
	}

	for _, tt := range cases {
		pack := nameScanResponsePacket(tt.name)
		if got := fmt.Sprintf("%x", pack); got != tt.want {
			t.Errorf("nameScanResponsePacket(%q): got %q want %q", tt.name, got, tt.want)
		}
	}
}

func TestServiceAdvertisingPacket(t *testing.T) {
	cases := []struct {
		uu   []UUID
		want string
		fit  []UUID // if different than uu
	}{
		{
			uu:   []UUID{UUID16(0xFAFE)},
			want: "0201060302fefa",
		},
		{
			uu:   []UUID{UUID16(0xFAFE), UUID16(0xFAF9)},
			want: "0201060302fefa0302f9fa",
		},
		{
			uu:   []UUID{MustParseUUID("ABABABABABABABABABABABABABABABAB")},
			want: "0201061106abababababababababababababababab",
		},
		{
			uu: []UUID{
				MustParseUUID("ABABABABABABABABABABABABABABABAB"),
				MustParseUUID("CDCDCDCDCDCDCDCDCDCDCDCDCDCDCDCD"),
			},
			want: "0201061106abababababababababababababababab",
			fit:  []UUID{MustParseUUID("ABABABABABABABABABABABABABABABAB")},
		},
		{
			uu: []UUID{
				UUID16(0xaaaa), UUID16(0xbbbb),
				UUID16(0xcccc), UUID16(0xdddd),
				UUID16(0xeeee), UUID16(0xffff),
				UUID16(0xaaaa), UUID16(0xbbbb),
			},
			want: "0201060302aaaa0302bbbb0302cccc0302dddd0302eeee0302ffff0302aaaa",
			fit: []UUID{
				UUID16(0xaaaa), UUID16(0xbbbb),
				UUID16(0xcccc), UUID16(0xdddd),
				UUID16(0xeeee), UUID16(0xffff),
				UUID16(0xaaaa),
			},
		},
	}

	for _, tt := range cases {
		pack, fit := serviceAdvertisingPacket(tt.uu)
		if got := fmt.Sprintf("%x", pack); got != tt.want {
			t.Errorf("serviceAdvertisingPacket(%x) packet: got %q want %q", tt.uu, got, tt.want)
		}
		if tt.fit == nil {
			tt.fit = tt.uu
		}
		if !reflect.DeepEqual(fit, tt.fit) {
			t.Errorf("serviceAdvertisingPacket(%x) fit: got %x want %x", tt.uu, fit, tt.fit)
		}
	}
}
