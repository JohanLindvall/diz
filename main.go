package main

import (
	gozip "archive/zip"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/DataDog/zstd"
	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/imagesource"
	"github.com/JohanLindvall/diz/str"
	"github.com/JohanLindvall/diz/zip"
	"github.com/docker/docker/client"
)

var (
	cli     *client.Client
	fromZip = flag.String("fromzip", "", "set to read Docker tags and images from zip file")
	level   = flag.Int("level", 5, "Set the compressions level. 0-22")
)

const (
	zstdMethod = 1337
)

func main() {
	flag.Parse()

	gozip.RegisterCompressor(zstdMethod, func(wr io.Writer) (io.WriteCloser, error) {
		return zstd.NewWriterLevel(wr, *level), nil
	})
	gozip.RegisterDecompressor(zstdMethod, func(r io.Reader) io.ReadCloser {
		return zstd.NewReader(r)
	})
	ccli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	cli = ccli

	args := flag.Args()
	switch args[0] {
	case "list":
		err = list(args[1:])
	case "create":
		err = create(args[1], args[2:])
	case "update":
		err = update(args[1], args[2], args[3:])
	case "restore":
		err = restore(args[1:])
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
		return createUpdate(initialSource, fn, globTags)
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
			zipWriter := zip.NewWriterMethod(out, zstdMethod)

			// Copy tags and contents from initial image source (if there is one)
			var copyTags []string
			if copyTags, err = initial.GlobTags([]string{"*"}); err != nil {
				return err
			}

			// Remove tags to be updated
			copyTags = str.RemoveSlice(copyTags, globTags)
			var m1, m2 []diz.Manifest
			if m1, err = s.CopyToZip(zipWriter, copyTags); err != nil {
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
		return imagesource.NewDockerImageSource(cli), nil
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
