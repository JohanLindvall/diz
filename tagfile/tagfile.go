// Package tagfile reads and rewrites files that list Docker image references,
// one per line. Lines may take the form "key=value" and may carry a trailing
// "# comment"; only the reference portion is interpreted.
package tagfile

import (
	"os"
	"strings"

	"github.com/JohanLindvall/diz/dockerref"
	"github.com/JohanLindvall/diz/util"
)

// Reference extracts the Docker reference from a single line, or returns "" if
// the line holds no valid reference.
func Reference(line string) string {
	start, end := referenceSpan(line)
	return line[start:end]
}

// referenceSpan locates the reference without including its key or comment.
func referenceSpan(line string) (start, end int) {
	end = len(line)
	if i := strings.IndexByte(line, '#'); i != -1 {
		end = i
	}
	if i := strings.IndexByte(line[:end], '='); i != -1 {
		start = i + 1
	}
	value := line[start:end]
	ref := strings.TrimSpace(value)
	if ref == "" || !dockerref.IsValidReference(ref) {
		return 0, 0
	}
	start += strings.Index(value, ref)
	return start, start + len(ref)
}

// Load reads path and returns every Docker reference found in it.
func Load(path string) ([]string, error) {
	lines, err := util.ReadLines(path)
	if err != nil {
		return nil, err
	}
	var refs []string
	for _, line := range lines {
		if ref := Reference(line); ref != "" {
			refs = append(refs, ref)
		}
	}
	return refs, nil
}

// Rewrite applies transform to the reference on each line of path and writes the
// file back if anything changed. transform returns the replacement reference and
// true to substitute it in place, or false to leave the line unchanged.
// Keys, comments, whitespace and line endings are preserved.
func Rewrite(path string, transform func(ref string) (string, bool)) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")

	changed := false
	for i, line := range lines {
		start, end := referenceSpan(line)
		ref := line[start:end]
		if ref == "" {
			continue
		}
		newRef, ok := transform(ref)
		if !ok || newRef == ref {
			continue
		}
		lines[i] = line[:start] + newRef + line[end:]
		changed = true
	}

	if !changed {
		return nil
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}
