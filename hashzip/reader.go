package hashzip

import (
	"io"
	"io/ioutil"

	"github.com/klauspost/compress/zip"

	"encoding/json"
)

// Reader defines the zip reader
type Reader struct {
	reader *zip.Reader
	File   []*File
}

// File holds information about one file in a zip archive
type File struct {
	file               *zip.File
	Name               string
	UncompressedSize64 uint64
	FileHeader         zip.FileHeader
	Hash               string
}

// NewReader returns a new Reader reading from r, which is assumed to
// have the given size in bytes.
func NewReader(r io.ReaderAt, size int64) (*Reader, error) {
	rdr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	fileHashes := make(map[string]string, 0)

	result := Reader{reader: rdr}
	for _, f := range rdr.File {
		file := &File{file: f, Name: f.Name, UncompressedSize64: f.UncompressedSize64, FileHeader: f.FileHeader}
		if file.Name == ".hashes" {
			s, err := file.Open()
			if err != nil {
				return nil, err
			}
			b, err := ioutil.ReadAll(s)
			s.Close()
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal(b, &fileHashes)
			if err != nil {
				return nil, err
			}
		} else {
			result.File = append(result.File, file)
		}
	}

	for _, f := range result.File {
		f.Hash = fileHashes[f.Name]
	}

	return &result, nil
}

// GetFile gets the file by name
func (r *Reader) GetFile(name string) *File {
	for _, f := range r.File {
		if f.Name == name {
			return f
		}
	}

	return nil
}

// GetHash gets the hash for the file name
func (r *Reader) GetHash(name string) string {
	for _, f := range r.File {
		if f.Name == name {
			return f.Hash
		}
	}

	return ""
}

// GetFileByHash gets the file by the hash value
func (r *Reader) GetFileByHash(hash string) *File {
	for _, f := range r.File {
		if f.Hash == hash {
			return f
		}
	}

	return nil
}

// Open opens the file for reading
func (f *File) Open() (io.ReadCloser, error) {
	return f.file.Open()
}
