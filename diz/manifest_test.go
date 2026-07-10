package diz

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterTags(t *testing.T) {
	// simple glob match
	assert.Equal(t,
		[]string{"nginx:latest"},
		FilterTags([]string{"nginx:latest", "redis:5"}, nil, []string{"nginx:*"}))

	// include all then exclude with a negative glob
	assert.Equal(t,
		[]string{"a:1", "c:3"},
		FilterTags([]string{"a:1", "b:2", "c:3"}, nil, []string{"*", "-b:*"}))

	// the placeholder tag is always dropped
	assert.Equal(t,
		[]string{"x:1"},
		FilterTags([]string{"<none>:<none>", "x:1"}, nil, []string{"*"}))

	// repo digests match by reference equality
	assert.Equal(t,
		[]string{"repo@sha256:" + zeroDigest},
		FilterTags(nil, []string{"repo@sha256:" + zeroDigest}, []string{"repo@sha256:" + zeroDigest}))
}

func TestFilter(t *testing.T) {
	manifests := []Manifest{
		{Config: "c1", RepoTags: []string{"nginx:latest", "nginx:1"}},
		{Config: "c2", RepoTags: []string{"redis:5"}},
	}
	result := Filter(manifests, []string{"nginx:*"})
	assert.Len(t, result, 1)
	assert.Equal(t, "c1", result[0].Config)
	assert.Equal(t, []string{"nginx:latest", "nginx:1"}, result[0].RepoTags)
}

func TestMergeDistinctConfigs(t *testing.T) {
	result := Merge(
		[]Manifest{{Config: "c1", RepoTags: []string{"a:1"}}},
		[]Manifest{{Config: "c2", RepoTags: []string{"b:1"}}},
	)
	assert.Len(t, result, 2)
}

func TestMergeSameConfigUnionsTags(t *testing.T) {
	result := Merge(
		[]Manifest{{Config: "c1", RepoTags: []string{"a:1"}}},
		[]Manifest{{Config: "c1", RepoTags: []string{"a:1", "b:1", "c:1"}}},
	)
	assert.Len(t, result, 1)
	// all new tags are retained (regression: earlier only the last survived)
	assert.Equal(t, []string{"a:1", "b:1", "c:1"}, result[0].RepoTags)
}

// zeroDigest is a valid-looking 64-hex digest used in reference tests.
const zeroDigest = "0000000000000000000000000000000000000000000000000000000000000000"
