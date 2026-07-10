package diz

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/JohanLindvall/diz/hashzip"
	"github.com/JohanLindvall/diz/util"
)

// errNotFound is returned when a requested blob cannot be located in the archive.
var errNotFound = errors.New("not found")

// Archive holds the data for reading a diz zip archive.
type Archive struct {
	Manifests []Manifest
	reader    *hashzip.Reader
}

// NewArchive creates a new archive from the reader.
func NewArchive(reader io.ReaderAt, size int64) (*Archive, error) {
	zipReader, err := hashzip.NewReader(reader, size)
	if err != nil {
		return nil, err
	}

	manifests, err := readManifests(zipReader)
	if err != nil {
		return nil, err
	}

	return &Archive{Manifests: manifests, reader: zipReader}, nil
}

// UncompressedSize returns the uncompressed size of the diz file with the given
// name, or -1 if it does not exist.
func (a *Archive) UncompressedSize(name string) int64 {
	if f := dizFile(a.reader, name); f != nil {
		return int64(f.UncompressedSize64)
	}
	return -1
}

// WriteBlob writes the file with the given content hash to the writer.
func (a *Archive) WriteBlob(writer io.Writer, hash string) error {
	f := a.reader.GetFileByHash(hash)
	if f == nil {
		return errNotFound
	}
	rdr, err := f.Open()
	if err != nil {
		return err
	}
	_, err = util.CopyAndClose(writer, rdr)
	return err
}

// Read returns a reader for the given path. If the path ends in "/" the reader
// yields a zip archive of the directory's contents; otherwise it yields the
// single file. A nil reader with a nil error means the path was not found.
func (a *Archive) Read(path string) (io.ReadCloser, error) {
	if strings.HasSuffix(path, "/") {
		return a.readDir(path), nil
	}
	if item := a.reader.GetFile(path); item != nil {
		return item.Open()
	}
	return nil, nil
}

// readDir streams a zip archive of every file under the given directory path.
func (a *Archive) readDir(path string) io.ReadCloser {
	r, w := io.Pipe()
	go func() {
		zw := hashzip.NewWriter(w)
		var err error
		base := strings.LastIndex(path[:len(path)-1], "/") + 1
		for _, item := range a.reader.File {
			if strings.HasPrefix(item.Name, path) {
				if err = zw.Copy(item.FileHeader.Name[base:], item); err != nil {
					break
				}
			}
		}
		if er := zw.Close(); err == nil {
			err = er
		}
		w.CloseWithError(err)
	}()
	return r
}

// dizFile returns the archive entry for the given diz-relative name, or nil.
func dizFile(zipReader *hashzip.Reader, name string) *hashzip.File {
	name = dizPrefix + name
	for _, file := range zipReader.File {
		if file.Name == name {
			return file
		}
	}

	return nil
}

// readManifests reads and decodes the manifest.json entry from the archive.
func readManifests(zipReader *hashzip.Reader) (manifests []Manifest, err error) {
	f := dizFile(zipReader, manifestJSON)
	if f == nil {
		return
	}

	var reader io.ReadCloser
	if reader, err = f.Open(); err != nil {
		return
	}
	defer reader.Close()

	var b []byte
	if b, err = io.ReadAll(reader); err != nil {
		return
	}

	err = json.Unmarshal(b, &manifests)

	return
}
