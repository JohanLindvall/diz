package imagesource

import (
	"io"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/hashzip"
)

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

func (s *nullImageSource) CopyToZip(writer *hashzip.Writer, tags []string) (m []diz.Manifest, err error) {
	return
}

func (s *nullImageSource) ReadTar(tags []string) (rdr io.ReadCloser, err error) {
	rdr = s
	return
}

func (s *nullImageSource) Read(b []byte) (n int, err error) {
	n = 0
	err = io.EOF
	return
}
