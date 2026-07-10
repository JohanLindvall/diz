package tagfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReference(t *testing.T) {
	assert.Equal(t, "nginx:latest", Reference("nginx:latest"))
	assert.Equal(t, "nginx:latest", Reference("  nginx:latest  "))
	assert.Equal(t, "nginx:latest", Reference("image=nginx:latest"))
	assert.Equal(t, "nginx:latest", Reference("image=nginx:latest # a comment"))
	assert.Equal(t, "", Reference("# just a comment"))
	assert.Equal(t, "", Reference(""))
}

func TestLoad(t *testing.T) {
	path := writeTemp(t, "nginx:latest\n# comment\nimage=redis:5\n\n")
	refs, err := Load(path)
	assert.NoError(t, err)
	assert.Equal(t, []string{"nginx:latest", "redis:5"}, refs)
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist"))
	assert.Error(t, err)
}

func TestRewrite(t *testing.T) {
	path := writeTemp(t, "image=nginx:latest # keep comment\nredis:5\n")
	err := Rewrite(path, func(ref string) (string, bool) {
		if ref == "nginx:latest" {
			return "registry.example.com/nginx:latest", true
		}
		return "", false
	})
	assert.NoError(t, err)
	assert.Equal(t,
		"image=registry.example.com/nginx:latest # keep comment\nredis:5\n",
		readFile(t, path))
}

func TestRewriteNoChange(t *testing.T) {
	original := "nginx:latest\nredis:5\n"
	path := writeTemp(t, original)
	err := Rewrite(path, func(ref string) (string, bool) { return "", false })
	assert.NoError(t, err)
	assert.Equal(t, original, readFile(t, path))
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tags")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
