package diz

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"strings"

	"github.com/JohanLindvall/diz/dockerref"
	"github.com/JohanLindvall/diz/hashzip"
	"github.com/JohanLindvall/diz/str"
	"github.com/JohanLindvall/diz/util"

	"github.com/ryanuber/go-glob"
)

const (
	dizPrefix         = ".diz/"
	manifestJSON      = "manifest.json"
	repos             = "repositories"
	layersTarSuffix   = "/layer.tar"
	latestTag         = "latest"
	dotJSON           = ".json"
	manifestMediaType = "application/vnd.docker.distribution.manifest.v2+json"
	configMediaType   = "application/vnd.docker.container.image.v1+json"
	layerMediaType    = "application/vnd.docker.image.rootfs.diff.tar"
)

// Manifest defines a Docker image manifest
type Manifest struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

// Archive holds the data for reading a diz zip archive
type Archive struct {
	Manifests []Manifest
	reader    *hashzip.Reader
}

type repositories map[string]map[string]string

// NewArchive creates a new archive from the reader
func NewArchive(reader io.ReaderAt, size int64) (*Archive, error) {
	zipReader, err := hashzip.NewReader(reader, size)
	if err != nil {
		return nil, err
	}

	manifests, err := readManifest(zipReader)
	if err != nil {
		return nil, err
	}

	return &Archive{Manifests: manifests, reader: zipReader}, nil
}

// GetUncompressedSize returns the uncompressed size of the file with the given name
func (a *Archive) GetUncompressedSize(name string) int64 {
	if f := getDizFile(a.reader, name); f != nil {
		return int64(f.UncompressedSize64)
	}
	return -1
}

func getDizFile(zipReader *hashzip.Reader, name string) *hashzip.File {
	name = dizPrefix + name
	for _, file := range zipReader.File {
		if file.Name == name {
			return file
		}
	}

	return nil
}

func readManifest(zipReader *hashzip.Reader) (manifests []Manifest, err error) {
	f := getDizFile(zipReader, manifestJSON)
	if f == nil {
		return
	}

	var reader io.ReadCloser
	if reader, err = f.Open(); err != nil {
		return
	}

	defer reader.Close()

	var bytes []byte
	if bytes, err = ioutil.ReadAll(reader); err != nil {
		return
	}

	err = json.Unmarshal(bytes, &manifests)

	return
}

type fileHandler func(zf *hashzip.File, fn string, contents []byte) error

func (a *Archive) copyTo(handler fileHandler, manifests []Manifest, includeForeign, includeManifests bool) (err error) {
	include := make(map[string]bool, 0)

	for _, m := range manifests {
		include[m.Config] = true
		for _, l := range m.Layers {
			include[strings.TrimSuffix(l, layersTarSuffix)] = true
		}
	}

	var additional map[string][]byte
	if includeManifests {
		if additional, err = createManifestRepositories(manifests); err != nil {
			return
		}
	}

	for _, f := range a.reader.File {
		included := false
		if strings.HasPrefix(f.Name, dizPrefix) {
			fn := f.Name[len(dizPrefix):]
			if fn != repos && fn != manifestJSON {
				dir := strings.SplitN(fn, "/", 2)[0]
				if _, ok := include[dir]; ok {
					included = true
				}
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

// CopyToZip copies the contents for the archive, selected by manifests, to the zip writer. The manifest itself is not included
func (a *Archive) CopyToZip(zipWriter *hashzip.Writer, manifests []Manifest) (err error) {
	return a.copyTo(func(zf *hashzip.File, fn string, contents []byte) (err error) {
		var writer io.Writer
		if zf == nil {
			if !zipWriter.Exists(fn) {
				if writer, err = zipWriter.Create(fn); err != nil {
					return
				}
				if _, err = io.Copy(writer, bytes.NewReader(contents)); err != nil {
					return
				}
			}
		} else {
			hdr := zf.FileHeader
			if !zipWriter.Exists(hdr.Name) {
				if writer, err = zipWriter.CreateHeader(&hdr); err != nil {
					return
				}
				err = copyZipFile(writer, zf)
			}
		}
		return
	}, manifests, true, false)
}

func copyZipFile(writer io.Writer, zf *hashzip.File) (err error) {
	var readCloser io.ReadCloser
	if readCloser, err = zf.Open(); err != nil {
		return
	}
	_, err = util.CopyAndClose(writer, readCloser)
	return
}

// CopyToTar copies the contents for the archive, selected by manifests, to the tar writer. The manifest is always included
func (a *Archive) CopyToTar(writer io.Writer, manifests []Manifest) (err error) {
	tarWriter := tar.NewWriter(writer)

	err = a.copyTo(func(zf *hashzip.File, fn string, contents []byte) (err error) {
		var size int64
		mode := int64(0644)
		if zf == nil {
			size = int64(len(contents))
		} else {
			size = int64(zf.UncompressedSize64)
			mode = int64(zf.FileHeader.Mode())
		}
		if err = tarWriter.WriteHeader(&tar.Header{Name: fn, Size: size, Typeflag: tar.TypeRegA, Mode: mode}); err != nil {
			return
		}
		if zf == nil {
			if _, err = io.Copy(tarWriter, bytes.NewReader(contents)); err != nil {
				return
			}
		} else {
			err = copyZipFile(tarWriter, zf)
		}

		return

	}, manifests, false, true)

	if er := tarWriter.Close(); err == nil {
		err = er
	}

	return
}

// CopyFromTar copies the contents of the tar archive to the zip writer. The manifest is not copied
func CopyFromTar(rdr io.Reader, zipWriter *hashzip.Writer) (manifests []Manifest, err error) {
	tarReader := tar.NewReader(rdr)

	for {
		var header *tar.Header
		header, err = tarReader.Next()
		if err == io.EOF {
			err = nil
			break
		}
		if header.Name == manifestJSON {
			var bytes []byte
			if bytes, err = ioutil.ReadAll(tarReader); err != nil {
				return
			}
			if err = json.Unmarshal(bytes, &manifests); err != nil {
				return
			}
		} else if header.Name != repos {
			var entry io.Writer
			fn := dizPrefix + header.Name
			if !zipWriter.Exists(fn) {
				if entry, err = zipWriter.Create(fn); err != nil {
					return
				}
				if header.Typeflag == tar.TypeReg {
					if _, err = io.Copy(entry, tarReader); err != nil {
						return
					}
				}
			}
		}
	}

	return
}

// WriteManifests writes the manifests to the zip writer.
func WriteManifests(manifests []Manifest, zipWriter *hashzip.Writer) error {
	if m, err := createManifestRepositories(manifests); err != nil {
		return err
	} else {
		for k, v := range m {
			if entry, err := zipWriter.Create(dizPrefix + k); err != nil {
				return err
			} else {
				if _, err := io.Copy(entry, bytes.NewReader(v)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// MergeManifests mergs the manifests m1 and m2 into a single slice of manifests
func MergeManifests(m1, m2 []Manifest) (result []Manifest) {
	for _, m := range append(append([]Manifest(nil), m1...), m2...) {
		handled := false
		for i, e := range result {
			if e.Config == m.Config {
				// add missing repo tags
				for _, rt := range m.RepoTags {
					if str.IndexOf(e.RepoTags, rt) == -1 {
						result[i].RepoTags = append(e.RepoTags, rt)
					}
				}
				handled = true
			}
		}

		if !handled {
			result = append(result, Manifest{Config: m.Config, Layers: append([]string(nil), m.Layers...), RepoTags: append([]string(nil), m.RepoTags...)})
		}
	}

	return
}

func createManifestRepositories(manifests []Manifest) (result map[string][]byte, err error) {
	result = make(map[string][]byte, 0)
	var b []byte

	if b, err = json.Marshal(&manifests); err != nil {
		return
	}
	result[manifestJSON] = b
	r := make(repositories, 0)
	for _, m := range manifests {
		for _, t := range m.RepoTags {
			s := strings.SplitN(t, ":", 2)
			if len(s) == 1 {
				s = append(s, latestTag)
			}
			repo, ok := r[s[0]]
			if !ok {
				repo = make(map[string]string, 0)
				r[s[0]] = repo
			}
			repo[s[1]] = strings.TrimSuffix(m.Layers[len(m.Layers)-1], layersTarSuffix)
		}
	}
	if b, err = json.Marshal(&r); err != nil {
		return
	}
	result[repos] = b
	return
}

// FilterManifests returns a copy of the manifests filtered according to the tags glob
func FilterManifests(manifests []Manifest, tags []string) (result []Manifest) {
	for _, m := range manifests {
		mm := Manifest{Config: m.Config, Layers: m.Layers, RepoTags: FilterImageTags(m.RepoTags, tags)}
		if len(mm.RepoTags) > 0 {
			result = append(result, mm)
		}
	}

	return
}

// GetConfig returns the config from the manifest.
func GetConfig(m Manifest) string {
	return strings.TrimSuffix(m.Config, dotJSON)
}

// GetConfigs returns the configs from the manifests.
func GetConfigs(manifests []Manifest) (result []string) {
	for _, m := range manifests {
		result = append(result, GetConfig(m))
	}

	return
}

// FilterImageTags filters the given image tags according to the glob expressions. A '-' prepending the glob expression means a negative match
func FilterImageTags(imageTags, globTags []string) (result []string) {
	for _, repoTag := range imageTags {
		if repoTag != "<none>:<none>" {
			first := true
			match := false
			for _, globTag := range globTags {
				neg := false
				if strings.HasPrefix(globTag, "-") {
					globTag = globTag[1:]
					neg = true
					if first {
						match = true
					}
				}

				if glob.Glob(globTag, repoTag) {
					match = !neg
				}

				first = false
			}

			if match {
				result = append(result, repoTag)
			}
		}
	}
	return
}

// GetRegistryManifest gets the registry manifest for the given repo tag.
func (a *Archive) GetRegistryManifest(repoTag string) (RegistryManifest, error) {
	for _, m := range a.Manifests {
		for _, rt := range m.RepoTags {
			if dockerref.FamiliarReference(repoTag) == rt {
				result := RegistryManifest{}
				result.SchemaVersion = 2
				result.MediaType = manifestMediaType
				result.Config.MediaType = configMediaType
				result.Config.Size = a.GetUncompressedSize(m.Config)
				result.Config.Digest = "sha256:" + strings.TrimSuffix(m.Config, dotJSON)
				for _, l := range m.Layers {
					result.Layers = append(result.Layers, RegistryLayer{MediaType: layerMediaType, Size: a.GetUncompressedSize(l), Digest: "sha256:" + a.reader.GetHash(dizPrefix+l)})
				}
				return result, nil
			}
		}
	}

	return RegistryManifest{}, nil
}

// WriteFileByHash writes the file with the given content hash to the writer
func (a *Archive) WriteFileByHash(writer io.Writer, layerHash string) error {
	if f := a.reader.GetFileByHash(layerHash); f != nil {
		if rdr, err := f.Open(); rdr != nil && err == nil {
			_, err = util.CopyAndClose(writer, rdr)
			return err
		} else {
			return err
		}
	}

	return errors.New("not found")
}

// Read reads the path from the source zip and returns a zip archive if the path is a directory or single file if the path is a file
func (a *Archive) Read(path string) (read io.ReadCloser, err error) {
	if strings.HasSuffix(path, "/") {
		r, w := io.Pipe()
		go func() {
			zw := hashzip.NewWriter(w)
			var err error
			i := strings.LastIndex(path[:len(path)-1], "/") + 1
			for _, item := range a.reader.File {
				if strings.HasPrefix(item.Name, path) {
					hdr := item.FileHeader
					hdr.Name = item.FileHeader.Name[i:]
					var zfw io.Writer
					if zfw, err = zw.CreateHeader(&hdr); err != nil {
						break
					}
					var rdr io.ReadCloser
					if rdr, err = item.Open(); err != nil {
						break
					}
					if _, err = util.CopyAndClose(zfw, rdr); err != nil {
						break
					}
				}
			}
			er := zw.Close()
			if err == nil {
				err = er
			}
			w.CloseWithError(err)
		}()
		return r, nil
	} else if item := a.reader.GetFile(path); item != nil {
		return item.Open()
	}
	return nil, nil
}

// RegistryManifest defines the Docker image manifest from the registry's point of view
type RegistryManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Config        struct {
		MediaType string `json:"mediaType"`
		Size      int64  `json:"size"`
		Digest    string `json:"digest"`
	} `json:"config"`
	Layers []RegistryLayer `json:"layers"`
}

// RegistryLayer defines the Docker image layer from the registry's point of view
type RegistryLayer struct {
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Digest    string `json:"digest"`
}
