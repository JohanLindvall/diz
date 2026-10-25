package hashzip

import (
	"bytes"
	"io"
	"testing"

	"github.com/klauspost/compress/flate"
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

	got, err := io.ReadAll(flate.NewReader(&compressed))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, src) {
		t.Fatalf("round trip mismatch: got %d bytes, want %d", len(got), len(src))
	}
}
