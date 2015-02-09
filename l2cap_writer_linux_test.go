package gatt

import (
	"bytes"
	"testing"
)

// TODO: More test coverage.

func TestL2capWriterChunk(t *testing.T) {
	cases := []struct {
		mtu   uint16
		head  int
		chunk int
		ok    bool
	}{
		{mtu: 5, head: 0, chunk: 4, ok: true},
		{mtu: 5, head: 0, chunk: 5, ok: true},
		{mtu: 5, head: 0, chunk: 6, ok: false},
		{mtu: 5, head: 1, chunk: 3, ok: true},
		{mtu: 5, head: 1, chunk: 4, ok: true},
		{mtu: 5, head: 1, chunk: 5, ok: false},
	}

	for _, tt := range cases {
		w := newL2capWriter(tt.mtu)
		var want []byte
		for i := 0; i < tt.head; i++ {
			w.WriteByteFit(byte(i))
			want = append(want, byte(i))
		}
		w.Chunk()
		for i := 0; i < tt.chunk; i++ {
			w.WriteByteFit(byte(i))
			if tt.ok {
				want = append(want, byte(i))
			}
		}
		ok := w.Commit()
		if ok != tt.ok {
			t.Errorf("Chunk(%d %d %d) commit: got %t want %t", tt.mtu, tt.head, tt.chunk, ok, tt.ok)
			continue
		}
		if !bytes.Equal(want, w.Bytes()) {
			t.Errorf("Chunk(%d %d %d) write: got %x want %x", tt.mtu, tt.head, tt.chunk, w.Bytes(), want)
		}
	}
}

func TestL2capWriterPanicDoubleChunk(t *testing.T) {
	defer func() { recover() }()
	w := newL2capWriter(5)
	w.Chunk()
	w.Chunk()
	t.Errorf("l2capWriter should panic on double-chunk")
}

func TestL2capWriterPanicCommitBeforeChunk(t *testing.T) {
	defer func() { recover() }()
	w := newL2capWriter(5)
	w.Commit()
	t.Errorf("l2capWriter should panic on commit-before-chunk")
}

func TestL2capWriterPanicDoubleCommit(t *testing.T) {
	defer func() { recover() }()
	w := newL2capWriter(5)
	w.Chunk()
	w.Commit()
	w.Commit()
	t.Errorf("l2capWriter should panic on double-commit")
}

func BenchmarkWriteUint16(b *testing.B) {
	for i := 0; i < b.N; i++ {
		w := newL2capWriter(17)
		w.WriteUint16Fit(0)
	}
}
