package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/hashzip"
	"github.com/JohanLindvall/diz/imagesource"
	"github.com/JohanLindvall/diz/registry"
)

// List prints the tags in the image source that match the given globs.
func (a *App) List(tags []string) error {
	s, err := a.imageSource()
	if err != nil {
		return err
	}
	defer s.Close()

	matched, err := s.GlobTags(tags)
	if err != nil {
		return err
	}
	for _, m := range matched {
		fmt.Println(m)
	}
	return nil
}

// Create writes a new zip archive fn containing the images matching tags.
func (a *App) Create(fn string, tags []string) error {
	return a.createUpdate(imagesource.NewNullImageSource(), fn, tags)
}

// Update writes a new zip archive fn seeded with the contents of the initial
// source, replacing any images matching tags with fresh copies.
func (a *App) Update(initial, fn string, tags []string) error {
	initialSource, err := a.namedImageSource(initial)
	if err != nil {
		return err
	}
	err = a.createUpdate(initialSource, fn, tags)
	if er := initialSource.Close(); err == nil {
		err = er
	}
	return err
}

// createUpdate copies the images matching tags from the primary source into fn,
// carrying over all other images from initial, then rewrites the tag file.
func (a *App) createUpdate(initial imagesource.ImageSource, fn string, globTags []string) (err error) {
	s, err := a.imageSource()
	if err != nil {
		return err
	}
	defer s.Close()

	tags, err := s.GlobTags(globTags)
	if err != nil {
		return err
	}

	out, err := outFile(fn)
	if err != nil {
		return err
	}
	defer out.Close()

	zipWriter := hashzip.NewWriterLevel(out, a.Level)

	// Carry over the tags and contents of the initial source, except the tags
	// being updated.
	copyTags, err := initial.GlobTags([]string{"*"})
	if err != nil {
		return err
	}
	copyTags = slices.DeleteFunc(copyTags, func(s string) bool { return slices.Contains(tags, s) })

	initialManifests, err := initial.CopyToZip(zipWriter, copyTags)
	if err != nil {
		return err
	}

	newManifests, err := s.CopyToZip(zipWriter, tags)
	if err != nil {
		return err
	}

	err = diz.WriteManifests(diz.Merge(initialManifests, newManifests), zipWriter)
	if er := zipWriter.Close(); err == nil {
		err = er
	}
	if err != nil {
		return err
	}

	return a.updateTagFile(fn)
}

// Restore loads the images matching tags from the source into the Docker daemon.
func (a *App) Restore(tags []string) error {
	s, err := a.imageSource()
	if err != nil {
		return err
	}
	defer s.Close()

	matched, err := s.GlobTags(tags)
	if err != nil {
		return err
	}
	fmt.Printf("Restoring %s\n", strings.Join(matched, ", "))

	rdr, err := s.ReadTar(matched)
	if err != nil {
		return err
	}
	defer rdr.Close()

	response, err := a.docker.ImageLoad(context.Background(), rdr)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	decoder := json.NewDecoder(response.Body)
	for {
		var message struct {
			Error string `json:"error"`
		}
		if err := decoder.Decode(&message); err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("read Docker load response: %w", err)
		}
		if message.Error != "" {
			return fmt.Errorf("load image: %s", message.Error)
		}
	}
}

// Serve serves the given zip archive as a Docker registry.
func (a *App) Serve(zip string) error {
	source, err := imagesource.NewZipImageSource(zip)
	if err != nil {
		return err
	}
	defer source.Close()
	return registry.ListenAndServe(defaultServeAddress, source)
}
