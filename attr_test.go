package gatt

import (
	"reflect"
	"testing"
)

func TestHandleRangeAt(t *testing.T) {
	r := &attrRange{
		aa:   make([]attr, 3),
		base: 4,
	}
	r.aa[0].h = 4
	r.aa[1].h = 5
	r.aa[2].h = 6

	for _, h := range [...]uint16{0, 2, 3, 7, 8, 100} {
		if _, ok := r.At(h); ok {
			t.Errorf("At(%d) should return !ok", h)
		}
	}

	for _, h := range [...]uint16{4, 5, 6} {
		if _, ok := r.At(h); !ok {
			t.Errorf("At(%d) should return ok", h)
		}
		if a, _ := r.At(h); a.h != h {
			t.Errorf("At(%d) returned wrong attr, got %d want %d", h, a.h, h)
		}
	}
}

func TestHandleRangeSubrange(t *testing.T) {
	r := &attrRange{
		aa: make([]attr, 3),
	}

	cases := []struct {
		start, end uint16
		base       uint16
		want       []attr
	}{
		{start: 0, end: 3, base: 4, want: []attr{}},
		{start: 0, end: 4, base: 4, want: []attr{r.aa[0]}},
		{start: 0, end: 5, base: 4, want: []attr{r.aa[0], r.aa[1]}},
		{start: 4, end: 5, base: 4, want: []attr{r.aa[0], r.aa[1]}},
		{start: 4, end: 6, base: 4, want: []attr{r.aa[0], r.aa[1], r.aa[2]}},
		{start: 4, end: 100, base: 4, want: []attr{r.aa[0], r.aa[1], r.aa[2]}},
		{start: 5, end: 100, base: 4, want: []attr{r.aa[1], r.aa[2]}},
		{start: 5, end: 6, base: 4, want: []attr{r.aa[1], r.aa[2]}},
		{start: 5, end: 5, base: 4, want: []attr{r.aa[1]}},
		{start: 6, end: 6, base: 4, want: []attr{r.aa[2]}},
		{start: 6, end: 100, base: 4, want: []attr{r.aa[2]}},
		{start: 7, end: 100, base: 4, want: []attr{}},
		{start: 100, end: 1000, base: 4, want: []attr{}},
		{start: 1000, end: 100, base: 4, want: []attr{}},
		{start: 5, end: 1, base: 4, want: []attr{}},
		{start: 1, end: 65535, base: 4, want: []attr{r.aa[0], r.aa[1], r.aa[2]}},
		{start: 1, end: 65535, base: 0, want: []attr{r.aa[1], r.aa[2]}},
	}

	for _, tt := range cases {
		r.base = tt.base
		if got := r.Subrange(tt.start, tt.end); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Range(%d, %d): got %v want %v", tt.start, tt.end, got, tt.want)
		}
	}
}
