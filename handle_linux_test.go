package gatt

import (
	"reflect"
	"testing"
)

func TestHandleRangeAt(t *testing.T) {
	r := &handleRange{
		hh:   make([]handle, 3),
		base: 4,
	}
	r.hh[0].n = 4
	r.hh[1].n = 5
	r.hh[2].n = 6

	for _, n := range [...]uint16{0, 2, 3, 7, 8, 100} {
		if _, ok := r.At(n); ok {
			t.Errorf("At(%d) should return !ok", n)
		}
	}

	for _, n := range [...]uint16{4, 5, 6} {
		if _, ok := r.At(n); !ok {
			t.Errorf("At(%d) should return ok", n)
		}
		if h, _ := r.At(n); h.n != n {
			t.Errorf("At(%d) returned wrong handle, got %d want %d", n, h.n, n)
		}
	}
}

func TestHandleRangeSubrange(t *testing.T) {
	r := &handleRange{
		hh: make([]handle, 3),
	}

	cases := []struct {
		start, end uint16
		base       uint16
		want       []handle
	}{
		{start: 0, end: 3, base: 4, want: []handle{}},
		{start: 0, end: 4, base: 4, want: []handle{r.hh[0]}},
		{start: 0, end: 5, base: 4, want: []handle{r.hh[0], r.hh[1]}},
		{start: 4, end: 5, base: 4, want: []handle{r.hh[0], r.hh[1]}},
		{start: 4, end: 6, base: 4, want: []handle{r.hh[0], r.hh[1], r.hh[2]}},
		{start: 4, end: 100, base: 4, want: []handle{r.hh[0], r.hh[1], r.hh[2]}},
		{start: 5, end: 100, base: 4, want: []handle{r.hh[1], r.hh[2]}},
		{start: 5, end: 6, base: 4, want: []handle{r.hh[1], r.hh[2]}},
		{start: 5, end: 5, base: 4, want: []handle{r.hh[1]}},
		{start: 6, end: 6, base: 4, want: []handle{r.hh[2]}},
		{start: 6, end: 100, base: 4, want: []handle{r.hh[2]}},
		{start: 7, end: 100, base: 4, want: []handle{}},
		{start: 100, end: 1000, base: 4, want: []handle{}},
		{start: 1000, end: 100, base: 4, want: []handle{}},
		{start: 5, end: 1, base: 4, want: []handle{}},
		{start: 1, end: 65535, base: 4, want: []handle{r.hh[0], r.hh[1], r.hh[2]}},
		{start: 1, end: 65535, base: 0, want: []handle{r.hh[1], r.hh[2]}},
	}

	for _, tt := range cases {
		r.base = tt.base
		if got := r.Subrange(tt.start, tt.end); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Range(%d, %d): got %v want %v", tt.start, tt.end, got, tt.want)
		}
	}
}
