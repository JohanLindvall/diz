package hashzip

import (
	"bytes"
	"compress/flate"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"testing"

	"github.com/klauspost/pgzip"
)

// TestDeflateWriterRoundTrip pushes several megabytes through the parallel
// DeflateWriter so more than one block is produced (blockSize is 1 MB), then
// inflates the result to confirm the raw DEFLATE stream is valid and lossless.
func TestDeflateWriterRoundTrip(t *testing.T) {
	// ~5 MB of semi-compressible data spanning multiple 1 MB blocks.
	src := make([]byte, 5<<20)
	for i := range src {
		src[i] = byte(i*2654435761 + i>>3)
	}

	var compressed bytes.Buffer
	w, err := NewDeflateWriterLevel(&compressed, DefaultCompression)
	if err != nil {
		t.Fatal(err)
	}
	// Write in odd-sized chunks to exercise the block-splitting logic.
	for off := 0; off < len(src); off += 7000 {
		end := off + 7000
		if end > len(src) {
			end = len(src)
		}
		if _, err := w.Write(src[off:end]); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if w.UncompressedSize() != len(src) {
		t.Fatalf("UncompressedSize = %d, want %d", w.UncompressedSize(), len(src))
	}

	assertDeflateStream(t, compressed.Bytes(), src)
}

func assertDeflateStream(t *testing.T, compressed, want []byte) {
	t.Helper()
	input := bytes.NewReader(compressed)
	r := flate.NewReader(input)
	defer r.Close()
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("round trip mismatch: got %d bytes, want %d", len(got), len(want))
	}
	if input.Len() != 0 {
		t.Fatalf("%d trailing bytes after DEFLATE stream", input.Len())
	}
}

func TestDeflateWriterMatchesUpstream(t *testing.T) {
	const blockSize = 32 << 10
	random := make([]byte, 3*blockSize+1)
	rand.New(rand.NewSource(1)).Read(random)
	for level := HuffmanOnly; level <= BestCompression; level++ {
		for _, size := range []int{0, 1, blockSize - 1, blockSize, blockSize + 1, len(random)} {
			t.Run(fmt.Sprintf("level=%d/size=%d", level, size), func(t *testing.T) {
				src := random[:size]
				var raw, framed bytes.Buffer
				w, err := NewDeflateWriterLevel(&raw, level)
				if err != nil {
					t.Fatal(err)
				}
				upstream, err := pgzip.NewWriterLevel(&framed, level)
				if err != nil {
					t.Fatal(err)
				}
				for _, compressor := range []interface {
					io.WriteCloser
					SetConcurrency(int, int) error
				}{w, upstream} {
					if err := compressor.SetConcurrency(blockSize, 3); err != nil {
						t.Fatal(err)
					}
					if n, err := compressor.Write(src); err != nil || n != len(src) {
						t.Fatalf("Write = %d, %v", n, err)
					}
					if err := compressor.Close(); err != nil {
						t.Fatal(err)
					}
				}
				if !bytes.Equal(raw.Bytes(), framed.Bytes()[10:framed.Len()-8]) {
					t.Fatal("output differs from upstream's DEFLATE payload")
				}
				assertDeflateStream(t, raw.Bytes(), src)
			})
		}
	}
}

func TestDeflateWriterFlush(t *testing.T) {
	var raw, framed, src bytes.Buffer
	w := NewDeflateWriter(&raw)
	upstream := pgzip.NewWriter(&framed)
	for _, p := range [][]byte{nil, bytes.Repeat([]byte("first"), 1000), nil, []byte("last")} {
		src.Write(p)
		for _, compressor := range []interface {
			io.Writer
			Flush() error
		}{w, upstream} {
			if _, err := compressor.Write(p); err != nil {
				t.Fatal(err)
			}
			if err := compressor.Flush(); err != nil {
				t.Fatal(err)
			}
		}
		if !bytes.Equal(raw.Bytes(), framed.Bytes()[10:]) {
			t.Fatal("Flush held back compressed data")
		}
		reader := flate.NewReader(bytes.NewReader(raw.Bytes()))
		got, err := io.ReadAll(reader)
		reader.Close()
		if !errors.Is(err, io.ErrUnexpectedEOF) || !bytes.Equal(got, src.Bytes()) {
			t.Fatalf("flushed stream: decoded %d of %d bytes, err = %v", len(got), src.Len(), err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := upstream.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw.Bytes(), framed.Bytes()[10:framed.Len()-8]) {
		t.Fatal("output after Flush and Close differs from upstream's DEFLATE payload")
	}
	assertDeflateStream(t, raw.Bytes(), src.Bytes())
	closedSize := raw.Len()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if raw.Len() != closedSize {
		t.Fatal("Close or Flush emitted data after Close")
	}
	if _, err := w.Write([]byte("closed")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Write after Close = %v", err)
	}
}

func TestDeflateWriterReset(t *testing.T) {
	for _, state := range []string{"unused", "open", "closed"} {
		t.Run(state, func(t *testing.T) {
			w := NewDeflateWriter(io.Discard)
			if state != "unused" {
				if err := w.SetConcurrency(32<<10, 2); err != nil {
					t.Fatal(err)
				}
				if _, err := w.Write(bytes.Repeat([]byte("old"), 50000)); err != nil {
					t.Fatal(err)
				}
			}
			if state == "closed" {
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
			}
			var out bytes.Buffer
			w.Reset(&out)
			if w.UncompressedSize() != 0 {
				t.Fatal("Reset did not clear UncompressedSize")
			}
			src := []byte("new stream")
			if _, err := w.Write(src); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			assertDeflateStream(t, out.Bytes(), src)
		})
	}
}

type deflateTestWriter func([]byte) (int, error)

func (f deflateTestWriter) Write(p []byte) (int, error) { return f(p) }

func TestDeflateWriterErrors(t *testing.T) {
	failure := errors.New("destination failed")
	for _, operation := range []string{"Flush", "Close"} {
		for _, short := range []bool{false, true} {
			// The first write emits the body; the second drains the held bytes.
			for _, failAt := range []int{1, 2} {
				t.Run(fmt.Sprintf("%s/short=%t/write=%d", operation, short, failAt), func(t *testing.T) {
					calls := 0
					w := NewDeflateWriter(deflateTestWriter(func(p []byte) (int, error) {
						calls++
						if calls != failAt {
							return len(p), nil
						}
						if short {
							return len(p) / 2, nil
						}
						return len(p) / 2, failure
					}))
					if _, err := w.Write([]byte("data to compress")); err != nil {
						t.Fatal(err)
					}
					wantErr := failure
					if short {
						wantErr = io.ErrShortWrite
					}
					finish := w.Close
					if operation == "Flush" {
						finish = w.Flush
					}
					if err := finish(); !errors.Is(err, wantErr) {
						t.Fatalf("%s = %v, want %v", operation, err, wantErr)
					}
					if _, err := w.Write(nil); !errors.Is(err, wantErr) {
						t.Fatalf("Write lost error: %v", err)
					}
					if err := w.Flush(); !errors.Is(err, wantErr) {
						t.Fatalf("Flush lost error: %v", err)
					}
					if err := w.Close(); !errors.Is(err, wantErr) {
						t.Fatalf("Close lost error: %v", err)
					}
					var out bytes.Buffer
					w.Reset(&out)
					if err := w.Close(); err != nil {
						t.Fatalf("Close after Reset = %v", err)
					}
					assertDeflateStream(t, out.Bytes(), nil)
				})
			}
		}
	}
}

func TestDeflateWriterInvalidLevel(t *testing.T) {
	for _, level := range []int{ConstantCompression - 1, BestCompression + 1} {
		if w, err := NewDeflateWriterLevel(io.Discard, level); err == nil || w != nil {
			t.Fatalf("level %d: writer = %v, err = %v", level, w, err)
		}
	}
}

func TestGzipBodyWriterWriteBoundaries(t *testing.T) {
	var framed bytes.Buffer
	w := pgzip.NewWriter(&framed)
	if _, err := w.Write([]byte("a small payload")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	want := framed.Bytes()[10 : framed.Len()-8]
	for size := 1; size <= framed.Len(); size++ {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			var out bytes.Buffer
			body := gzipBodyWriter{w: &out, header: 10}
			for remaining := framed.Bytes(); len(remaining) > 0; {
				chunk := remaining[:min(size, len(remaining))]
				if n, err := body.Write(chunk); err != nil || n != len(chunk) {
					t.Fatalf("Write = %d, %v", n, err)
				}
				remaining = remaining[len(chunk):]
			}
			if !bytes.Equal(out.Bytes(), want) {
				t.Fatalf("output = %x, want %x", out.Bytes(), want)
			}
		})
	}
}
