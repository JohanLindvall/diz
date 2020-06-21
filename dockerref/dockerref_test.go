package dockerref

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var testData = []struct {
	ref, norm, fam, reg, repo, tag string
}{
	{ref: "nginx",
		norm: "docker.io/library/nginx:latest",
		repo: "nginx"},
	{ref: "nginx:latest",
		norm: "docker.io/library/nginx:latest",
		fam:  "nginx",
		repo: "nginx", tag: "latest"},
	{ref: "foo/bar:xyz",
		norm: "docker.io/foo/bar:xyz",
		repo: "foo/bar", tag: "xyz"},
	{ref: "gcr.io/etcd-development/etcd:v3.4.9",
		reg: "gcr.io", repo: "etcd-development/etcd", tag: "v3.4.9"},
	{ref: "192.168.1.1:5000/foo/bar:xyz",
		reg: "192.168.1.1:5000", repo: "foo/bar", tag: "xyz"},
	{ref: "nginx@sha256:21f32f6c08406306d822a0e6e8b7dc81f53f336570e852e25fbe1e3e3d0d0133",
		norm: "docker.io/library/nginx@sha256:21f32f6c08406306d822a0e6e8b7dc81f53f336570e852e25fbe1e3e3d0d0133",
		repo: "nginx", tag: "sha256:21f32f6c08406306d822a0e6e8b7dc81f53f336570e852e25fbe1e3e3d0d0133"},
}

func init() {
	for i, data := range testData {
		if data.norm == "" {
			data.norm = data.ref
		}
		if data.fam == "" {
			data.fam = data.ref
		}
		testData[i] = data
	}
}

func Test_SplitRegistryRepository(t *testing.T) {
	for _, data := range testData {
		reg, repo, tag := SplitRegistryRepositoryTag(data.ref)
		assert.Equal(t, data.reg, reg)
		assert.Equal(t, data.repo, repo)
		assert.Equal(t, data.tag, tag)
	}
}

func Test_JoinRegistryRepository(t *testing.T) {
	for _, data := range testData {
		ref := JoinRegistryRepositoryTag(data.reg, data.repo, data.tag)
		assert.Equal(t, data.ref, ref)
	}
}

func Test_NormalizeReference(t *testing.T) {
	for _, data := range testData {
		norm := NormalizeReference(data.ref)
		assert.Equal(t, data.norm, norm)
	}
}

func Test_FamiliarizeReference(t *testing.T) {
	for _, data := range testData {
		fam := FamiliarizeReference(data.norm)
		assert.Equal(t, data.fam, fam)
	}
}
