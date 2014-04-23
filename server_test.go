package gatt

import "testing"

func TestCleanHCIDevice(t *testing.T) {
	cases := []struct {
		hci  string
		want string
	}{
		{hci: "", want: ""},
		{hci: "1", want: "1"},
		{hci: "hci1", want: "1"},
		{hci: "hci", want: ""},
		{hci: "hci2.5", want: ""},
		{hci: "h2", want: ""},
		{hci: "-1", want: ""},
		{hci: "hci-1", want: ""},
	}

	for _, tt := range cases {
		if got := cleanHCIDevice(tt.hci); got != tt.want {
			t.Errorf("cleanHCIDevice(%q): got %q want %q", tt.hci, got, tt.want)
		}
	}
}
