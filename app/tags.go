package app

import (
	"fmt"

	"github.com/JohanLindvall/diz/dockerref"
	"github.com/JohanLindvall/diz/imagesource"
	"github.com/JohanLindvall/diz/tagfile"
)

// Tags appends the references found in the configured tag file (if any) to the
// given command-line tags. A tag file that cannot be read is ignored.
func (a *App) Tags(tags []string) []string {
	if a.TagFile == "" {
		return tags
	}
	if refs, err := tagfile.Load(a.TagFile); err == nil {
		tags = append(tags, refs...)
	}
	return tags
}

// updateTagFile rewrites the tag file to reflect the digests of the images just
// written to zipFile, according to the DigestTags and RegistryAddress options.
// When no tag file is configured, the tag/digest pairs are printed instead.
func (a *App) updateTagFile(zipFile string) error {
	if !a.DigestTags && a.RegistryAddress == "" {
		return nil
	}

	zf, err := imagesource.NewZipImageSource(zipFile)
	if err != nil {
		return err
	}
	defer zf.Close()

	tagsToDigest, err := zf.NormalizedTagsToDigest()
	if err != nil {
		return err
	}

	if a.TagFile == "" {
		for tag, digest := range tagsToDigest {
			fmt.Printf("%s=%s\n", tag, digest)
		}
		return nil
	}

	return tagfile.Rewrite(a.TagFile, func(ref string) (string, bool) {
		digest, ok := tagsToDigest[dockerref.NormalizeReference(ref)]
		if !ok {
			return "", false
		}
		registry, repository, tag := dockerref.SplitRegistryRepositoryTag(ref)
		if a.DigestTags {
			tag = dockerref.MakeDigestTag(digest)
		}
		if a.RegistryAddress != "" {
			registry = a.RegistryAddress
		}
		return dockerref.JoinRegistryRepositoryTag(registry, repository, tag), true
	})
}
