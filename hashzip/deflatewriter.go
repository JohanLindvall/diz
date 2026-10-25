package hashzip

import (
	"io"

	"github.com/klauspost/pgzip"
)

// Compression levels supported by DeflateWriter.
const (
	NoCompression       = pgzip.NoCompression
	BestSpeed           = pgzip.BestSpeed
	BestCompression     = pgzip.BestCompression
	DefaultCompression  = pgzip.DefaultCompression
	ConstantCompression = pgzip.ConstantCompression
	HuffmanOnly         = pgzip.HuffmanOnly
)

// DeflateWriter uses pgzip's parallel compressor to write a raw DEFLATE stream,
// as required by ZIP. The upstream writer is private so its optional GZIP
// header fields cannot be set: the header is always exactly ten bytes.
type DeflateWriter struct {
	writer *pgzip.Writer
	output gzipBodyWriter
	closed bool
	err    error
}

// NewDeflateWriter returns a writer using DefaultCompression.
// Call Close to finish the stream without closing the underlying writer.
func NewDeflateWriter(w io.Writer) *DeflateWriter {
	z, _ := NewDeflateWriterLevel(w, DefaultCompression)
	return z
}

// NewDeflateWriterLevel returns a writer using the given compression level.
func NewDeflateWriterLevel(w io.Writer, level int) (*DeflateWriter, error) {
	z := &DeflateWriter{output: gzipBodyWriter{w: w, header: 10}}
	var err error
	z.writer, err = pgzip.NewWriterLevel(&z.output, level)
	if err != nil {
		return nil, err
	}
	return z, nil
}

// SetConcurrency sets the block size and number of parallel compression blocks.
// Call it before the first Write or Flush.
func (z *DeflateWriter) SetConcurrency(blockSize, blocks int) error {
	return z.writer.SetConcurrency(blockSize, blocks)
}

// Reset discards the current stream and reuses the compressor for w.
func (z *DeflateWriter) Reset(w io.Writer) {
	// Reset waits for upstream's outstanding writes before we replace output.
	z.writer.Reset(&z.output)
	z.output = gzipBodyWriter{w: w, header: 10}
	z.closed = false
	z.err = nil
}

// Write buffers p for compression. Flush or Close reports asynchronous errors.
func (z *DeflateWriter) Write(p []byte) (int, error) {
	if z.err != nil {
		return 0, z.err
	}
	if z.closed {
		return 0, io.ErrClosedPipe
	}
	return z.writer.Write(p)
}

// Flush writes all pending compressed data without ending the stream.
func (z *DeflateWriter) Flush() error {
	if z.err != nil || z.closed {
		return z.err
	}
	if z.err = z.writer.Flush(); z.err == nil {
		// No trailer exists yet, so the held bytes are all DEFLATE data.
		z.err = z.output.flush()
	}
	return z.err
}

// UncompressedSize returns the number of uncompressed bytes written.
func (z *DeflateWriter) UncompressedSize() int {
	return z.writer.UncompressedSize()
}

// Close finishes the stream without closing the underlying writer.
func (z *DeflateWriter) Close() error {
	if z.err != nil || z.closed {
		return z.err
	}
	z.closed = true
	z.err = z.writer.Close()
	// The final eight bytes held by output are the GZIP CRC32 and size trailer.
	return z.err
}

// gzipBodyWriter removes the fixed GZIP header and holds back the last eight
// bytes. It does not rely on upstream's header, body or trailer write boundaries.
type gzipBodyWriter struct {
	w      io.Writer
	header int
	tail   [8]byte
	n      int
}

func (w *gzipBodyWriter) Write(p []byte) (int, error) {
	skipped := min(w.header, len(p))
	w.header -= skipped
	p = p[skipped:]

	// Emit everything except the final eight bytes of tail + p.
	emit := max(0, w.n+len(p)-len(w.tail))
	fromTail := min(emit, w.n)
	if fromTail > 0 {
		if _, err := writeFull(w.w, w.tail[:fromTail]); err != nil {
			return skipped, err
		}
		w.n = copy(w.tail[:], w.tail[fromTail:w.n])
	}
	fromP := emit - fromTail
	if fromP > 0 {
		if n, err := writeFull(w.w, p[:fromP]); err != nil {
			return skipped + n, err
		}
	}
	w.n += copy(w.tail[w.n:], p[fromP:])
	return skipped + len(p), nil
}

func (w *gzipBodyWriter) flush() error {
	if w.n == 0 {
		return nil
	}
	_, err := writeFull(w.w, w.tail[:w.n])
	w.n = 0
	return err
}

func writeFull(w io.Writer, p []byte) (int, error) {
	n, err := w.Write(p)
	if n != len(p) && err == nil {
		err = io.ErrShortWrite
	}
	return n, err
}
