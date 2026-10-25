package registry

import (
	"encoding/json"
	"net/http"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/util"
)

const octetStream = "application/octet-stream"

// handleBase answers the registry version check at /v2/.
func (s *Server) handleBase(w http.ResponseWriter, r *http.Request, match []string) error {
	w.WriteHeader(http.StatusOK)
	return nil
}

// handleManifestList returns the raw diz manifests as JSON.
func (s *Server) handleManifestList(w http.ResponseWriter, r *http.Request, match []string) error {
	js, err := json.MarshalIndent(s.source.Manifests(), "", "  ")
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	return err
}

// handleBlob serves a blob by digest. If the digest names a manifest, the
// registry manifest is returned; otherwise the raw layer/config blob is written.
func (s *Server) handleBlob(w http.ResponseWriter, r *http.Request, match []string) error {
	hash := match[1]
	if tags, ok := s.digestedTags[hash]; ok && len(tags) > 0 {
		return s.writeRegistryManifest(w, tags[0])
	}
	w.Header().Set("Content-Type", octetStream)
	return s.source.WriteBlob(w, hash)
}

// handleManifestByTag returns the registry manifest for a repo:tag reference.
func (s *Server) handleManifestByTag(w http.ResponseWriter, r *http.Request, match []string) error {
	return s.writeRegistryManifest(w, match[1]+":"+match[2])
}

// handleGet streams a raw path out of the archive; directories are zipped.
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request, match []string) error {
	rdr, err := s.source.Read(match[1])
	if err != nil {
		return err
	}
	if rdr == nil {
		return diz.ErrNotFound
	}
	w.Header().Set("Content-Type", octetStream)
	_, err = util.CopyAndClose(w, rdr)
	return err
}

// writeRegistryManifest renders and writes the registry manifest for repoTag.
func (s *Server) writeRegistryManifest(w http.ResponseWriter, repoTag string) error {
	m, err := s.source.RegistryManifest(repoTag)
	if err != nil {
		return err
	}
	js, digest, err := diz.MarshalRegistryManifest(m)
	if err != nil {
		return err
	}
	w.Header().Set("Docker-Content-Digest", "sha256:"+digest)
	w.Header().Set("Content-Type", m.MediaType)
	_, err = w.Write(js)
	return err
}
