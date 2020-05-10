package main

import (
	"bufio"
	"compress/flate"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/hashzip"
	"github.com/JohanLindvall/diz/imagesource"
	"github.com/JohanLindvall/diz/str"
	"github.com/docker/docker/client"
)

var (
	cli     *client.Client
	fromZip = flag.String("fromzip", "", "Set to read Docker tags and images from zip file")
	tagFile = flag.String("tagfile", "", "Set to load tags from file")
	pull    = flag.Bool("pull", false, "If set, pulls images from docker registry")
	level   = flag.Int("level", flate.DefaultCompression, "Sets the deflate compression level (0-9)")
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

var (
	tagRe = regexp.MustCompile("[a-zA-Z0-9/:._-]+")
)

func getTags(tags []string) []string {
	if *tagFile != "" {
		if f, err := os.Open(*tagFile); err == nil {
			for s := bufio.NewScanner(f); s.Scan(); {
				ln := s.Text()
				if i := strings.Index(ln, "#"); i != -1 {
					ln = ln[:i]
				}
				if split := strings.SplitN(ln, "=", 2); len(split) == 2 {
					ln = split[1]
				}
				if match := tagRe.FindString(ln); match != "" {
					tags = append(tags, match)
				}
			}
			f.Close()
		}
	}
	return tags
}
