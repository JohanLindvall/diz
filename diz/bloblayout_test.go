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

func TestBlobLayoutRoundTrip(t *testing.T) {
	config := []byte(`{"architecture":"amd64","os":"linux"}`)
	layer := []byte("a layer stored using the current Docker save layout")
	configHash := fmt.Sprintf("%x", sha256.Sum256(config))
	layerHash := fmt.Sprintf("%x", sha256.Sum256(layer))
	configPath, layerPath := "blobs/sha256/"+configHash, "blobs/sha256/"+layerHash
	manifests := []Manifest{{Config: configPath, RepoTags: []string{"example/image:latest"}, Layers: []string{layerPath}}}

	var source bytes.Buffer
	zw := hashzip.NewWriter(&source)
	for name, data := range map[string][]byte{
		configPath:                config,
		layerPath:                 layer,
		"blobs/sha256/unselected": []byte("another image"),
	} {
		entry, err := zw.Create(dizPrefix + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := WriteManifests(manifests, zw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	archive, err := NewArchive(bytes.NewReader(source.Bytes()), int64(source.Len()))
	if err != nil {
		t.Fatal(err)
	}
	rm, err := archive.RegistryManifest("example/image:latest")
	if err != nil {
		t.Fatal(err)
	}
	if rm.Config.Digest != "sha256:"+configHash || rm.Layers[0].Digest != "sha256:"+layerHash {
		t.Errorf("registry digest depends on the blob path: %+v", rm)
	}

	var copied bytes.Buffer
	zw = hashzip.NewWriter(&copied)
	if err := archive.CopyToZip(zw, manifests); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifests(manifests, zw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	copy, err := NewArchive(bytes.NewReader(copied.Bytes()), int64(copied.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for name, a := range map[string]*Archive{"original": archive, "copied": copy} {
		t.Run(name, func(t *testing.T) {
			var restored bytes.Buffer
			if err := a.CopyToTar(&restored, manifests); err != nil {
				t.Fatal(err)
			}
			entries := map[string][]byte{}
			tr := tar.NewReader(&restored)
			for {
				header, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				entries[header.Name], err = io.ReadAll(tr)
				if err != nil {
					t.Fatal(err)
				}
			}
			if !bytes.Equal(entries[configPath], config) || !bytes.Equal(entries[layerPath], layer) {
				t.Fatalf("restore omitted image blobs: %v", keys(entries))
			}
			if _, exists := entries["blobs/sha256/unselected"]; exists {
				t.Fatal("restore included an unselected blob")
			}
		})
	}
}
