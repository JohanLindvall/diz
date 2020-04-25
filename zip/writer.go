package zip

import (
	gozip "archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
)

// Writer holds the data for writing to a zip archive, ignoring duplicate file names
type Writer struct {
	io.Writer
	writer  *gozip.Writer
	seen    map[string]bool
	verbose bool
	method  uint16
}

// NewWriter returns a new writer
func NewWriter(w io.Writer) *Writer {
	return NewWriterMethod(w, gozip.Deflate)
}

// NewWriterMethod returns a new writer using a custom method
func NewWriterMethod(w io.Writer, method uint16) *Writer {
	return &Writer{writer: gozip.NewWriter(w), seen: make(map[string]bool, 0), verbose: w != os.Stdout, method: method}
}

// Exists returns true if the given file exists in the archive
func (w *Writer) Exists(name string) bool {
	return w.seen[name]
}

// Create creates a new entry and returns a writer
func (w *Writer) Create(name string) (io.Writer, error) {
	return w.CreateHeader(&gozip.FileHeader{Name: name, Method: w.method})
}

// CreateHeader creates a new header entry and returns a writer
func (w *Writer) CreateHeader(fh *gozip.FileHeader) (io.Writer, error) {
	copy := *fh
	copy.Method = w.method
	if w.seen[copy.Name] {
		return nil, errors.New("file exists")
	}
	w.seen[copy.Name] = true
	if w.verbose {
		fmt.Printf("Writing '%s'\n", copy.Name)
	}
	return w.writer.CreateHeader(&copy)
}

// Write implements the standard write interface
func (w *Writer) Write(p []byte) (n int, err error) {
	n, err = w.Write(p)
	return
}

// Close closes the zip writer
func (w *Writer) Close() error {
	return w.writer.Close()
}
