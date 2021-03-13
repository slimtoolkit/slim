package dockerimage

import (
	"archive/tar"
	"bytes"
	"container/heap"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v3"
	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/docker/dockerutil"
	"github.com/docker-slim/docker-slim/pkg/system"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
)

const (
	manifestFileName = "manifest.json"
	layerSuffix      = "/layer.tar"
	topObjectMax     = 10
)

type Package struct {
	Manifest    *ManifestObject
	Config      *ConfigObject
	Layers      []*Layer
	LayerIDRefs map[string]*Layer
}

type LayerReport struct {
	ID                  string                `json:"id"`
	Index               int                   `json:"index"`
	Path                string                `json:"path,omitempty"`
	LayerDataSource     string                `json:"layer_data_source,omitempty"`
	MetadataChangesOnly bool                  `json:"metadata_changes_only,omitempty"`
	FSDiffID            string                `json:"fsdiff_id,omitempty"`
	Stats               LayerStats            `json:"stats"`
	Changes             ChangesetSummary      `json:"changes"`
	Top                 []*ObjectMetadata     `json:"top"`
	Deleted             []*ObjectMetadata     `json:"deleted,omitempty"`
	Added               []*ObjectMetadata     `json:"added,omitempty"`
	Modified            []*ObjectMetadata     `json:"modified,omitempty"`
	ChangeInstruction   *InstructionSummary   `json:"change_instruction,omitempty"`
	OtherInstructions   []*InstructionSummary `json:"other_instructions,omitempty"`
}

type InstructionSummary struct {
	Index      int    `json:"index"`
	ImageIndex int    `json:"image_index"`
	Type       string `json:"type"`
	All        string `json:"all"`
	Snippet    string `json:"snippet"`
}

type ChangesetSummary struct {
	Deleted  uint64 `json:"deleted"`
	Added    uint64 `json:"added"`
	Modified uint64 `json:"modified"`
}

type Layer struct {
	ID                  string
	Index               int
	Path                string
	LayerDataSource     string
	MetadataChangesOnly bool
	FSDiffID            string
	Stats               LayerStats
	Changes             Changeset
	Objects             []*ObjectMetadata
	References          map[string]*ObjectMetadata
	Top                 TopObjects
	Distro              *system.DistroInfo
	DataMatches         map[string][]*ChangeDataMatcher //object.Name -> matched CDM
}

type LayerStats struct {
	//BlobSize         uint64 `json:"blob_size"`
	AllSize          uint64 `json:"all_size"`
	ObjectCount      uint64 `json:"object_count"`
	DirCount         uint64 `json:"dir_count"`
	FileCount        uint64 `json:"file_count"`
	LinkCount        uint64 `json:"link_count"`
	MaxFileSize      uint64 `json:"max_file_size"`
	MaxDirSize       uint64 `json:"max_dir_size"`
	DeletedCount     uint64 `json:"deleted_count"`
	DeletedDirCount  uint64 `json:"deleted_dir_count"`
	DeletedFileCount uint64 `json:"deleted_file_count"`
	DeletedLinkCount uint64 `json:"deleted_link_count"`
	DeletedSize      uint64 `json:"deleted_size"`
	AddedSize        uint64 `json:"added_size"`
	ModifiedSize     uint64 `json:"modified_size"`
}

type ChangeType int

const (
	ChangeUnknown ChangeType = iota
	ChangeDelete
	ChangeAdd
	ChangeModify
)

var changeTypeToStrings = map[ChangeType]string{
	ChangeUnknown: "U",
	ChangeDelete:  "D",
	ChangeAdd:     "A",
	ChangeModify:  "M",
}

var changeTypeFromStrings = map[string]ChangeType{
	"U": ChangeUnknown,
	"D": ChangeDelete,
	"A": ChangeAdd,
	"M": ChangeModify,
}

const CDMDumpToConsole = "console"

type ChangeDataMatcher struct {
	Dump        bool
	DumpConsole bool
	DumpDir     string
	PathPattern string
	DataPattern string
	Matcher     *regexp.Regexp
}

func (ct ChangeType) String() string {
	if v, ok := changeTypeToStrings[ct]; ok {
		return v
	}

	return "U"
}

func (ct ChangeType) MarshalJSON() ([]byte, error) {
	v, ok := changeTypeToStrings[ct]
	if !ok {
		v = "O"
	}

	d := bytes.NewBufferString(`"`)
	d.WriteString(v)
	d.WriteString(`"`)

	return d.Bytes(), nil
}

func (ct *ChangeType) UnmarshalJSON(b []byte) error {
	var d string
	err := json.Unmarshal(b, &d)
	if err != nil {
		return err
	}

	if v, ok := changeTypeFromStrings[d]; ok {
		*ct = v
	} else {
		*ct = ChangeUnknown
	}

	return nil
}

type ObjectMetadata struct {
	Change     ChangeType     `json:"change,omitempty"`
	Name       string         `json:"name,omitempty"`
	Size       int64          `json:"size,omitempty"`
	SizeHuman  string         `json:"size_human,omitempty"`
	Mode       os.FileMode    `json:"mode,omitempty"`
	ModeHuman  string         `json:"mode_human,omitempty"`
	UID        int            `json:"uid,omitempty"`
	GID        int            `json:"gid,omitempty"`
	ModTime    time.Time      `json:"mod_time,omitempty"`
	ChangeTime time.Time      `json:"change_time,omitempty"`
	LinkTarget string         `json:"link_target,omitempty"`
	History    *ObjectHistory `json:"history,omitempty"`
}

type ObjectHistory struct {
	Add      *ChangeInfo   `json:"A,omitempty"`
	Modifies []*ChangeInfo `json:"M,omitempty"`
	Delete   *ChangeInfo   `json:"D,omitempty"`
}

type ChangeInfo struct {
	Layer  int             `json:"layer"`
	Object *ObjectMetadata `json:"-"`
}

type Changeset struct {
	Deleted  []int
	Added    []int
	Modified []int
}

func newPackage() *Package {
	pkg := Package{
		LayerIDRefs: map[string]*Layer{},
	}

	return &pkg
}

func newLayer(id string) *Layer {
	layer := Layer{
		ID:          id,
		Index:       -1,
		References:  map[string]*ObjectMetadata{},
		Top:         NewTopObjects(topObjectMax),
		DataMatches: map[string][]*ChangeDataMatcher{},
	}

	heap.Init(&(layer.Top))

	return &layer
}

func LoadPackage(archivePath, imageID string, skipObjects bool, changeDataMatchers map[string]*ChangeDataMatcher) (*Package, error) {
	imageID = dockerutil.CleanImageID(imageID)

	configObjectFileName := fmt.Sprintf("%s.json", imageID)
	afile, err := os.Open(archivePath)
	if err != nil {
		log.Errorf("dockerimage.LoadPackage: os.Open error - %v", err)
		return nil, err
	}

	defer afile.Close()

	pkg := newPackage()
	layers := map[string]*Layer{}

	tr := tar.NewReader(afile)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Errorf("dockerimage.LoadPackage: error reading archive(%v) - %v", archivePath, err)
			return nil, err
		}

		if hdr == nil || hdr.Name == "" {
			log.Debugf("dockerimage.LoadPackage: ignoring bad tar header")
			continue
		}

		hdr.Name = filepath.Clean(hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeReg, tar.TypeSymlink:
			switch {
			case hdr.Name == manifestFileName:
				var manifests []ManifestObject
				if err := jsonFromStream(tr, &manifests); err != nil {
					log.Errorf("dockerimage.LoadPackage: error reading manifest file from archive(%v/%v) - %v", archivePath, manifestFileName, err)
					return nil, err
				}

				if len(manifests) == 0 {
					return nil, fmt.Errorf("dockerimage.LoadPackage: malformed manifest file - no manifests")
				}

				for _, m := range manifests {
					if m.Config == configObjectFileName {
						manifest := m
						pkg.Manifest = &manifest
						break
					}
				}

			case hdr.Name == configObjectFileName:
				var imageConfig ConfigObject
				if err := jsonFromStream(tr, &imageConfig); err != nil {
					log.Errorf("dockerimage.LoadPackage: error reading config object from archive(%v/%v) - %v", archivePath, configObjectFileName, err)
					return nil, err
				}

				pkg.Config = &imageConfig
			case strings.HasSuffix(hdr.Name, layerSuffix):
				parts := strings.Split(hdr.Name, "/")
				layerID := parts[0]

				var layer *Layer
				if hdr.Typeflag == tar.TypeSymlink {
					layer = newLayer(layerID)
					layer.Path = hdr.Name
					layer.MetadataChangesOnly = true

					parts := strings.Split(hdr.Linkname, "/")
					if len(parts) == 3 && parts[2] == "layer.tar" {
						layer.LayerDataSource = parts[1]

						if srcLayer, ok := layers[layer.LayerDataSource]; ok {
							for _, srcObj := range srcLayer.Objects {
								if srcObj.Change != ChangeDelete {
									newObj := *srcObj
									newObj.Change = ChangeUnknown
									layer.Objects = append(layer.Objects, &newObj)
									layer.References[srcObj.Name] = &newObj
									layer.Stats.ObjectCount++
								}
							}

							layer.Stats.LinkCount = srcLayer.Stats.LinkCount - srcLayer.Stats.DeletedLinkCount
							layer.Stats.FileCount = srcLayer.Stats.FileCount - srcLayer.Stats.DeletedFileCount
							layer.Stats.DirCount = srcLayer.Stats.DirCount - srcLayer.Stats.DeletedDirCount
						} else {
							log.Debugf("dockerimage.LoadPackage: could not find source layer - %v", layer.LayerDataSource)
						}
					}
				} else {
					layer, err = layerFromStream(hdr.Name, tar.NewReader(tr), layerID, changeDataMatchers)
					if err != nil {
						log.Errorf("dockerimage.LoadPackage: error reading layer from archive(%v/%v) - %v", archivePath, hdr.Name, err)
						return nil, err
					}
				}

				layers[layerID] = layer
			}
		}
	}

	if pkg.Manifest == nil {
		return nil, fmt.Errorf("dockerimage.LoadPackage: missing manifest object for image ID - %v", imageID)
	}

	if pkg.Config == nil {
		return nil, fmt.Errorf("dockerimage.LoadPackage: missing image config object for image ID - %v", imageID)
	}

	for idx, layerPath := range pkg.Manifest.Layers {
		parts := strings.Split(layerPath, "/")
		layerID := parts[0]
		layer, ok := layers[layerID]
		if !ok {
			log.Errorf("dockerimage.LoadPackage: error missing layer in archive(%v/%v) - %v", archivePath, layerPath, err)
			return nil, fmt.Errorf("dockerimage.LoadPackage: missing layer (%v) for image ID - %v", layerPath, imageID)
		}

		layer.Index = idx
		//adding layers based on their manifest order
		pkg.Layers = append(pkg.Layers, layer)
		if len(pkg.Layers)-1 != layer.Index {
			return nil, fmt.Errorf("dockerimage.LoadPackage: layer index mismatch - %v / %v", len(pkg.Layers)-1, layer.Index)
		}

		if layerPath != layer.Path {
			return nil, fmt.Errorf("dockerimage.LoadPackage: layer path mismatch - %v / %v", layerPath, layer.Path)
		}

		if idx == 0 {
			for oidx, object := range layer.Objects {
				object.History = &ObjectHistory{}

				if object.Change == ChangeUnknown {
					object.Change = ChangeAdd
					layer.Changes.Added = append(layer.Changes.Added, oidx)
					layer.Stats.AddedSize += uint64(object.Size)
				}

				changeInfo := ChangeInfo{
					Layer:  layer.Index,
					Object: object,
				}

				switch object.Change {
				case ChangeAdd:
					object.History.Add = &changeInfo
				case ChangeDelete:
					object.History.Delete = &changeInfo
				}
			}
		} else {
			for oidx, object := range layer.Objects {
				if object.Change == ChangeUnknown {
					for prevIdx := 0; prevIdx < idx; prevIdx++ {
						prevLayer := pkg.Layers[prevIdx]
						if om, ok := prevLayer.References[object.Name]; ok {
							object.Change = ChangeModify
							layer.Changes.Modified = append(layer.Changes.Modified, oidx)
							layer.Stats.ModifiedSize += uint64(object.Size)

							changeInfo := ChangeInfo{
								Layer:  layer.Index,
								Object: object,
							}

							if om.History != nil {
								object.History = om.History
							} else {
								object.History = &ObjectHistory{}
								om.History = object.History
							}

							object.History.Modifies = append(object.History.Modifies, &changeInfo)
							break
						}
					}

					if object.Change == ChangeUnknown {
						object.History = &ObjectHistory{
							Add: &ChangeInfo{
								Layer:  layer.Index,
								Object: object,
							},
						}

						object.Change = ChangeAdd
						layer.Changes.Added = append(layer.Changes.Added, oidx)
						layer.Stats.AddedSize += uint64(object.Size)
					}
				}
			}
		}

		pkg.LayerIDRefs[layerID] = layer

		if pkg.Config.RootFS != nil && idx < len(pkg.Config.RootFS.DiffIDs) {
			diffID := pkg.Config.RootFS.DiffIDs[idx]
			layer.FSDiffID = diffID
		} else {
			log.Debugf("dockerimage.LoadPackage: no FS diff for layer index %v", idx)
		}
	}

	if len(pkg.Layers) > 0 {
		var currentLayerIndex int
		for hidx := range pkg.Config.History {
			var currentLayer *Layer
			switch pkg.Config.History[hidx].EmptyLayer {
			case false:
				currentLayer = pkg.Layers[currentLayerIndex]

				if currentLayerIndex < (len(pkg.Layers) - 1) {
					currentLayerIndex++
				}
			case true:
				if currentLayerIndex == 0 {
					//no previous layer to reference (use the first layer)...
					currentLayer = pkg.Layers[currentLayerIndex]
				} else {
					//for empty layers add instructions to the previous layer
					currentLayer = pkg.Layers[currentLayerIndex-1]
				}
			}

			if currentLayer != nil {
				pkg.Config.History[hidx].LayerFSDiffID = currentLayer.FSDiffID
				pkg.Config.History[hidx].LayerID = currentLayer.ID
				pkg.Config.History[hidx].LayerIndex = currentLayer.Index
			}
		}
	}

	return pkg, nil
}

func layerFromStream(layerPath string, tr *tar.Reader, layerID string, changeDataMatchers map[string]*ChangeDataMatcher) (*Layer, error) {
	layer := newLayer(layerID)
	layer.Path = layerPath

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Errorf("layerFromStream: error reading layer(%v) - %v", layerID, err)
			return nil, err
		}

		if hdr == nil || hdr.Name == "" {
			log.Debug("layerFromStream: ignoring bad tar header")
			continue
		}

		hdr.Name = filepath.Clean(hdr.Name)

		object := &ObjectMetadata{
			Name:       hdr.Name,
			Size:       hdr.Size,
			Mode:       hdr.FileInfo().Mode(),
			UID:        hdr.Uid,
			GID:        hdr.Gid,
			ModTime:    hdr.ModTime,
			ChangeTime: hdr.ChangeTime,
		}

		layer.Objects = append(layer.Objects, object)
		layer.References[object.Name] = object

		heap.Push(&(layer.Top), object)
		if layer.Top.Len() > topObjectMax {
			_ = heap.Pop(&(layer.Top))
		}

		layer.Stats.AllSize += uint64(object.Size)
		layer.Stats.ObjectCount++

		normalized, isDeleted, err := NormalizeFileObjectLayerPath(object.Name)
		if err == nil {
			object.Name = fmt.Sprintf("/%s", normalized)
		}

		if isDeleted {
			object.Change = ChangeDelete
			idx := len(layer.Objects) - 1
			layer.Changes.Deleted = append(layer.Changes.Deleted, idx)
			layer.Stats.DeletedCount++
			layer.Stats.DeletedSize += uint64(object.Size)
			//NOTE:
			//This is not the real deleted size.
			//Need to find the actual object in a previous layer to know the actual size.
		}

		switch hdr.Typeflag {
		case tar.TypeSymlink, tar.TypeLink:
			object.LinkTarget = hdr.Linkname
			layer.Stats.LinkCount++
			if isDeleted {
				layer.Stats.DeletedLinkCount++
			}
		case tar.TypeReg:
			layer.Stats.FileCount++
			if uint64(object.Size) > layer.Stats.MaxFileSize {
				layer.Stats.MaxFileSize = uint64(object.Size)
			}

			if isDeleted {
				layer.Stats.DeletedFileCount++
			} else {
				err = inspectFile(object, tr, layer, changeDataMatchers)
				if err != nil {
					log.Errorf("layerFromStream: error inspecting layer file (%s) - (%v) - %v", object.Name, layerID, err)
				}
			}
		case tar.TypeDir:
			layer.Stats.DirCount++
			if uint64(object.Size) > layer.Stats.MaxDirSize {
				layer.Stats.MaxDirSize = uint64(object.Size)
			}

			if isDeleted {
				layer.Stats.DeletedDirCount++
			}
		}
	}

	return layer, nil
}

func inspectFile(object *ObjectMetadata, reader io.Reader, layer *Layer, changeDataMatchers map[string]*ChangeDataMatcher) error {
	//TODO: refactor and enhance the OS Distro detection logic
	fullPath := object.Name
	if system.IsOSReleaseFile(fullPath) || len(changeDataMatchers) > 0 {
		data, err := ioutil.ReadAll(reader)
		if err != nil {
			return err
		}

		if system.IsOSReleaseFile(fullPath) {
			osr, err := system.NewOsRelease(data)
			if err != nil {
				return err
			}

			distro := &system.DistroInfo{
				Name:        osr.Name,
				Version:     osr.VersionID,
				DisplayName: osr.PrettyName,
			}

			if distro.Version == "" {
				distro.Version = osr.Version
			}

			if distro.DisplayName == "" {
				nameMain := osr.Name
				nameVersion := osr.Version
				if nameVersion == "" {
					nameVersion = osr.VersionID
				}

				distro.DisplayName = fmt.Sprintf("%v %v", nameMain, nameVersion)
			}

			layer.Distro = distro
		}

		for _, cdm := range changeDataMatchers {
			if cdm.PathPattern != "" {
				pmatch, err := doublestar.Match(cdm.PathPattern, fullPath)
				if err != nil {
					log.Errorf("doublestar.Match name='%s' error=%v", fullPath, err)
					continue
				}

				if !pmatch {
					continue
				}
			}

			if cdm.Matcher.Match(data) {
				layer.DataMatches[fullPath] = append(layer.DataMatches[fullPath], cdm)

				if cdm.Dump {
					if cdm.DumpConsole {
						fmt.Printf("cmd=xray info=change.data.match.start\n")
						fmt.Printf("cmd=xray info=change.data.match file='%s' ppattern='%s' dpattern='%s')\n",
							fullPath, cdm.PathPattern, cdm.DataPattern)
						fmt.Printf("%s\n", string(data))
						fmt.Printf("cmd=xray info=change.data.match.end\n")
					}

					if cdm.DumpDir != "" {
						dumpPath := filepath.Join(cdm.DumpDir, fullPath)
						dirPath := fsutil.FileDir(dumpPath)
						if !fsutil.DirExists(dirPath) {
							err := os.MkdirAll(dirPath, 0755)
							if err != nil {
								fmt.Printf("cmd=xray info=change.data.match.dump.error file='%s' ppattern='%s' dpattern='%s' target='%s' error='%s'):\n",
									fullPath, cdm.PathPattern, cdm.DataPattern, dumpPath, err)
								continue
							}
						}

						err := ioutil.WriteFile(dumpPath, data, 0644)
						if err != nil {
							fmt.Printf("cmd=xray info=change.data.match.dump.error file='%s' ppattern='%s' dpattern='%s' target='%s' error='%s'):\n",
								fullPath, cdm.PathPattern, cdm.DataPattern, dumpPath, err)
							continue
						}

						fmt.Printf("cmd=xray info=change.data.match.dump file='%s' ppattern='%s' dpattern='%s' target='%s'):\n",
							fullPath, cdm.PathPattern, cdm.DataPattern, dumpPath)
					}
				}
			}
		}
	}

	return nil
}

func jsonFromStream(reader io.Reader, data interface{}) error {
	return json.NewDecoder(reader).Decode(data)
}

type TarReadCloser struct {
	io.Reader
	io.Closer
}

func FileReaderFromTar(tarPath, filePath string) (io.ReadCloser, error) {
	tfile, err := os.Open(tarPath)
	if err != nil {
		log.Errorf("dockerimage.FileReaderFromTar: os.Open error - %v", err)
		return nil, err
	}

	defer tfile.Close()
	tr := tar.NewReader(tfile)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		if hdr == nil || hdr.Name == "" {
			continue
		}

		hdr.Name = filepath.Clean(hdr.Name)
		if hdr.Name == filePath {
			switch hdr.Typeflag {
			case tar.TypeReg, tar.TypeSymlink, tar.TypeLink:
				return TarReadCloser{
					Reader: tr,
					Closer: tfile,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("no file - %s", filePath)
}

func FileDataFromTar(tarPath, filePath string) ([]byte, error) {
	tfile, err := os.Open(tarPath)
	if err != nil {
		log.Errorf("dockerimage.FileDataFromTar: os.Open error - %v", err)
		return nil, err
	}

	defer tfile.Close()
	tr := tar.NewReader(tfile)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		if hdr == nil || hdr.Name == "" {
			continue
		}

		hdr.Name = filepath.Clean(hdr.Name)
		if hdr.Name == filePath {
			switch hdr.Typeflag {
			case tar.TypeReg, tar.TypeSymlink, tar.TypeLink:
				return ioutil.ReadAll(tr)
			}
		}
	}

	return nil, fmt.Errorf("no file - %s", filePath)
}

func LoadManifestObject(archivePath, imageID string) (*ManifestObject, error) {
	return nil, nil
}

func LoadConfigObject(archivePath, imageID string) (*ConfigObject, error) {
	return nil, nil
}

func LoadLayer(archivePath, imageID, layerID string) (*Layer, error) {
	return nil, nil
}
