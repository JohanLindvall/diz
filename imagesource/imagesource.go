package imagesource

import (
	"io"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/hashzip"
)

// ImageSource defines an interface for working with Docker image sources
type ImageSource interface {
	GlobTags(tags []string) ([]string, error)
	Close() error
	// CopyToZip copies the image contents to the zip archive, but does not write the manifest.
	CopyToZip(writer *hashzip.Writer, tags []string) ([]diz.Manifest, error)
	// ReadTar returns an io.Reader with a tar archive of the contents
	ReadTar(tags []string) (io.ReadCloser, error)
}
