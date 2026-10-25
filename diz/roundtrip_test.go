package diz

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"testing"

	"github.com/JohanLindvall/diz/hashzip"
)

// buildDockerSaveTar builds an in-memory tar resembling `docker save` output.
func buildDockerSaveTar(t *testing.T, layer []byte) []byte {
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
	write("manifest.json", []byte(`[{"Config":"config.json","RepoTags":["repo/name:tag"],"Layers":["layerid/layer.tar"]}]`))
	write("repositories", []byte(`{"repo/name":{"tag":"layerid"}}`))
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRoundTrip(t *testing.T) {
	layer := []byte("this is a fake layer")
	layerHash := fmt.Sprintf("%x", sha256.Sum256(layer))

	// docker save tar -> diz zip
	var zipBuf bytes.Buffer
	zw := hashzip.NewWriter(&zipBuf)
	manifests, err := CopyFromTar(bytes.NewReader(buildDockerSaveTar(t, layer)), zw)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteManifests(manifests, zw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	// read the archive back
	a, err := NewArchive(bytes.NewReader(zipBuf.Bytes()), int64(zipBuf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Manifests) != 1 || len(a.Manifests[0].RepoTags) != 1 || a.Manifests[0].RepoTags[0] != "repo/name:tag" {
		t.Fatalf("unexpected manifests: %+v", a.Manifests)
	}

	// registry manifest resolves the layer digest via the stored hash
	rm, err := a.RegistryManifest("repo/name:tag")
	if err != nil {
		t.Fatal(err)
	}
	if rm.Config.Digest != fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(`{"config":true}`))) {
		t.Fatalf("config digest = %q", rm.Config.Digest)
	}
	if len(rm.Layers) != 1 || rm.Layers[0].Digest != "sha256:"+layerHash {
		t.Fatalf("layer digest = %+v want sha256:%s", rm.Layers, layerHash)
	}

	// WriteBlob returns the raw layer content by hash
	var blob bytes.Buffer
	if err := a.WriteBlob(&blob, layerHash); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(blob.Bytes(), layer) {
		t.Fatalf("blob = %q", blob.Bytes())
	}

	// CopyToTar reproduces the manifest and the layer content
	var tarBuf bytes.Buffer
	if err := a.CopyToTar(&tarBuf, a.Manifests); err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(&tarBuf)
	seen := map[string][]byte{}
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		b, _ := io.ReadAll(tr)
		seen[h.Name] = b
	}
	if _, ok := seen["manifest.json"]; !ok {
		t.Fatalf("manifest.json missing from tar, saw %v", keys(seen))
	}
	if !bytes.Equal(seen["layerid/layer.tar"], layer) {
		t.Fatalf("tar layer = %q", seen["layerid/layer.tar"])
	}
}

func keys(m map[string][]byte) (r []string) {
	for k := range m {
		r = append(r, k)
	}
	return
}
