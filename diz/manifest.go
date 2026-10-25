package diz

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/JohanLindvall/diz/dockerref"

	"github.com/ryanuber/go-glob"
)

const (
	dizPrefix         = ".diz/"
	manifestJSON      = "manifest.json"
	repos             = "repositories"
	layersTarSuffix   = "/layer.tar"
	latestTag         = "latest"
	dotJSON           = ".json"
	manifestMediaType = "application/vnd.docker.distribution.manifest.v2+json"
	configMediaType   = "application/vnd.docker.container.image.v1+json"
	layerMediaType    = "application/vnd.docker.image.rootfs.diff.tar"
)

// Manifest defines a Docker image manifest.
type Manifest struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

// repositories maps a repository name to a map of tag to layer id, matching the
// format of the legacy "repositories" file in a Docker image tar.
type repositories map[string]map[string]string

// Merge combines manifests m1 and m2 into a single slice, coalescing manifests
// that share a config and unioning their repo tags.
func Merge(m1, m2 []Manifest) (result []Manifest) {
	for _, m := range append(append([]Manifest(nil), m1...), m2...) {
		handled := false
		for i, e := range result {
			if e.Config == m.Config {
				// add missing repo tags
				for _, rt := range m.RepoTags {
					if !slices.Contains(result[i].RepoTags, rt) {
						result[i].RepoTags = append(result[i].RepoTags, rt)
					}
				}
				handled = true
			}
		}

		if !handled {
			result = append(result, Manifest{Config: m.Config, Layers: append([]string(nil), m.Layers...), RepoTags: append([]string(nil), m.RepoTags...)})
		}
	}

	return
}

// Filter returns a copy of the manifests keeping only the repo tags that match
// the tags glob. Manifests with no matching tags are dropped.
func Filter(manifests []Manifest, tags []string) (result []Manifest) {
	for _, m := range manifests {
		mm := Manifest{Config: m.Config, Layers: m.Layers, RepoTags: FilterTags(m.RepoTags, []string{}, tags)}
		if len(mm.RepoTags) > 0 {
			result = append(result, mm)
		}
	}

	return
}

// FilterTags filters the given image tags according to the glob expressions. A
// '-' prepending the glob expression means a negative match.
func FilterTags(repoTags, repoDigests, globTags []string) (result []string) {
	for _, repoTag := range repoTags {
		if repoTag != "<none>:<none>" {
			first := true
			match := false
			for _, globTag := range globTags {
				neg := false
				if strings.HasPrefix(globTag, "-") {
					globTag = globTag[1:]
					neg = true
					if first {
						match = true
					}
				}

				if glob.Glob(globTag, repoTag) {
					match = !neg
				}

				first = false
			}

			if match {
				result = append(result, repoTag)
			}
		}
	}

	for _, repoDigest := range repoDigests {
		for _, globTag := range globTags {
			if dockerref.CompareReferences(repoDigest, globTag) {
				result = append(result, repoDigest)
				break
			}
		}
	}

	return
}

// buildRepositoryFiles renders the manifest.json and repositories files that
// describe the given manifests, keyed by file name.
func buildRepositoryFiles(manifests []Manifest) (result map[string][]byte, err error) {
	result = make(map[string][]byte)
	var b []byte

	if b, err = json.Marshal(&manifests); err != nil {
		return
	}
	result[manifestJSON] = b

	r := make(repositories)
	for _, m := range manifests {
		for _, t := range m.RepoTags {
			s := strings.SplitN(t, ":", 2)
			if len(s) == 1 {
				s = append(s, latestTag)
			}
			repo, ok := r[s[0]]
			if !ok {
				repo = make(map[string]string)
				r[s[0]] = repo
			}
			repo[s[1]] = strings.TrimSuffix(m.Layers[len(m.Layers)-1], layersTarSuffix)
		}
	}
	if b, err = json.Marshal(&r); err != nil {
		return
	}
	result[repos] = b
	return
}
