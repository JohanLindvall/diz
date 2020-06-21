package imagesource

import (
	"io"
	"os"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/dockerref"
	"github.com/JohanLindvall/diz/hashzip"
)

// NewZipImageSource returns a zip image source
func NewZipImageSource(zip string) (source *ZipImageSource, err error) {
	var z ZipImageSource
	if z.file, err = os.Open(zip); err != nil {
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

type ZipImageSource struct {
	file    *os.File
	archive *diz.Archive
}

func (z *ZipImageSource) GlobTags(tags []string) (result []string, err error) {
	for _, m := range diz.FilterManifests(z.archive.Manifests, tags) {
		for _, t := range m.RepoTags {
			result = append(result, t)
		}
	}

	return
}

func (z *ZipImageSource) Close() error {
	return z.file.Close()
}

func (z *ZipImageSource) CopyToZip(writer *hashzip.Writer, tags []string) (m []diz.Manifest, err error) {
	m = diz.FilterManifests(z.archive.Manifests, tags)
	err = z.archive.CopyToZip(writer, m)

	return
}

func (z *ZipImageSource) ReadTar(tags []string) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(z.archive.CopyToTar(pw, diz.FilterManifests(z.archive.Manifests, tags)))
	}()
	return pr, nil
}

func (z *ZipImageSource) GetRegistryManifest(repoTag string) (diz.RegistryManifest, error) {
	return z.archive.GetRegistryManifest(repoTag)
}

func (z *ZipImageSource) WriteFileByHash(writer io.Writer, layer string) error {
	return z.archive.WriteFileByHash(writer, layer)
}

func (z *ZipImageSource) Manifests() []diz.Manifest {
	return z.archive.Manifests
}

func (z *ZipImageSource) Read(path string) (io.ReadCloser, error) {
	return z.archive.Read(path)
}

func (z *ZipImageSource) GetDigestToTags() (result map[string][]string, err error) {
	result = make(map[string][]string)
	tags, _ := z.GlobTags([]string{"*"})
	for _, tag := range tags {
		var repoManifest diz.RegistryManifest
		if repoManifest, err = z.GetRegistryManifest(tag); err != nil {
			break
		}
		var digest string
		if _, digest, err = diz.GetManifestBytes(repoManifest); err != nil {
			break
		}
		result[digest] = append(result[digest], tag)
	}
	return
}

func (z *ZipImageSource) GetNormalizedTagsToDigest() (result map[string]string, err error) {
	result = make(map[string]string)
	var digestToTags map[string][]string
	if digestToTags, err = z.GetDigestToTags(); err == nil {
		for digest, tags := range digestToTags {
			for _, tag := range tags {
				result[dockerref.NormalizeReference(tag)] = digest
			}
		}
	}

	return
}
