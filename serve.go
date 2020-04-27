package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/imagesource"
)

func serve(address string) (err error) {
	is, err := imagesource.NewZipImageSource(*fromZip)
	if err != nil {
		return err
	}

	return http.ListenAndServe(":5000", &server{is: is})
}

type server struct {
	is *imagesource.ZipImageSource
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.String())
	if r.URL.Path == "/v2/" {
		w.WriteHeader(http.StatusOK)
		return
	} else if match := manifestRe.FindStringSubmatch(r.URL.Path); match != nil {
		repoTag := match[1] + ":" + match[2]
		repoManifest, err := s.is.GetRegistryManifest(repoTag)
		if err == nil {
			if err = sendDigestResponse(w, repoManifest); err == nil {
				return
			}
		}
	} else if match := blobRe.FindStringSubmatch(r.URL.Path); match != nil {
		sum := match[1]
		w.Header().Add("Content-Type", "application/octet-stream")
		if err := s.is.WriteFileByHash(w, sum); err == nil {
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func sendDigestResponse(w http.ResponseWriter, m diz.RegistryManifest) (err error) {
	var js []byte
	if js, err = json.Marshal(m); err == nil {
		w.Header().Set("Docker-Content-Digest", fmt.Sprintf("sha256:%x", sha256.Sum256(js)))
		w.Header().Set("Content-Type", m.MediaType)
		w.Write(js)
	}

	return
}

var (
	manifestRe = regexp.MustCompile("^/v2/([^/]+)/manifests/([^/]+)$")
	blobRe     = regexp.MustCompile("^/v2/[^/]+/blobs/sha256:([0-9a-f]{64})$")
)
