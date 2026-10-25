package imagesource

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/hashzip"
)

const testTag = "lib/app:1"

// dockerSaveTar builds an in-memory `docker save`-style tar for one image.
func dockerSaveTar(t *testing.T, layer []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	write := func(name string, data []byte) {
		if err := tw.WriteHeader(&tar.Header{Name: name, Size: int64(len(data)), Mode: 0644, Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	write("config.json", []byte(`{"config":true}`))
	write("layerid/layer.tar", layer)
	write("manifest.json", []byte(`[{"Config":"config.json","RepoTags":["`+testTag+`"],"Layers":["layerid/layer.tar"]}]`))
	write("repositories", []byte(`{"lib/app":{"1":"layerid"}}`))
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// buildDizFile writes a diz archive to a temp file, returning its path, the raw
// layer bytes and the layer's content hash.
func buildDizFile(t *testing.T) (path string, layer []byte, layerHash string) {
	t.Helper()
	layer = []byte("imagesource test layer payload")
	layerHash = fmt.Sprintf("%x", sha256.Sum256(layer))

	var zbuf bytes.Buffer
	zw := hashzip.NewWriter(&zbuf)
	manifests, err := diz.CopyFromTar(bytes.NewReader(dockerSaveTar(t, layer)), zw)
	if err != nil {
		t.Fatal(err)
	}
	if err := diz.WriteManifests(manifests, zw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	path = filepath.Join(t.TempDir(), "src.diz")
	if err := os.WriteFile(path, zbuf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	return
}

func openZipSource(t *testing.T) (*ZipImageSource, []byte, string) {
	t.Helper()
	path, layer, layerHash := buildDizFile(t)
	src, err := NewZipImageSource(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { src.Close() })
	return src, layer, layerHash
}

func TestZipGlobTags(t *testing.T) {
	src, _, _ := openZipSource(t)

	all, err := src.GlobTags([]string{"*"})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0] != testTag {
		t.Fatalf("GlobTags(*) = %v", all)
	}

	none, err := src.GlobTags([]string{"nomatch:*"})
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("GlobTags(nomatch) = %v", none)
	}
}

func TestZipManifestsAndRegistryManifest(t *testing.T) {
	src, _, layerHash := openZipSource(t)

	if m := src.Manifests(); len(m) != 1 {
		t.Fatalf("Manifests len = %d", len(m))
	}

	rm, err := src.RegistryManifest(testTag)
	if err != nil {
		t.Fatal(err)
	}
	if rm.Config.Digest != fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(`{"config":true}`))) {
		t.Fatalf("config digest = %q", rm.Config.Digest)
	}
	if len(rm.Layers) != 1 || rm.Layers[0].Digest != "sha256:"+layerHash {
		t.Fatalf("layers = %+v", rm.Layers)
	}
}

func TestZipDigestMaps(t *testing.T) {
	src, _, _ := openZipSource(t)

	rm, _ := src.RegistryManifest(testTag)
	_, digest, err := diz.MarshalRegistryManifest(rm)
	if err != nil {
		t.Fatal(err)
	}

	d2t, err := src.DigestToTags()
	if err != nil {
		t.Fatal(err)
	}
	if got := d2t[digest]; len(got) != 1 || got[0] != testTag {
		t.Fatalf("DigestToTags[%s] = %v", digest, got)
	}

	t2d, err := src.NormalizedTagsToDigest()
	if err != nil {
		t.Fatal(err)
	}
	if t2d["docker.io/lib/app:1"] != digest {
		t.Fatalf("NormalizedTagsToDigest = %v", t2d)
	}
}

func TestZipWriteBlob(t *testing.T) {
	src, layer, layerHash := openZipSource(t)
	var buf bytes.Buffer
	if err := src.WriteBlob(&buf, layerHash); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), layer) {
		t.Fatalf("blob = %q", buf.Bytes())
	}
}

func TestZipRead(t *testing.T) {
	src, _, _ := openZipSource(t)

	rdr, err := src.Read(".diz/config.json")
	if err != nil {
		t.Fatal(err)
	}
	if rdr == nil {
		t.Fatal("Read returned nil reader for existing file")
	}
	b, _ := io.ReadAll(rdr)
	rdr.Close()
	if string(b) != `{"config":true}` {
		t.Fatalf("config = %q", b)
	}

	missing, err := src.Read("does/not/exist")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Fatal("expected nil reader for missing path")
	}
}

func TestZipReadTar(t *testing.T) {
	src, layer, _ := openZipSource(t)

	rdr, err := src.ReadTar([]string{"*"})
	if err != nil {
		t.Fatal(err)
	}
	defer rdr.Close()

	tr := tar.NewReader(rdr)
	found := map[string][]byte{}
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		b, _ := io.ReadAll(tr)
		found[h.Name] = b
	}
	if _, ok := found["manifest.json"]; !ok {
		t.Fatalf("manifest.json missing from tar")
	}
	if !bytes.Equal(found["layerid/layer.tar"], layer) {
		t.Fatalf("layer mismatch in tar")
	}
}

func TestZipCopyToZip(t *testing.T) {
	src, _, _ := openZipSource(t)

	var out bytes.Buffer
	zw := hashzip.NewWriter(&out)
	manifests, err := src.CopyToZip(zw, []string{"*"})
	if err != nil {
		t.Fatal(err)
	}
	if err := diz.WriteManifests(manifests, zw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	// The copied archive must be readable and contain the same tag.
	a, err := diz.NewArchive(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Manifests) != 1 || a.Manifests[0].RepoTags[0] != testTag {
		t.Fatalf("copied manifests = %+v", a.Manifests)
	}
}

func TestNullImageSource(t *testing.T) {
	s := NewNullImageSource()

	tags, err := s.GlobTags([]string{"*"})
	if err != nil || tags != nil {
		t.Fatalf("GlobTags = %v, %v", tags, err)
	}
	m, err := s.CopyToZip(nil, nil)
	if err != nil || m != nil {
		t.Fatalf("CopyToZip = %v, %v", m, err)
	}
	rdr, err := s.ReadTar(nil)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := rdr.Read(make([]byte, 4)); n != 0 || err != io.EOF {
		t.Fatalf("ReadTar read = %d, %v; want 0, EOF", n, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMarshalCredentials(t *testing.T) {
	enc := marshalCredentials("user", "pass")
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["username"] != "user" || got["password"] != "pass" {
		t.Fatalf("credentials = %v", got)
	}
}
