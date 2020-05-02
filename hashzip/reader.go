package hashzip

import (
	"io"
	"io/ioutil"

	"archive/zip"

	"encoding/json"
)

type Reader struct {
	reader                *zip.Reader
	fileHashes, hashFiles map[string]string
	File                  []*File
}

type File struct {
	file               *zip.File
	Name               string
	UncompressedSize64 uint64
	FileHeader         zip.FileHeader
}

// NewReader returns a new Reader reading from r, which is assumed to
// have the given size in bytes.
func NewReader(r io.ReaderAt, size int64) (*Reader, error) {
	rdr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	result := Reader{reader: rdr, fileHashes: make(map[string]string, 0), hashFiles: make(map[string]string, 0)}
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
			err = json.Unmarshal(b, &result.fileHashes)
			if err != nil {
				return nil, err
			}
			for k, v := range result.fileHashes {
				result.hashFiles[v] = k
			}
		} else {
			result.File = append(result.File, file)
		}
	}

	return &result, nil
}

func (r *Reader) GetFile(name string) *File {
	for _, f := range r.File {
		if f.Name == name {
			return f
		}
	}

	return nil
}

func (r *Reader) GetFileByHash(hash string) *File {
	if file := r.hashFiles[hash]; file != "" {
		return r.GetFile(file)
	} else {
		return nil
	}
}
func (f *File) Open() (io.ReadCloser, error) {
	return f.file.Open()
}

func (r *Reader) OpenFile(name string) (io.ReadCloser, error) {
	if f := r.GetFile(name); f != nil {
		return f.Open()
	} else {
		return nil, nil
	}
}

func (r *Reader) OpenFileByHash(hash string) (io.ReadCloser, error) {
	if f := r.GetFileByHash(hash); f != nil {
		return f.Open()
	} else {
		return nil, nil
	}
}

func (r *Reader) GetHash(name string) string {
	return r.fileHashes[name]
}
