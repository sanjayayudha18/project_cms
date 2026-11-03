package exportjob

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

type fakeSink struct {
	chunks [][]byte
	seqs   []int32
}

func (f *fakeSink) InsertChunk(_ context.Context, _ int64, seq int32, data []byte) error {
	f.seqs = append(f.seqs, seq)
	f.chunks = append(f.chunks, data)
	return nil
}

func newTestWriter(s *fakeSink, chunk int, max int64) *ChunkWriter {
	w := NewChunkWriter(context.Background(), s, 1)
	w.chunkSize, w.maxBytes = chunk, max
	return w
}

func TestChunkWriterBoundaries(t *testing.T) {
	cases := []struct {
		name   string
		size   int
		chunks []int
	}{
		{"empty", 0, nil},
		{"small", 3, []int{3}},
		{"exact", 8, []int{8}},
		{"one over", 9, []int{8, 1}},
		{"two and a half", 20, []int{8, 8, 4}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &fakeSink{}
			w := newTestWriter(s, 8, 1000)
			in := bytes.Repeat([]byte("x"), c.size)
			if _, err := w.Write(in); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			if len(s.chunks) != len(c.chunks) || int(w.Chunks()) != len(c.chunks) {
				t.Fatalf("chunks = %d, want %d", len(s.chunks), len(c.chunks))
			}
			for i, n := range c.chunks {
				if len(s.chunks[i]) != n || s.seqs[i] != int32(i) {
					t.Fatalf("chunk %d: len %d seq %d", i, len(s.chunks[i]), s.seqs[i])
				}
			}
			if w.Bytes() != int64(c.size) {
				t.Fatalf("Bytes = %d", w.Bytes())
			}
		})
	}
}

func TestChunkWriterTooLargeIsSticky(t *testing.T) {
	s := &fakeSink{}
	w := newTestWriter(s, 8, 10)
	if _, err := w.Write([]byte("0123456")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("abcd")); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("err = %v", err)
	}
	if _, err := w.Write([]byte("a")); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("not sticky: %v", err)
	}
	if err := w.Close(); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("close err = %v", err)
	}
}

func TestChunkWriterCopiesBuffer(t *testing.T) {
	s := &fakeSink{}
	w := newTestWriter(s, 4, 100)
	_, _ = w.Write([]byte("aaaa"))
	_, _ = w.Write([]byte("bbbb"))
	if string(s.chunks[0]) != "aaaa" || string(s.chunks[1]) != "bbbb" {
		t.Fatalf("chunks aliased: %q %q", s.chunks[0], s.chunks[1])
	}
}
