package registry

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/hashzip"
	"github.com/JohanLindvall/diz/imagesource"
)

const (
	testRepoTag = "myimage:v1"
	testConfig  = `{"config":true}`
)

// buildDockerSaveTar builds an in-memory tar resembling `docker save` output
// for a single image with one layer.
func buildDockerSaveTar(t *testing.T, layer []byte, repoTag string) []byte {
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
	write("config.json", []byte(testConfig))
	write("layerid/layer.tar", layer)
	write("manifest.json", []byte(`[{"Config":"config.json","RepoTags":["`+repoTag+`"],"Layers":["layerid/layer.tar"]}]`))
	write("repositories", []byte(`{"myimage":{"v1":"layerid"}}`))
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// newTestServer writes a diz archive to a temp file and returns a Server for it
// along with the raw layer bytes it contains.
func newTestServer(t *testing.T) (*Server, []byte) {
	t.Helper()
	return newTestServerWithTag(t, testRepoTag)
}

func newTestServerWithTag(t *testing.T, repoTag string) (*Server, []byte) {
	t.Helper()
	layer := []byte("fake layer contents for the registry integration test")

	var zipBuf bytes.Buffer
	zw := hashzip.NewWriter(&zipBuf)
	manifests, err := diz.CopyFromTar(bytes.NewReader(buildDockerSaveTar(t, layer, repoTag)), zw)
	if err != nil {
		t.Fatal(err)
	}
	if err := diz.WriteManifests(manifests, zw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "test.diz")
	if err := os.WriteFile(path, zipBuf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	src, err := imagesource.NewZipImageSource(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { src.Close() })

	s, err := NewServer(src)
	if err != nil {
		t.Fatal(err)
	}
	return s, layer
}

// get issues a GET request against the server and returns the recorder.
func get(s *Server, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestBaseRoute(t *testing.T) {
	s, _ := newTestServer(t)
	if rec := get(s, "/v2/"); rec.Code != http.StatusOK {
		t.Fatalf("/v2/ = %d, want 200", rec.Code)
	}
}

func TestManifestList(t *testing.T) {
	s, _ := newTestServer(t)
	rec := get(s, "/v2/manifests")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	var manifests []diz.Manifest
	if err := json.Unmarshal(rec.Body.Bytes(), &manifests); err != nil {
		t.Fatalf("body is not manifest JSON: %v", err)
	}
	if len(manifests) != 1 || len(manifests[0].RepoTags) != 1 || manifests[0].RepoTags[0] != testRepoTag {
		t.Fatalf("unexpected manifests: %+v", manifests)
	}
}

func TestManifestByTagDigestHeader(t *testing.T) {
	s, _ := newTestServer(t)

	// Expected digest computed directly from the source manifest.
	rm, err := s.source.RegistryManifest(testRepoTag)
	if err != nil {
		t.Fatal(err)
	}
	wantBody, wantDigest, err := diz.MarshalRegistryManifest(rm)
	if err != nil {
		t.Fatal(err)
	}

	rec := get(s, "/v2/myimage/manifests/v1")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	// The header must be sha256: + the 64-hex digest, not a double-encoding.
	if got := rec.Header().Get("Docker-Content-Digest"); got != "sha256:"+wantDigest {
		t.Fatalf("Docker-Content-Digest = %q, want %q", got, "sha256:"+wantDigest)
	}
	if !bytes.Equal(rec.Body.Bytes(), wantBody) {
		t.Fatalf("manifest body mismatch")
	}
}

func TestBlobByLayerHash(t *testing.T) {
	s, layer := newTestServer(t)
	layerHash := fmt.Sprintf("%x", sha256.Sum256(layer))

	rec := get(s, "/v2/myimage/blobs/sha256:"+layerHash)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if !bytes.Equal(rec.Body.Bytes(), layer) {
		t.Fatalf("blob body = %q, want %q", rec.Body.Bytes(), layer)
	}
}

func TestBlobByManifestDigestReturnsManifest(t *testing.T) {
	s, _ := newTestServer(t)
	rm, _ := s.source.RegistryManifest(testRepoTag)
	_, digest, _ := diz.MarshalRegistryManifest(rm)

	// A digest that names a manifest resolves to the registry manifest.
	rec := get(s, "/v2/myimage/manifests/sha256:"+digest)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if rec.Header().Get("Docker-Content-Digest") != "sha256:"+digest {
		t.Fatalf("digest header = %q", rec.Header().Get("Docker-Content-Digest"))
	}
}

func TestGetRawPath(t *testing.T) {
	s, _ := newTestServer(t)
	rec := get(s, "/get/.diz/config.json")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if rec.Body.String() != testConfig {
		t.Fatalf("body = %q, want %q", rec.Body.String(), testConfig)
	}
}

func TestUnknownRouteIs404(t *testing.T) {
	s, _ := newTestServer(t)
	if rec := get(s, "/nope"); rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rec.Code)
	}
}

func TestNamespacedRepository(t *testing.T) {
	const repo = "team/project/myimage"
	s, layer := newTestServerWithTag(t, repo+":v1")
	rm, err := s.source.RegistryManifest(repo + ":v1")
	if err != nil {
		t.Fatal(err)
	}
	body, digest, err := diz.MarshalRegistryManifest(rm)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/v2/" + repo + "/manifests/v1",
		"/v2/" + repo + "/manifests/sha256:" + digest,
	} {
		t.Run(path, func(t *testing.T) {
			rec := get(s, path)
			if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), body) {
				t.Fatalf("manifest response = %d %q", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Docker-Content-Digest"); got != "sha256:"+digest {
				t.Fatalf("digest header = %q", got)
			}
		})
	}
	layerHash := fmt.Sprintf("%x", sha256.Sum256(layer))
	rec := get(s, "/v2/"+repo+"/blobs/sha256:"+layerHash)
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), layer) {
		t.Fatalf("blob response = %d %q", rec.Code, rec.Body.String())
	}
}

func TestMissingContentIs404(t *testing.T) {
	s, _ := newTestServer(t)
	for _, path := range []string{
		"/v2/myimage/manifests/missing",
		"/v2/unknown/manifests/v1",
		"/v2/myimage/manifests/sha256:" + fmt.Sprintf("%064x", 0),
		"/v2/myimage/blobs/sha256:" + fmt.Sprintf("%064x", 0),
		"/get/missing",
	} {
		t.Run(path, func(t *testing.T) {
			rec := get(s, path)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("response = %d %q, want 404", rec.Code, rec.Body.String())
			}
			if digest := rec.Header().Get("Docker-Content-Digest"); digest != "" {
				t.Fatalf("missing content has digest %q", digest)
			}
		})
	}
}

func TestArchiveReadErrorIs500(t *testing.T) {
	s, _ := newTestServer(t)
	if err := s.source.Close(); err != nil {
		t.Fatal(err)
	}
	if rec := get(s, "/get/.diz/config.json"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("unreadable archive = %d, want 500", rec.Code)
	}
}
