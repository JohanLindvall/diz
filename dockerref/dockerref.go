package dockerref

import (
	"regexp"
	"strings"
)

var (
	refRe         = regexp.MustCompile(`^((localhost|([a-z0-9]+(\.[a-z0-9]+)+)(:[0-9]+)?)/)?(.*)+$`)
	dockerIoSlash = "docker.io/"
	librarySlash  = "library/"
)

func NormalizeReference(tag string) string {
	match := refRe.FindStringSubmatch(tag)
	if match == nil {
		return tag
	}
	if match[1] == "" {
		match[1] = dockerIoSlash
	}
	if match[1] == dockerIoSlash {
		if strings.Index(match[6], "/") == -1 {
			match[6] = librarySlash + match[6]
		}
	}
	return match[1] + match[6]
}

func FamiliarReference(reference string) string {
	match := refRe.FindStringSubmatch(reference)
	if match == nil {
		return reference
	}
	if match[1] == "" || match[1] == dockerIoSlash && strings.HasPrefix(match[6], librarySlash) {
		match[6] = string([]byte(match[6])[len(librarySlash):])
	}
	return match[6]
}
