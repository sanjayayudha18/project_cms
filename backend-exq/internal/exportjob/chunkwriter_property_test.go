package exportjob

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// teeAccepted mirrors only the bytes the ChunkWriter accepted.
type teeAccepted struct {
	w   *ChunkWriter
	buf bytes.Buffer
}

func (t *teeAccepted) Write(p []byte) (int, error) {
	n, err := t.w.Write(p)
	if err == nil {
		t.buf.Write(p[:n])
	}
	return n, err
}

func randString(r *rand.Rand, max int) string {
	const alpha = "abc ,\"\n=+-@é"
	rs := []rune(alpha)
	n := r.IntN(max + 1)
	out := make([]rune, n)
	for i := range out {
		out[i] = rs[r.IntN(len(rs))]
	}
	return string(out)
}

func checkChunks(t *testing.T, s *fakeSink, w *ChunkWriter, want []byte, chunkSize int) {
	t.Helper()
	var got []byte
	for i, c := range s.chunks {
		if len(c) == 0 || len(c) > chunkSize {
			t.Fatalf("chunk %d len %d, chunkSize %d", i, len(c), chunkSize)
		}
		if s.seqs[i] != int32(i) {
			t.Fatalf("seq[%d] = %d", i, s.seqs[i])
		}
		got = append(got, c...)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("concatenated chunks differ from written bytes (%d vs %d)", len(got), len(want))
	}
	if w.Bytes() != int64(len(want)) || int(w.Chunks()) != len(s.chunks) {
		t.Fatalf("Bytes=%d Chunks=%d, want %d/%d", w.Bytes(), w.Chunks(), len(want), len(s.chunks))
	}
}

// Property 7 (writer side): chunks concatenate to exactly what was written.
func TestChunkWriterProperty(t *testing.T) {
	for seed := uint64(0); seed < 200; seed++ {
		r := rand.New(rand.NewPCG(seed, 7))
		fail := func(f string, a ...any) { t.Helper(); t.Fatalf("seed=%d: %s", seed, fmt.Sprintf(f, a...)) }

		chunkSize := 1 + r.IntN(64)
		s := &fakeSink{}
		w := newTestWriter(s, chunkSize, 1<<30)
		tee := &teeAccepted{w: w}
		header := []string{"id", "name"}
		cw, err := csvexport.NewWriter(tee, header)
		if err != nil {
			fail("NewWriter: %v", err)
		}
		rows := r.IntN(30) // 0 => header only
		for i := 0; i < rows; i++ {
			if err := cw.Write([]string{fmt.Sprint(i), randString(r, 40)}); err != nil {
				fail("row: %v", err)
			}
			if r.IntN(3) == 0 { // random write sizes via flush points
				if err := cw.Flush(); err != nil {
					fail("flush: %v", err)
				}
			}
			if r.IntN(5) == 0 { // raw writes of random size straight to the writer
				raw := []byte(randString(r, 100))
				if _, err := tee.Write(raw); err != nil {
					fail("raw: %v", err)
				}
			}
		}
		if err := cw.Flush(); err != nil {
			fail("flush: %v", err)
		}
		if err := w.Close(); err != nil {
			fail("close: %v", err)
		}
		if rows == 0 && len(s.chunks) != 1 && tee.buf.Len() <= chunkSize {
			fail("header-only file has %d chunks, want 1", len(s.chunks))
		}
		func() {
			defer func() {
				if t.Failed() {
					t.Logf("failing seed=%d", seed)
				}
			}()
			checkChunks(t, s, w, tee.buf.Bytes(), chunkSize)
		}()
	}
}

func TestChunkWriterHeaderOnlyIsOneChunk(t *testing.T) {
	s := &fakeSink{}
	w := newTestWriter(s, 1<<20, 1<<30)
	cw, err := csvexport.NewWriter(w, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := cw.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if len(s.chunks) != 1 || w.Chunks() != 1 {
		t.Fatalf("chunks = %d, want 1", len(s.chunks))
	}
}

func TestChunkWriterLimitProperty(t *testing.T) {
	for seed := uint64(0); seed < 200; seed++ {
		r := rand.New(rand.NewPCG(seed, 11))
		chunkSize := 1 + r.IntN(64)
		maxBytes := int64(1 + r.IntN(300))
		s := &fakeSink{}
		w := newTestWriter(s, chunkSize, maxBytes)
		tee := &teeAccepted{w: w}
		var hitErr error
		for i := 0; i < 100 && hitErr == nil; i++ {
			_, hitErr = tee.Write([]byte(randString(r, 50)))
		}
		if hitErr != nil && !errors.Is(hitErr, ErrFileTooLarge) {
			t.Fatalf("seed=%d: err = %v", seed, hitErr)
		}
		if hitErr == nil {
			// total produced never exceeded the limit; nothing to assert beyond bound
			if w.Bytes() > maxBytes {
				t.Fatalf("seed=%d: Bytes %d > max %d", seed, w.Bytes(), maxBytes)
			}
			continue
		}
		if w.Bytes() > maxBytes {
			t.Fatalf("seed=%d: Bytes %d > max %d", seed, w.Bytes(), maxBytes)
		}
		var stored int64
		for _, c := range s.chunks {
			stored += int64(len(c))
		}
		if stored > maxBytes {
			t.Fatalf("seed=%d: stored %d > max %d", seed, stored, maxBytes)
		}
		if err := w.Close(); !errors.Is(err, ErrFileTooLarge) {
			t.Fatalf("seed=%d: close err = %v", seed, err)
		}
	}
}

func FuzzChunkWriter(f *testing.F) {
	f.Add([]byte("hello,world\n"), uint8(3))
	f.Add([]byte{}, uint8(1))
	f.Add(bytes.Repeat([]byte("x"), 200), uint8(64))
	f.Fuzz(func(t *testing.T, data []byte, cs uint8) {
		chunkSize := int(cs%64) + 1
		s := &fakeSink{}
		w := newTestWriter(s, chunkSize, 1<<30)
		// split data at deterministic pseudo-random points derived from it
		rest := data
		for len(rest) > 0 {
			n := int(rest[0])%17 + 1
			if n > len(rest) {
				n = len(rest)
			}
			if _, err := w.Write(rest[:n]); err != nil {
				t.Fatal(err)
			}
			rest = rest[n:]
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		checkChunks(t, s, w, data, chunkSize)
	})
}
