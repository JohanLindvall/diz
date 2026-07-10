package diz

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/JohanLindvall/diz/hashzip"
	"github.com/JohanLindvall/diz/util"
)

// fileHandler receives each selected archive entry during a copyTo walk. Exactly
// one of zf (an existing zip entry) or contents (synthesized bytes) is set.
type fileHandler func(zf *hashzip.File, name string, contents []byte) error

// selectedFiles returns the set of top-level entry names referenced by the
// given manifests (their configs and layer directories).
func selectedFiles(manifests []Manifest) map[string]bool {
	include := make(map[string]bool)
	for _, m := range manifests {
		include[m.Config] = true
		for _, l := range m.Layers {
			include[strings.TrimSuffix(l, layersTarSuffix)] = true
		}
	}
	return include
}

// copyTo walks the archive and invokes handler for every entry selected by the
// manifests. When includeForeign is true, non-diz entries are included and diz
// entries keep their prefix. When includeManifests is true, freshly rendered
// manifest.json/repositories files are appended.
func (a *Archive) copyTo(handler fileHandler, manifests []Manifest, includeForeign, includeManifests bool) (err error) {
	include := selectedFiles(manifests)

	var additional map[string][]byte
	if includeManifests {
		if additional, err = buildRepositoryFiles(manifests); err != nil {
			return
		}
	}

	for _, f := range a.reader.File {
		included := false
		if strings.HasPrefix(f.Name, dizPrefix) {
			fn := f.Name[len(dizPrefix):]
			if fn != repos && fn != manifestJSON {
				dir := strings.SplitN(fn, "/", 2)[0]
				included = include[dir]
			}
		} else {
			included = includeForeign
		}

		if included {
			nm := f.Name
			if !includeForeign {
				nm = strings.TrimPrefix(nm, dizPrefix)
			}
			if err = handler(f, nm, nil); err != nil {
				return
			}
		}
	}

	for k, v := range additional {
		if includeForeign {
			k = dizPrefix + k
		}
		if err = handler(nil, k, v); err != nil {
			return
		}
	}

	return
}

// CopyToZip copies the contents selected by manifests to the zip writer. The
// manifest itself is not included.
func (a *Archive) CopyToZip(zipWriter *hashzip.Writer, manifests []Manifest) error {
	return a.copyTo(func(zf *hashzip.File, name string, contents []byte) error {
		if zf == nil {
			if zipWriter.Exists(name) {
				return nil
			}
			writer, err := zipWriter.Create(name)
			if err != nil {
				return err
			}
			_, err = io.Copy(writer, bytes.NewReader(contents))
			return err
		}
		if zipWriter.Exists(zf.Name) {
			return nil
		}
		return zipWriter.Copy(zf.Name, zf)
	}, manifests, true, false)
}

// CopyToTar copies the contents selected by manifests to the tar writer. The
// manifest is always included.
func (a *Archive) CopyToTar(writer io.Writer, manifests []Manifest) (err error) {
	tarWriter := tar.NewWriter(writer)

	err = a.copyTo(func(zf *hashzip.File, name string, contents []byte) error {
		size := int64(len(contents))
		mode := int64(0644)
		if zf != nil {
			size = int64(zf.UncompressedSize64)
			mode = int64(zf.FileHeader.Mode())
		}
		if err := tarWriter.WriteHeader(&tar.Header{Name: name, Size: size, Typeflag: tar.TypeRegA, Mode: mode}); err != nil {
			return err
		}
		if zf == nil {
			_, err := io.Copy(tarWriter, bytes.NewReader(contents))
			return err
		}
		return copyZipFile(tarWriter, zf)
	}, manifests, false, true)

	if er := tarWriter.Close(); err == nil {
		err = er
	}

	return
}

// CopyFromTar copies the contents of the tar archive to the zip writer,
// returning the manifests read from it. The manifest itself is not copied.
func CopyFromTar(rdr io.Reader, zipWriter *hashzip.Writer) (manifests []Manifest, err error) {
	tarReader := tar.NewReader(rdr)

	for {
		var header *tar.Header
		header, err = tarReader.Next()
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			return
		}

		switch header.Name {
		case manifestJSON:
			var b []byte
			if b, err = io.ReadAll(tarReader); err != nil {
				return
			}
			if err = json.Unmarshal(b, &manifests); err != nil {
				return
			}
		case repos:
			// legacy repositories file is regenerated on write; skip it
		default:
			if err = copyTarEntry(zipWriter, header, tarReader); err != nil {
				return
			}
		}
	}

	return
}

// copyTarEntry writes a single tar entry into the zip writer under the diz
// prefix, unless it already exists.
func copyTarEntry(zipWriter *hashzip.Writer, header *tar.Header, tarReader io.Reader) error {
	fn := dizPrefix + header.Name
	if zipWriter.Exists(fn) {
		return nil
	}
	entry, err := zipWriter.Create(fn)
	if err != nil {
		return err
	}
	if header.Typeflag == tar.TypeReg {
		if _, err = io.Copy(entry, tarReader); err != nil {
			return err
		}
	}
	return nil
}

// WriteManifests writes the manifest.json and repositories files for the given
// manifests to the zip writer.
func WriteManifests(manifests []Manifest, zipWriter *hashzip.Writer) error {
	files, err := buildRepositoryFiles(manifests)
	if err != nil {
		return err
	}
	for k, v := range files {
		entry, err := zipWriter.Create(dizPrefix + k)
		if err != nil {
			return err
		}
		if _, err := io.Copy(entry, bytes.NewReader(v)); err != nil {
			return err
		}
	}
	return nil
}

// copyZipFile copies the decompressed contents of the zip entry to the writer.
func copyZipFile(writer io.Writer, zf *hashzip.File) error {
	readCloser, err := zf.Open()
	if err != nil {
		return err
	}
	_, err = util.CopyAndClose(writer, readCloser)
	return err
}
