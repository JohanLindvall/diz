package dockerref

import (
	"regexp"
	"strings"
)

var (
	refRe    = regexp.MustCompile(`^((localhost|([a-z0-9]+(\.[a-z0-9]+)+)(:[0-9]+)?)/)?([^:]*)(:([^:]*))?$`)
	sha256Re = regexp.MustCompile("^[a-fA-F0-9]{64}$")
)

const (
	dockerIo     = "docker.io"
	librarySlash = "library/"
	sha256Colon  = "sha256:"
	atSha256     = "@sha256"
	latest       = "latest"
)

// IsValidReference determines if the passed reference is valid
func IsValidReference(ref string) bool {
	return refRe.MatchString(ref)
}

// CompareReferences compares the two references for equality.
func CompareReferences(ref1, ref2 string) bool {
	if ref1 == ref2 {
		return true
	}
	return NormalizeReference(ref1) == NormalizeReference(ref2)
}

// NormalizeReference normalizes the docker reference, transforming "nginx" into "docker.io/library/nginx:latest"
func NormalizeReference(ref string) string {
	registry, repository, tag := SplitRegistryRepositoryTag(ref)
	registry, repository, tag = NormalizeRegistryRepositoryTag(registry, repository, tag)

	return JoinRegistryRepositoryTag(registry, repository, tag)
}

// FamiliarizeReference familarizes the docker reference, transforming "docker.io/library/nginx:latest" into "nginx"
func FamiliarizeReference(ref string) string {
	registry, repository, tag := SplitRegistryRepositoryTag(ref)
	if registry == "" && repository == "" && tag == "" {
		return ref
	}
	registry, repository, tag = FamiliarizeRegistryRepositorytag(registry, repository, tag)

	return JoinRegistryRepositoryTag(registry, repository, tag)
}

// NormalizeRegistryRepositoryTag normalizes the registry, repository and tag
func NormalizeRegistryRepositoryTag(registry, repository, tag string) (string, string, string) {
	if registry == "" {
		registry = dockerIo
	}
	if registry == dockerIo && !strings.Contains(repository, "/") {
		repository = librarySlash + repository
	}

	if tag == "" {
		tag = latest
	}

	return registry, repository, tag
}

// FamiliarizeRegistryRepositorytag familarizes the registry, repository and tag
func FamiliarizeRegistryRepositorytag(registry, repository, tag string) (string, string, string) {
	if registry == dockerIo {
		registry = ""
	}

	if registry == "" && strings.HasPrefix(repository, librarySlash) {
		repository = repository[len(librarySlash):]
	}

	if tag == latest {
		tag = ""
	}

	return registry, repository, tag
}

// GetRegistry get the registry from the passed reference
func GetRegistry(ref string) string {
	registry, _, _ := SplitRegistryRepositoryTag(ref)

	return registry
}

// SplitRegistryRepositoryTag splits the reference into registry, repository and tag
func SplitRegistryRepositoryTag(ref string) (string, string, string) {
	if match := refRe.FindStringSubmatch(ref); match == nil {
		return "", "", ""
	} else {
		if strings.HasSuffix(match[6], atSha256) && sha256Re.MatchString(match[8]) {
			match[6] = match[6][:len(match[6])-len(atSha256)]
			match[8] = sha256Colon + match[8]
		}
		return match[2], match[6], match[8]
	}
}

// JoinRegistryRepositoryTag joins registry, repository and tag into a reference
func JoinRegistryRepositoryTag(registry, repository, tag string) string {
	var sb strings.Builder
	if registry != "" {
		sb.WriteString(registry)
		sb.WriteRune('/')
	}
	isDigest := strings.HasPrefix(tag, sha256Colon) && sha256Re.MatchString(tag[len(sha256Colon):])
	sb.WriteString(repository)
	if isDigest {
		sb.WriteString(atSha256)
		tag = tag[len(sha256Colon):]
	}

	if tag != "" {
		sb.WriteRune(':')
		sb.WriteString(tag)
	}

	return sb.String()
}

// MakeDigestTag creates a digest tag from the digest
func MakeDigestTag(digest string) string {
	return sha256Colon + digest
}
