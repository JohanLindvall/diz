package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/imagesource"
	"github.com/JohanLindvall/diz/str"
	"github.com/JohanLindvall/diz/zip"
	"github.com/docker/docker/client"
)

var (
	cli     *client.Client
	fromZip = flag.String("fromzip", "", "set to read Docker tags and images from zip file")
)

func main() {
	ccli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	cli = ccli
	flag.Parse()

	args := flag.Args()
	switch args[0] {
	case "list":
		list(args[1:])
	case "create":
		create(args[1], args[2:])
	case "update":
		// Copy everything from args[1] to out (args[2]), except the tags found in imageSource.
	default:
		os.Exit(1)
	}
	os.Exit(0)

	if os.Args[1] == "restore" {
		file, err := os.Open(os.Args[2])
		if err != nil {
			panic(err)
		}
		stat, err := file.Stat()
		if err != nil {
			panic(err)
		}
		err = restore(file, stat.Size(), os.Args[3:])
		if err != nil {
			panic(err)
		}
	}
}

func create(fn string, globTags []string) error {
	return createUpdate(imagesource.NewNullImageSource(), fn, globTags)
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
			zipWriter := zip.NewWriter(out)

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

func restore(reader io.ReaderAt, size int64, tags []string) error {
	archive, err := diz.NewArchive(reader, size)
	if err != nil {
		return err
	}

	manifest := diz.FilterManifests(archive.Manifests, tags)

	tar, _ := os.Create("c:\\temp\\data.tar")
	err = archive.CopyToTar(tar, manifest)
	tar.Close()
	return err
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
	if *fromZip == "" {
		return imagesource.NewDockerImageSource(cli), nil
	} else {
		return imagesource.NewZipImageSource(*fromZip)
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
