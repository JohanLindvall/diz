package hashzip

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"testing"
)

func hashOf(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}

// buildZip writes a two-entry archive and returns its bytes.
func buildZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := NewWriter(&buf)
	a, err := w.Create("dir/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	a.Write([]byte("alpha"))
	b, err := w.Create("b.txt")
	if err != nil {
		t.Fatal(err)
	}
	b.Write([]byte("bravo"))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestWriterExists(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	a, _ := w.Create("a")
	a.Write([]byte("x"))
	b, _ := w.Create("b") // creating b ends a, recording its hash
	if !w.Exists("a") {
		t.Fatal("Exists(a) = false after it was ended")
	}
	if w.Exists("b") {
		t.Fatal("Exists(b) = true before it was ended")
	}
	b.Write([]byte("y"))
	w.Close()
}

func TestWriterDuplicateName(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	c, _ := w.Create("dup")
	c.Write([]byte("1"))
	if _, err := w.Create("dup"); err == nil {
		t.Fatal("expected error creating duplicate name")
	}
}

func TestReaderLookups(t *testing.T) {
	data := buildZip(t)
	r, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}

	f := r.GetFile("dir/a.txt")
	if f == nil {
		t.Fatal("GetFile(dir/a.txt) = nil")
	}
	if r.GetFile("missing") != nil {
		t.Fatal("GetFile(missing) != nil")
	}

	rc, err := f.Open()
	if err != nil {
		t.Fatal(err)
	}
	content, _ := io.ReadAll(rc)
	rc.Close()
	if string(content) != "alpha" {
		t.Fatalf("content = %q", content)
	}

	wantHash := hashOf("alpha")
	if r.GetHash("dir/a.txt") != wantHash {
		t.Fatalf("GetHash = %q, want %q", r.GetHash("dir/a.txt"), wantHash)
	}
	if r.GetHash("missing") != "" {
		t.Fatal("GetHash(missing) != empty")
	}

	byHash := r.GetFileByHash(wantHash)
	if byHash == nil || byHash.Name != "dir/a.txt" {
		t.Fatalf("GetFileByHash = %+v", byHash)
	}
	if r.GetFileByHash("nope") != nil {
		t.Fatal("GetFileByHash(nope) != nil")
	}
}

func TestWriterCopyRenames(t *testing.T) {
	data := buildZip(t)
	r, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	src := r.GetFile("dir/a.txt")
	if src == nil {
		t.Fatal("source entry not found")
	}

	var out bytes.Buffer
	w := NewWriter(&out)
	if err := w.Copy("renamed.txt", src); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r2, err := NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatal(err)
	}
	f := r2.GetFile("renamed.txt")
	if f == nil {
		t.Fatal("renamed.txt not found after Copy")
	}
	rc, _ := f.Open()
	content, _ := io.ReadAll(rc)
	rc.Close()
	if string(content) != "alpha" {
		t.Fatalf("copied content = %q", content)
	}
	// the content hash must survive the raw copy
	if r2.GetHash("renamed.txt") != hashOf("alpha") {
		t.Fatalf("copied hash = %q", r2.GetHash("renamed.txt"))
	}
}
