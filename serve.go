package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/imagesource"
	"github.com/JohanLindvall/diz/util"
)

func serve(zip string) error {
	is, err := imagesource.NewZipImageSource(zip)
	if err != nil {
		return err
	}

	digestedTags, err := is.GetDigestToTags()
	if err != nil {
		return err
	}

	return http.ListenAndServe(":5000", &server{is: is, digestedTags: digestedTags})
}

type server struct {
	is           *imagesource.ZipImageSource
	digestedTags map[string][]string
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.String())
	if r.URL.Path == "/v2/" {
		w.WriteHeader(http.StatusOK)
		return
	} else if r.URL.Path == "/v2/manifests" {
		if js, err := json.MarshalIndent(s.is.Manifests(), "", "  "); err == nil {
			w.Write(js)
			return
		}
	} else if match := blobRe.FindStringSubmatch(r.URL.Path); match != nil {
		sum := match[2]
		if repoTag, ok := s.digestedTags[sum]; ok && len(repoTag) > 0 {
			if err := s.returnRegistryManifest(w, repoTag[0]); err == nil {
				return
			}
		} else {
			w.Header().Add("Content-Type", "application/octet-stream")
			if err := s.is.WriteFileByHash(w, sum); err == nil {
				return
			}
		}
	} else if match := manifestRe.FindStringSubmatch(r.URL.Path); match != nil {
		repoTag := match[1] + ":" + match[2]
		if err := s.returnRegistryManifest(w, repoTag); err == nil {
			return
		}
	} else if match := getRe.FindStringSubmatch(r.URL.Path); match != nil {
		path := match[1]
		if rdr, err := s.is.Read(path); err == nil && rdr != nil {
			w.Header().Add("Content-Type", "application/octet-stream")
			_, err = util.CopyAndClose(w, rdr)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func (s *server) returnRegistryManifest(w http.ResponseWriter, repoTag string) (err error) {
	var repoManifest diz.RegistryManifest
	if repoManifest, err = s.is.GetRegistryManifest(repoTag); err == nil {
		err = sendDigestResponse(w, repoManifest)
	}
	return
}

func sendDigestResponse(w http.ResponseWriter, m diz.RegistryManifest) error {
	if js, digest, err := diz.GetManifestBytes(m); err == nil {
		w.Header().Set("Docker-Content-Digest", fmt.Sprintf("sha256:%x", digest))
		w.Header().Set("Content-Type", m.MediaType)
		_, err := w.Write(js)
		return err
	} else {
		return err
	}
}

var (
	manifestRe = regexp.MustCompile("^/v2/([^/]+)/manifests/([^/]+)$")
	blobRe     = regexp.MustCompile("^/v2/[^/]+/(manifests|blobs)/sha256:([0-9a-f]{64})$")
	getRe      = regexp.MustCompile("^/get/(.*)$")
)
