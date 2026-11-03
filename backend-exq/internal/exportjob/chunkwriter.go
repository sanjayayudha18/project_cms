package exportjob

import (
	"context"
	"errors"
	"fmt"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

var (
	// ErrFileTooLarge is returned when the file would exceed MaxBytes.
	ErrFileTooLarge = errors.New("export file too large")
	// ErrTooManyRows is returned by the worker's emit when rows exceed MaxRows.
	ErrTooManyRows = errors.New("export has too many rows")
)

// ChunkSink stores one numbered chunk of a job's file.
type ChunkSink interface {
	InsertChunk(ctx context.Context, jobID int64, seq int32, data []byte) error
}

// QueriesSink adapts the sqlc Queries to ChunkSink.
type QueriesSink struct{ Q *repository.Queries }

// InsertChunk implements ChunkSink.
func (s QueriesSink) InsertChunk(ctx context.Context, jobID int64, seq int32, data []byte) error {
	return s.Q.InsertExportJobChunk(ctx, repository.InsertExportJobChunkParams{
		ExportJobID: jobID, Seq: seq, Data: data,
	})
}

// ChunkWriter is an io.Writer that stores its input as ChunkSize chunks
// (seq 0,1,2...) and enforces the file size limit. Not safe for concurrent use.
type ChunkWriter struct {
	ctx       context.Context
	sink      ChunkSink
	jobID     int64
	chunkSize int
	maxBytes  int64
	buf       []byte
	total     int64
	seq       int32
	err       error // sticky
}

// NewChunkWriter returns a writer using the package limits.
func NewChunkWriter(ctx context.Context, sink ChunkSink, jobID int64) *ChunkWriter {
	return &ChunkWriter{
		ctx: ctx, sink: sink, jobID: jobID,
		chunkSize: ChunkSize, maxBytes: MaxBytes,
		buf: make([]byte, 0, ChunkSize),
	}
}

// Write buffers p, flushing full chunks. Once it fails it keeps returning the same error.
func (w *ChunkWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.total+int64(len(p)) > w.maxBytes {
		w.err = ErrFileTooLarge
		return 0, w.err
	}
	w.total += int64(len(p))
	rest := p
	for len(rest) > 0 {
		n := min(w.chunkSize-len(w.buf), len(rest))
		w.buf = append(w.buf, rest[:n]...)
		rest = rest[n:]
		if len(w.buf) == w.chunkSize {
			if err := w.flush(); err != nil {
				return 0, err
			}
		}
	}
	return len(p), nil
}

// Close flushes the remaining buffer as the last chunk (nothing if empty).
func (w *ChunkWriter) Close() error {
	if w.err != nil {
		return w.err
	}
	if len(w.buf) == 0 {
		return nil
	}
	return w.flush()
}

// Bytes is the total number of bytes accepted so far.
func (w *ChunkWriter) Bytes() int64 { return w.total }

// Chunks is the number of chunks stored so far.
func (w *ChunkWriter) Chunks() int32 { return w.seq }

func (w *ChunkWriter) flush() error {
	// Copy: the buffer is reused and the sink may retain the slice.
	data := append([]byte(nil), w.buf...)
	if err := w.sink.InsertChunk(w.ctx, w.jobID, w.seq, data); err != nil {
		w.err = fmt.Errorf("insert chunk %d: %w", w.seq, err)
		return w.err
	}
	w.seq++
	w.buf = w.buf[:0]
	return nil
}
