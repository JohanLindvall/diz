package diz

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/JohanLindvall/diz/dockerref"
)

// RegistryManifest defines the Docker image manifest from the registry's point
// of view.
type RegistryManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Config        struct {
		MediaType string `json:"mediaType"`
		Size      int64  `json:"size"`
		Digest    string `json:"digest"`
	} `json:"config"`
	Layers []RegistryLayer `json:"layers"`
}

// RegistryLayer defines the Docker image layer from the registry's point of view.
type RegistryLayer struct {
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Digest    string `json:"digest"`
}

// RegistryManifest builds the registry manifest for the given repo tag. If no
// manifest matches the tag, it returns ErrNotFound.
func (a *Archive) RegistryManifest(repoTag string) (RegistryManifest, error) {
	for _, m := range a.Manifests {
		for _, rt := range m.RepoTags {
			if dockerref.CompareReferences(repoTag, rt) {
				return a.registryManifest(m), nil
			}
		}
	}

	return RegistryManifest{}, ErrNotFound
}

// registryManifest converts a stored manifest into its registry representation.
func (a *Archive) registryManifest(m Manifest) RegistryManifest {
	result := RegistryManifest{SchemaVersion: 2, MediaType: manifestMediaType}
	result.Config.MediaType = configMediaType
	result.Config.Size = a.UncompressedSize(m.Config)
	result.Config.Digest = "sha256:" + a.reader.GetHash(dizPrefix+m.Config)
	for _, l := range m.Layers {
		result.Layers = append(result.Layers, RegistryLayer{
			MediaType: layerMediaType,
			Size:      a.UncompressedSize(l),
			Digest:    "sha256:" + a.reader.GetHash(dizPrefix+l),
		})
	}
	return result
}

// MarshalRegistryManifest returns the indented JSON encoding of the manifest and
// its sha256 digest (hex, without the "sha256:" prefix).
func MarshalRegistryManifest(m RegistryManifest) (js []byte, digest string, err error) {
	if js, err = json.MarshalIndent(m, "", "  "); err != nil {
		return nil, "", err
	}
	return js, fmt.Sprintf("%x", sha256.Sum256(js)), nil
}
