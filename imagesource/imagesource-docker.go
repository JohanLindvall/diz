package imagesource

import (
	"io"
	"io/ioutil"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/dockerref"
	"github.com/JohanLindvall/diz/hashzip"
	"github.com/docker/distribution/context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// NewDockerImageSource returns a Docker image source
func NewDockerImageSource(cli *client.Client, pull bool) (source ImageSource) {
	source = &dockerImageSource{cli: cli, pull: pull}
	return
}

type dockerImageSource struct {
	cli  *client.Client
	pull bool
}

func (s *dockerImageSource) GlobTags(globTags []string) (result []string, err error) {
	if s.pull {
		result = globTags
		return
	}
	var images []types.ImageSummary
	if images, err = s.cli.ImageList(context.Background(), types.ImageListOptions{}); err != nil {
		return
	}

	for _, image := range images {
		result = append(result, diz.FilterImageTags(image.RepoTags, globTags)...)
	}

	return
}

func (s *dockerImageSource) Close() error {
	return nil
}

func (s *dockerImageSource) CopyToZip(writer *hashzip.Writer, tags []string) (m []diz.Manifest, err error) {
	if err = s.pullIfNeeded(tags); err != nil {
		return
	}
	var rdr io.ReadCloser
	if rdr, err = s.cli.ImageSave(context.Background(), tags); err != nil {
		return
	}
	defer rdr.Close()

	m, err = diz.CopyFromTar(rdr, writer)

	return
}

func (s *dockerImageSource) ReadTar(tags []string) (io.ReadCloser, error) {
	if err := s.pullIfNeeded(tags); err != nil {
		return nil, err
	}
	return s.cli.ImageSave(context.Background(), tags)
}

func (s *dockerImageSource) pullIfNeeded(tags []string) error {
	if s.pull {
		for _, tag := range tags {
			normalized := dockerref.NormalizeReference(tag)
			reader, err := s.cli.ImagePull(context.Background(), normalized, types.ImagePullOptions{})
			if err != nil {
				return err
			}
			_, err = io.Copy(ioutil.Discard, reader)
			reader.Close()
			if err != nil {
				return err
			}
		}
	}

	return nil
}
