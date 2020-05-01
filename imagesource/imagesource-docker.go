package imagesource

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/dockerref"
	"github.com/JohanLindvall/diz/hashzip"
	"github.com/JohanLindvall/diz/str"
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
			ipo := types.ImagePullOptions{}
			user, pass, err := getCredentials(strings.SplitN(normalized, "/", -1)[0])
			if err == nil {
				b, _ := json.Marshal(&types.AuthConfig{Username: user, Password: pass})
				ipo.RegistryAuth = base64.URLEncoding.EncodeToString(b)
			}

			fmt.Printf("Pulling '%s'\n", normalized)
			reader, err := s.cli.ImagePull(context.Background(), normalized, ipo)
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
