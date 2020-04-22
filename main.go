package main

import (
	"archive/zip"
	"context"
	"io"
	"os"

	"github.com/JohanLindvall/diz/diz"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/ryanuber/go-glob"
)

var (
	cli *client.Client
)

func main() {
	ccli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	cli = ccli

	if os.Args[1] == "create" {
		var writer io.Writer
		if os.Args[2] == "-" {
			writer = os.Stdout
		} else {
			outf, err := os.Create(os.Args[2])
			if err != nil {
				panic(err)
			}
			writer = outf
			defer outf.Close()
		}
		err := create(writer, os.Args[3:])
		if err != nil {
			panic(err)
		}
	} else if os.Args[1] == "restore" {
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

func create(writer io.Writer, tags []string) error {
	images, err := cli.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		return err
	}

	imageIds := make([]string, 0)

	for _, image := range images {
		for _, tag := range tags {
			for _, repoTag := range image.RepoTags {
				if repoTag != "<none>:<none>" {
					if glob.Glob(tag, repoTag) {
						imageIds = append(imageIds, repoTag)
					}
				}
			}
		}
	}

	rdr, err := cli.ImageSave(context.Background(), imageIds)
	if err != nil {
		return err
	}
	defer rdr.Close()
	zipWriter := zip.NewWriter(writer)

	manifests, err := diz.CopyFromTar(rdr, zipWriter)
	if err != nil {
		return err
	}
	err = diz.WriteManifests(manifests, zipWriter)
	if err != nil {
		return err
	}

	return zipWriter.Close()
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
