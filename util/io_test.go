package util

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// trackReadCloser records whether Close was called.
type trackReadCloser struct {
	io.Reader
	closed bool
}

func (t *trackReadCloser) Close() error {
	t.closed = true
	return nil
}

func TestCopyAndClose(t *testing.T) {
	src := &trackReadCloser{Reader: strings.NewReader("hello world")}
	var dst bytes.Buffer

	n, err := CopyAndClose(&dst, src)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len("hello world")) {
		t.Fatalf("copied %d bytes, want %d", n, len("hello world"))
	}
	if dst.String() != "hello world" {
		t.Fatalf("dst = %q", dst.String())
	}
	if !src.closed {
		t.Fatal("source was not closed")
	}
}

func TestReadWriteLinesRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lines")
	lines := []string{"first", "second", "third"}

	if err := WriteLines(path, lines); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLines(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "|") != strings.Join(lines, "|") {
		t.Fatalf("round trip = %v, want %v", got, lines)
	}
}

func TestReadLinesNormalizesCRLF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crlf")
	if err := os.WriteFile(path, []byte("a\r\nb\r\nc"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLines(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b", "c"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestReadLinesMissingFile(t *testing.T) {
	if _, err := ReadLines(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
