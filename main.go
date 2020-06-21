package main

import (
	"compress/flate"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/dockerref"
	"github.com/JohanLindvall/diz/hashzip"
	"github.com/JohanLindvall/diz/imagesource"
	"github.com/JohanLindvall/diz/str"
	"github.com/JohanLindvall/diz/util"
	"github.com/docker/docker/client"
)

var (
	cli             *client.Client
	fromZip         = flag.String("fromzip", "", "Set to read Docker tags and images from zip file")
	tagFile         = flag.String("tagfile", "", "Set to read and write tags from file")
	digestTags      = flag.Bool("digest", false, "If set, update tags to use repo digest")
	pull            = flag.Bool("pull", false, "If set, pulls images from docker registry")
	level           = flag.Int("level", flate.DefaultCompression, "Sets the deflate compression level (0-9)")
	registryAddress = flag.String("registryAddress", "", "Sets the registry address of the given docker references")
)

func main() {
	flag.Parse()

	ccli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	cli = ccli

	args := flag.Args()
	switch args[0] {
	case "list":
		err = list(getTags(args[1:]))
	case "create":
		err = create(args[1], getTags(args[2:]))
	case "update":
		err = update(args[1], args[2], getTags(args[3:]))
	case "restore":
		err = restore(getTags(args[1:]))
	case "serve":
		err = serve(args[1])
	default:
		err = errors.New("bad command")
	}
	if err != nil {
		panic(err)
	}
	os.Exit(0)
}

func create(fn string, globTags []string) error {
	return createUpdate(imagesource.NewNullImageSource(), fn, globTags)
}

func update(initial string, fn string, globTags []string) error {
	if initialSource, err := getNamedImageSource(initial); err == nil {
		err = createUpdate(initialSource, fn, globTags)
		if er := initialSource.Close(); err == nil {
			err = er
		}
		return err
	} else {
		return err
	}
}

func createUpdate(initial imagesource.ImageSource, fn string, globTags []string) error {
	if s, err := getImageSource(); err != nil {
		return err
	} else {
		defer s.Close()
		if tags, err := s.GlobTags(globTags); err != nil {
			return err
		} else {
			var out *os.File
			if out, err = getOutFile(fn); err != nil {
				return err
			}
			defer out.Close()
			zipWriter := hashzip.NewWriterLevel(out, *level)

			// Copy tags and contents from initial image source (if there is one)
			var copyTags []string
			if copyTags, err = initial.GlobTags([]string{"*"}); err != nil {
				return err
			}

			// Remove tags to be updated
			copyTags = str.RemoveSlice(copyTags, tags)
			var m1, m2 []diz.Manifest
			if m1, err = initial.CopyToZip(zipWriter, copyTags); err != nil {
				return err
			}

			if m2, err = s.CopyToZip(zipWriter, tags); err != nil {
				return err
			} else {
				err = diz.WriteManifests(diz.MergeManifests(m1, m2), zipWriter)
				if er := zipWriter.Close(); err == nil {
					err = er
				}
				if err == nil {
					err = updateTags(fn)
				}
				return err
			}
		}
	}
}

func restore(globTags []string) error {
	if s, err := getImageSource(); err != nil {
		return err
	} else {
		defer s.Close()
		if tags, err := s.GlobTags(globTags); err != nil {
			return err
		} else {
			fmt.Printf("Restoring %s\n", strings.Join(tags, ", "))
			if rdr, err := s.ReadTar(tags); err != nil {
				return err
			} else {
				defer rdr.Close()
				if response, err := cli.ImageLoad(context.Background(), rdr, false); err != nil {
					return err
				} else {
					response.Body.Close()
				}
			}
		}
	}

	return nil
}

func list(tags []string) error {
	if s, err := getImageSource(); err != nil {
		return err
	} else {
		defer s.Close()
		if result, err := s.GlobTags(tags); err != nil {
			return err
		} else {
			for _, r := range result {
				fmt.Println(r)
			}
		}
	}
	return nil
}

func getImageSource() (imagesource.ImageSource, error) {
	return getNamedImageSource(*fromZip)
}

func getNamedImageSource(fn string) (imagesource.ImageSource, error) {
	if fn == "" {
		return imagesource.NewDockerImageSource(cli, *pull), nil
	} else {
		return imagesource.NewZipImageSource(fn)
	}
}

func getOutFile(fn string) (out *os.File, err error) {
	if fn == "-" {
		out = os.Stdout
	} else {
		out, err = os.Create(fn)
	}

	return
}

func getTags(tags []string) []string {
	if *tagFile != "" {
		if lines, err := util.ReadLines(*tagFile); err == nil {
			for _, line := range lines {
				if tag := getDockerReference(line); tag != "" {
					tags = append(tags, tag)
				}
			}
		}
	}

	return tags
}

func updateTags(zipFile string) error {
	if !*digestTags && *registryAddress == "" {
		return nil
	}
	zf, err := imagesource.NewZipImageSource(zipFile)
	if err != nil {
		return err
	}
	tagsToDigest, err := zf.GetNormalizedTagsToDigest()
	if err != nil {
		return err
	}

	if *tagFile == "" {
		for tag, digest := range tagsToDigest {
			fmt.Printf("%s=%s\n", tag, digest)
		}
	} else {
		lines, err := util.ReadLines(*tagFile)
		if err != nil {
			return err
		}
		changed := false
		for i, line := range lines {
			if ref := getDockerReference(line); ref != "" {
				if digest, ok := tagsToDigest[dockerref.NormalizeReference(ref)]; ok {
					registry, repository, tag := dockerref.SplitRegistryRepositoryTag(ref)
					if *digestTags {
						tag = dockerref.MakeDigestTag(digest)
					}
					if *registryAddress != "" {
						registry = *registryAddress
					}
					newRef := dockerref.JoinRegistryRepositoryTag(registry, repository, tag)
					line = strings.Replace(line, ref, newRef, -1)
					if lines[i] != line {
						changed = true
						lines[i] = line
					}
				}
			}
		}

		if changed {
			err = util.WriteLines(*tagFile, lines)
		}
	}

	return err
}

func getDockerReference(line string) string {
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
