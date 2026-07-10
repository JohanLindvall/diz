package app

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/hashzip"
	"github.com/JohanLindvall/diz/imagesource"
)

const appTestTag = "lib/app:1"

// writeDizFixture writes a diz archive containing one image to a temp file.
func writeDizFixture(t *testing.T) string {
	t.Helper()
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	write := func(name string, data []byte) {
		if err := tw.WriteHeader(&tar.Header{Name: name, Size: int64(len(data)), Mode: 0644, Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	write("config.json", []byte(`{"config":true}`))
	write("layerid/layer.tar", []byte("app package test layer"))
	write("manifest.json", []byte(`[{"Config":"config.json","RepoTags":["`+appTestTag+`"],"Layers":["layerid/layer.tar"]}]`))
	write("repositories", []byte(`{"lib/app":{"1":"layerid"}}`))
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	var zbuf bytes.Buffer
	zw := hashzip.NewWriter(&zbuf)
	manifests, err := diz.CopyFromTar(&tarBuf, zw)
	if err != nil {
		t.Fatal(err)
	}
	if err := diz.WriteManifests(manifests, zw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "fixture.diz")
	if err := os.WriteFile(path, zbuf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// captureStdout runs fn and returns whatever it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	w.Close()
	os.Stdout = orig
	return <-done
}

func TestTags(t *testing.T) {
	// No tag file: input is returned unchanged.
	a := New(nil, Options{})
	if got := a.Tags([]string{"a:1"}); len(got) != 1 || got[0] != "a:1" {
		t.Fatalf("Tags without file = %v", got)
	}

	// With a tag file, its references are appended.
	tagPath := filepath.Join(t.TempDir(), "tags")
	if err := os.WriteFile(tagPath, []byte("redis:5\n# comment\nnginx:latest\n"), 0644); err != nil {
		t.Fatal(err)
	}
	a = New(nil, Options{TagFile: tagPath})
	got := a.Tags([]string{"a:1"})
	if strings.Join(got, ",") != "a:1,redis:5,nginx:latest" {
		t.Fatalf("Tags with file = %v", got)
	}
}

func TestList(t *testing.T) {
	a := New(nil, Options{FromZip: writeDizFixture(t)})
	out := captureStdout(t, func() {
		if err := a.List([]string{"*"}); err != nil {
			t.Errorf("List: %v", err)
		}
	})
	if strings.TrimSpace(out) != appTestTag {
		t.Fatalf("List output = %q, want %q", out, appTestTag)
	}
}

func TestCreate(t *testing.T) {
	a := New(nil, Options{FromZip: writeDizFixture(t)})
	outPath := filepath.Join(t.TempDir(), "out.diz")

	if err := a.Create(outPath, []string{"*"}); err != nil {
		t.Fatal(err)
	}

	// The created archive must be a valid diz zip carrying the same tag.
	src, err := imagesource.NewZipImageSource(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	tags, err := src.GlobTags([]string{"*"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != appTestTag {
		t.Fatalf("created archive tags = %v", tags)
	}
}

func TestUpdateTagFileDigest(t *testing.T) {
	zipPath := writeDizFixture(t)
	tagPath := filepath.Join(t.TempDir(), "tags")
	if err := os.WriteFile(tagPath, []byte(appTestTag+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	a := New(nil, Options{TagFile: tagPath, DigestTags: true})
	if err := a.updateTagFile(zipPath); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(tagPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "lib/app@sha256:") {
		t.Fatalf("tag file not rewritten to digest: %q", b)
	}
}

func TestUpdateTagFileRegistry(t *testing.T) {
	zipPath := writeDizFixture(t)
	tagPath := filepath.Join(t.TempDir(), "tags")
	if err := os.WriteFile(tagPath, []byte(appTestTag+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	a := New(nil, Options{TagFile: tagPath, RegistryAddress: "myreg:5000"})
	if err := a.updateTagFile(zipPath); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(tagPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(b)) != "myreg:5000/lib/app:1" {
		t.Fatalf("tag file = %q, want myreg:5000/lib/app:1", b)
	}
}

func TestUpdateTagFileNoop(t *testing.T) {
	// Neither DigestTags nor RegistryAddress set: nothing to do, no error.
	a := New(nil, Options{})
	if err := a.updateTagFile(writeDizFixture(t)); err != nil {
		t.Fatal(err)
	}
}
