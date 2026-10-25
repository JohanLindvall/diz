package hashzip

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"

	"github.com/klauspost/compress/flate"
	"github.com/klauspost/compress/zip"
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
	_, exists := w.hashes[name]
	return exists || w.h != nil && w.current == name
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
	if w.Exists(copy.Name) {
		return nil, errors.New("file exists")
	}
	writer, err := w.writer.CreateHeader(&copy)
	if err != nil {
		return nil, err
	}
	if w.verbose {
		fmt.Printf("Writing '%s'\n", copy.Name)
	}
	w.current = copy.Name
	w.h = sha256.New()
	return io.MultiWriter(writer, w.h), nil
}

func (w *Writer) end() {
	if w.h != nil {
		w.hashes[w.current] = fmt.Sprintf("%x", w.h.Sum(nil))
		w.h = nil
	}
}

// Close closes the zip writer
func (w *Writer) Close() (err error) {
	defer func() {
		if closeErr := w.writer.Close(); err == nil {
			err = closeErr
		}
	}()
	w.end()
	wr, err := w.writer.CreateHeader(&zip.FileHeader{Name: ".hashes", Method: zip.Deflate})
	if err != nil {
		return err
	}
	data, err := json.Marshal(w.hashes)
	if err != nil {
		return err
	}
	_, err = wr.Write(data)
	return err
}

// Copy copies the compressed contents of the source file to this archive under
// the given name, bypassing decompression and recompression.
func (w *Writer) Copy(name string, zf *File) error {
	w.end()
	if w.Exists(name) {
		return errors.New("file exists")
	}
	if w.verbose {
		fmt.Printf("Copying '%s'\n", name)
	}
	// zip.Writer.Copy always reuses the source name; copy the raw entry
	// ourselves so it can be stored under a different name.
	raw, err := zf.file.OpenRaw()
	if err != nil {
		return err
	}
	header := zf.file.FileHeader
	header.Name = name
	entry, err := w.writer.CreateRaw(&header)
	if err != nil {
		return err
	}
	if _, err = io.Copy(entry, raw); err != nil {
		return err
	}
	w.hashes[name] = zf.Hash
	return nil
}
