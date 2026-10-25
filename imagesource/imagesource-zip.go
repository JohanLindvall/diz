package imagesource

import (
	"io"
	"os"

	"github.com/JohanLindvall/diz/diz"
	"github.com/JohanLindvall/diz/dockerref"
	"github.com/JohanLindvall/diz/hashzip"
)

// NewZipImageSource returns a zip image source reading from the given file.
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

// ZipImageSource is an ImageSource backed by a diz zip archive on disk.
type ZipImageSource struct {
	file    *os.File
	archive *diz.Archive
}

func (z *ZipImageSource) GlobTags(tags []string) (result []string, err error) {
	for _, m := range diz.Filter(z.archive.Manifests, tags) {
		result = append(result, m.RepoTags...)
	}

	return
}

func (z *ZipImageSource) Close() error {
	return z.file.Close()
}

func (z *ZipImageSource) CopyToZip(writer *hashzip.Writer, tags []string) (m []diz.Manifest, err error) {
	m = diz.Filter(z.archive.Manifests, tags)
	err = z.archive.CopyToZip(writer, m)

	return
}

func (z *ZipImageSource) ReadTar(tags []string) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(z.archive.CopyToTar(pw, diz.Filter(z.archive.Manifests, tags)))
	}()
	return pr, nil
}

// Manifests returns the manifests stored in the archive.
func (z *ZipImageSource) Manifests() []diz.Manifest {
	return z.archive.Manifests
}

// RegistryManifest returns the registry manifest for the given repo tag.
func (z *ZipImageSource) RegistryManifest(repoTag string) (diz.RegistryManifest, error) {
	return z.archive.RegistryManifest(repoTag)
}

// WriteBlob writes the blob with the given content hash to the writer.
func (z *ZipImageSource) WriteBlob(writer io.Writer, hash string) error {
	return z.archive.WriteBlob(writer, hash)
}

// Read returns a reader for the given path within the archive.
func (z *ZipImageSource) Read(path string) (io.ReadCloser, error) {
	return z.archive.Read(path)
}

// DigestToTags maps each manifest digest to the repo tags that resolve to it.
func (z *ZipImageSource) DigestToTags() (result map[string][]string, err error) {
	result = make(map[string][]string)
	tags, _ := z.GlobTags([]string{"*"})
	for _, tag := range tags {
		var manifest diz.RegistryManifest
		if manifest, err = z.RegistryManifest(tag); err != nil {
			return
		}
		var digest string
		if _, digest, err = diz.MarshalRegistryManifest(manifest); err != nil {
			return
		}
		result[digest] = append(result[digest], tag)
	}
	return
}

// NormalizedTagsToDigest maps each normalized repo tag to its manifest digest.
func (z *ZipImageSource) NormalizedTagsToDigest() (result map[string]string, err error) {
	result = make(map[string]string)
	var digestToTags map[string][]string
	if digestToTags, err = z.DigestToTags(); err == nil {
		for digest, tags := range digestToTags {
			for _, tag := range tags {
				result[dockerref.NormalizeReference(tag)] = digest
			}
		}
	}

	return
}
