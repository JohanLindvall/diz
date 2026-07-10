// Package tagfile reads and rewrites files that list Docker image references,
// one per line. Lines may take the form "key=value" and may carry a trailing
// "# comment"; only the reference portion is interpreted.
package tagfile

import (
	"strings"

	"github.com/JohanLindvall/diz/dockerref"
	"github.com/JohanLindvall/diz/util"
)

// Reference extracts the Docker reference from a single line, or returns "" if
// the line holds no valid reference.
func Reference(line string) string {
	if i := strings.Index(line, "#"); i != -1 {
		line = line[:i]
	}
	if split := strings.SplitN(line, "=", 2); len(split) == 2 {
		line = split[1]
	}
	line = strings.TrimSpace(line)

	if dockerref.IsValidReference(line) {
		return line
	}

	return ""
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
func Rewrite(path string, transform func(ref string) (string, bool)) error {
	lines, err := util.ReadLines(path)
	if err != nil {
		return err
	}

	changed := false
	for i, line := range lines {
		ref := Reference(line)
		if ref == "" {
			continue
		}
		newRef, ok := transform(ref)
		if !ok || newRef == ref {
			continue
		}
		if updated := strings.ReplaceAll(line, ref, newRef); updated != line {
			lines[i] = updated
			changed = true
		}
	}

	if !changed {
		return nil
	}
	return util.WriteLines(path, lines)
}
