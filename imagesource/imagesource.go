package imagesource

import (
	"io"
	"os"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/zip"
	"github.com/docker/distribution/context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/ryanuber/go-glob"
)

// ImageSource defines an interface for working with Docker image sources
type ImageSource interface {
	GlobTags(tags []string) ([]string, error)
	Close() error
	// CopyToZip copies the image contents to the zip archive, but does not write the manifest.
	CopyToZip(writer *zip.Writer, tags []string) ([]diz.Manifest, error)
}

// NewZipImageSource returns a zip image source
func NewZipImageSource(zip string) (source ImageSource, err error) {
	var z zipImageSource
	if z.file, err = os.Open(os.Args[2]); err != nil {
		return
	}
	var fi os.FileInfo
	if fi, err = z.file.Stat(); err != nil {
		z.file.Close()
		return
	}
	if z.archive, err = diz.NewArchive(z.file, fi.Size()); err != nil {
		z.file.Close()
		return
	}
	source = &z

	return
}

type zipImageSource struct {
	file    *os.File
	archive *diz.Archive
}

func (z *zipImageSource) GlobTags(tags []string) (result []string, err error) {
	for _, m := range diz.FilterManifests(z.archive.Manifests, tags) {
		for _, t := range m.RepoTags {
			result = append(result, t)
		}
	}

	return
}

func (z *zipImageSource) Close() error {
	return z.file.Close()
}

func stringInSlice(s string, ss []string) bool {
	for _, sss := range ss {
		if s == sss {
			return true
		}
	}

	return false
}

func (z *zipImageSource) CopyToZip(writer *zip.Writer, tags []string) (m []diz.Manifest, err error) {
	m = diz.FilterManifests(z.archive.Manifests, tags)
	err = z.archive.CopyToZip(writer, m)

	return
}

// NewDockerImageSource returns a Docker image source
func NewDockerImageSource(cli *client.Client) (source ImageSource) {
	source = &dockerImageSource{cli: cli}
	return
}

type dockerImageSource struct {
	cli *client.Client
}

func (s *dockerImageSource) GlobTags(globTags []string) (result []string, err error) {
	var images []types.ImageSummary
	if images, err = s.cli.ImageList(context.Background(), types.ImageListOptions{}); err != nil {
		return
	}

	for _, image := range images {
		for _, repoTag := range image.RepoTags {
			if repoTag != "<none>:<none>" {
				for _, tag := range globTags {
					if glob.Glob(tag, repoTag) {
						result = append(result, repoTag)
						break
					}
				}
			}
		}
	}

	return
}

func (s *dockerImageSource) Close() error {
	return nil
}

func (s *dockerImageSource) CopyToZip(writer *zip.Writer, tags []string) (m []diz.Manifest, err error) {
	var rdr io.ReadCloser
	if rdr, err = s.cli.ImageSave(context.Background(), tags); err != nil {
		return
	}
	defer rdr.Close()

	m, err = diz.CopyFromTar(rdr, writer)

	return
}

// NewNullImageSource returns an image source with no tags and no images
func NewNullImageSource() (source ImageSource) {
	source = &nullImageSource{}
	return
}

type nullImageSource struct {
}

func (s *nullImageSource) GlobTags(globTags []string) (result []string, err error) {
	return
}

func (s *nullImageSource) Close() (err error) {
	return
}

func (s *nullImageSource) CopyToZip(writer *zip.Writer, tags []string) (m []diz.Manifest, err error) {
	return
}
