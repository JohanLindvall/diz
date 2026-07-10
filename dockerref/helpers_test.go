package dockerref

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRegistry(t *testing.T) {
	assert.Equal(t, "gcr.io", GetRegistry("gcr.io/etcd-development/etcd:v3.4.9"))
	assert.Equal(t, "192.168.1.1:5000", GetRegistry("192.168.1.1:5000/foo/bar:xyz"))
	assert.Equal(t, "", GetRegistry("nginx"))
	assert.Equal(t, "", GetRegistry("foo/bar:xyz"))
}

func TestIsValidReference(t *testing.T) {
	assert.True(t, IsValidReference("nginx"))
	assert.True(t, IsValidReference("nginx:latest"))
	assert.True(t, IsValidReference("gcr.io/etcd-development/etcd:v3.4.9"))
	// more than one tag separator is not a valid reference
	assert.False(t, IsValidReference("a:b:c"))
}

func TestCompareReferences(t *testing.T) {
	assert.True(t, CompareReferences("nginx", "nginx"))
	assert.True(t, CompareReferences("nginx", "docker.io/library/nginx:latest"))
	assert.True(t, CompareReferences("foo/bar:xyz", "docker.io/foo/bar:xyz"))
	assert.False(t, CompareReferences("nginx", "redis"))
	assert.False(t, CompareReferences("nginx:1", "nginx:2"))
}

func TestMakeDigestTag(t *testing.T) {
	assert.Equal(t, "sha256:abc123", MakeDigestTag("abc123"))
}
