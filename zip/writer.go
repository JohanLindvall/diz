package zip

import (
	gozip "archive/zip"
	"errors"
	"io"
)

// Writer holds the data for writing to a zip archive, ignoring duplicate file names
type Writer struct {
	io.Writer
	writer *gozip.Writer
	seen   map[string]bool
}

// NewWriter returns a new writer
func NewWriter(w io.Writer) *Writer {
	return &Writer{writer: gozip.NewWriter(w), seen: make(map[string]bool, 0)}
}

// Exists returns true if the given file exists in the archive
func (w *Writer) Exists(name string) bool {
	return w.seen[name]
}

// Create creates a new entry and returns a writer
func (w *Writer) Create(name string) (io.Writer, error) {
	if w.seen[name] {
		return nil, errors.New("file exists")
	}
	w.seen[name] = true
	return w.writer.Create(name)
}

// CreateHeader creates a new header entry and returns a writer
func (w *Writer) CreateHeader(fh *gozip.FileHeader) (io.Writer, error) {
	if w.seen[fh.Name] {
		return nil, errors.New("file exists")
	}
	w.seen[fh.Name] = true
	return w.writer.CreateHeader(fh)
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
