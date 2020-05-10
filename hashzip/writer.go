package hashzip

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"

	"github.com/klauspost/compress/flate"
)

// Writer holds the data for writing to a zip archive, ignoring duplicate file names
type Writer struct {
	io.Writer
	writer  *zip.Writer
	verbose bool
	h       hash.Hash
	hashes  map[string]string
	current string
}

// NewWriter returns a new writer
func NewWriter(w io.Writer) *Writer {
	return NewWriterLevel(w, flate.DefaultCompression)
}

// NewWriterLevel returns a new writer using the specified level
func NewWriterLevel(w io.Writer, level int) *Writer {
	zw := zip.NewWriter(w)
	zw.RegisterCompressor(zip.Deflate, func(wr io.Writer) (io.WriteCloser, error) {
		return NewDeflateWriterLevel(wr, level)
	})

	return &Writer{writer: zw, verbose: w != os.Stdout, hashes: make(map[string]string, 0)}
}

// Exists returns true if the given file exists in the archive
func (w *Writer) Exists(name string) bool {
	return w.hashes[name] != ""
}

// Create creates a new entry and returns a writer
func (w *Writer) Create(name string) (io.Writer, error) {
	return w.CreateHeader(&zip.FileHeader{Name: name})
}

// CreateHeader creates a new header entry and returns a writer
func (w *Writer) CreateHeader(fh *zip.FileHeader) (io.Writer, error) {
	w.end()
	copy := *fh
	copy.Method = zip.Deflate
	if w.hashes[copy.Name] != "" {
		return nil, errors.New("file exists")
	}
	w.current = copy.Name
	if w.verbose {
		fmt.Printf("Writing '%s'\n", copy.Name)
	}
	w.h = sha256.New()
	if writer, err := w.writer.CreateHeader(&copy); err == nil {
		return io.MultiWriter(writer, w.h), err
	} else {
		return nil, err
	}
}

func (w *Writer) end() {
	if w.h != nil {
		w.hashes[w.current] = fmt.Sprintf("%x", w.h.Sum(nil))
		w.h = nil
	}
}

// Close closes the zip writer
func (w *Writer) Close() error {
	w.end()
	if wr, err := w.writer.CreateHeader(&zip.FileHeader{Name: ".hashes", Method: zip.Deflate}); err == nil {
		data, _ := json.Marshal(w.hashes)
		io.Copy(wr, bytes.NewReader(data))
	}
	return w.writer.Close()
}
