// Package registry serves a diz zip archive over a subset of the Docker
// registry v2 HTTP API, plus a /get endpoint for extracting raw archive paths.
package registry

import (
	"log"
	"net/http"
	"regexp"

	"github.com/JohanLindvall/diz/imagesource"
)

// handlerFunc handles a matched request. match holds the regexp submatches of
// the request path. Returning a non-nil error yields a 404 response.
type handlerFunc func(w http.ResponseWriter, r *http.Request, match []string) error

// route pairs a path pattern with its handler.
type route struct {
	pattern *regexp.Regexp
	handle  handlerFunc
}

// Server serves a zip image source as a Docker registry.
type Server struct {
	source       *imagesource.ZipImageSource
	digestedTags map[string][]string
	routes       []route
}

// ListenAndServe serves the given source on addr until an error occurs.
func ListenAndServe(addr string, source *imagesource.ZipImageSource) error {
	s, err := NewServer(source)
	if err != nil {
		return err
	}
	return http.ListenAndServe(addr, s)
}

// NewServer builds a Server for the given source, precomputing the mapping from
// manifest digest to repo tags.
func NewServer(source *imagesource.ZipImageSource) (*Server, error) {
	digestedTags, err := source.DigestToTags()
	if err != nil {
		return nil, err
	}

	s := &Server{source: source, digestedTags: digestedTags}
	s.routes = []route{
		{regexp.MustCompile(`^/v2/$`), s.handleBase},
		{regexp.MustCompile(`^/v2/manifests$`), s.handleManifestList},
		{regexp.MustCompile(`^/v2/[^/]+/(?:manifests|blobs)/sha256:([0-9a-f]{64})$`), s.handleBlob},
		{regexp.MustCompile(`^/v2/([^/]+)/manifests/([^/]+)$`), s.handleManifestByTag},
		{regexp.MustCompile(`^/get/(.*)$`), s.handleGet},
	}
	return s, nil
}

// ServeHTTP dispatches the request to the first matching route.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL)

	for _, rt := range s.routes {
		if match := rt.pattern.FindStringSubmatch(r.URL.Path); match != nil {
			if err := rt.handle(w, r, match); err != nil {
				w.WriteHeader(http.StatusNotFound)
			}
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}
