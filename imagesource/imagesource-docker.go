package imagesource

import (
	"fmt"
	"io"
	"io/ioutil"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/dockerref"
	"github.com/JohanLindvall/diz/hashzip"
	"github.com/JohanLindvall/diz/str"
	"github.com/JohanLindvall/diz/util"
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
	result, err = s.globTags(globTags)

	return
}

func (s *dockerImageSource) globTags(globTags []string) (result []string, err error) {
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
		existing, err := s.globTags(tags)
		if err != nil {
			return err
		}
		for _, tag := range str.RemoveSlice(tags, existing) {
			normalized := dockerref.NormalizeReference(tag)
			fmt.Printf("Pulling '%s'\n", normalized)
			reader, err := s.cli.ImagePull(context.Background(), normalized, types.ImagePullOptions{RegistryAuth: getCredentials(dockerref.GetRepository(normalized))})
			if err != nil {
				return err
			}
			if _, err = util.CopyAndClose(ioutil.Discard, reader); err != nil {
				return err
			}
		}
	}

	return nil
}
