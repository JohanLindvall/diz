// Package app implements the diz commands (list, create, update, restore,
// serve) on top of the image source, archive and registry packages.
package app

import (
	"os"

	"github.com/JohanLindvall/diz/imagesource"
	"github.com/docker/docker/client"
)

// defaultServeAddress is the address the serve command listens on.
const defaultServeAddress = ":5000"

// Options configures an App. The zero value uses the local Docker daemon as the
// image source and writes plain tags.
type Options struct {
	// FromZip, when set, reads images from the named zip archive instead of the
	// local Docker daemon.
	FromZip string
	// TagFile, when set, is a file of image references that is read to augment
	// command arguments and rewritten by create/update.
	TagFile string
	// DigestTags rewrites tags in TagFile to use the repo digest.
	DigestTags bool
	// Pull pulls missing images from their registry before saving.
	Pull bool
	// Level is the deflate compression level (0-9).
	Level int
	// RegistryAddress overrides the registry of references written to TagFile.
	RegistryAddress string
}

// App holds the configuration and Docker client used to run diz commands.
type App struct {
	Options
	docker *client.Client
}

// New returns an App using the given Docker client and options.
func New(docker *client.Client, opts Options) *App {
	return &App{Options: opts, docker: docker}
}

// imageSource returns the configured primary image source (a zip archive when
// FromZip is set, otherwise the local Docker daemon).
func (a *App) imageSource() (imagesource.ImageSource, error) {
	return a.namedImageSource(a.FromZip)
}

// namedImageSource returns a zip image source for fn, or the Docker daemon when
// fn is empty.
func (a *App) namedImageSource(fn string) (imagesource.ImageSource, error) {
	if fn == "" {
		return imagesource.NewDockerImageSource(a.docker, a.Pull), nil
	}
	return imagesource.NewZipImageSource(fn)
}

// outFile opens fn for writing, or returns stdout when fn is "-".
func outFile(fn string) (*os.File, error) {
	if fn == "-" {
		return os.Stdout, nil
	}
	return os.Create(fn)
}
