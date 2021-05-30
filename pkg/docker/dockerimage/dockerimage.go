package dockerimage

import (
	"archive/tar"
	"bytes"
	"container/heap"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bmatcuk/doublestar/v3"
	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/docker/dockerutil"
	"github.com/docker-slim/docker-slim/pkg/system"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
)

const (
	manifestFileName    = "manifest.json"
	layerSuffix         = "/layer.tar"
	defaultTopObjectMax = 10
)

type Package struct {
	Manifest       *ManifestObject
	Config         *ConfigObject
	Layers         []*Layer
	LayerIDRefs    map[string]*Layer
	HashReferences map[string]map[string]*ObjectMetadata
	Stats          PackageStats
}

type ImageReport struct {
	Stats      PackageStats                     `json:"stats"`
	Duplicates map[string]*DuplicateFilesReport `json:"duplicates,omitempty"`
}

type DuplicateFilesReport struct {
	FileCount   uint64         `json:"file_count"`
	FileSize    uint64         `json:"file_size"`
	AllFileSize uint64         `json:"all_file_size"`
	WastedSize  uint64         `json:"wasted_size"`
	Files       map[string]int `json:"files"`
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
	DataMatches         map[string][]*ChangeDataMatcher   //object.Name -> matched CDM
	DataHashMatches     map[string]*ChangeDataHashMatcher //object.Name -> matched CDHM
	pathMatches         bool
}

func (ref *Layer) HasMatches() bool {
	if len(ref.DataMatches) > 0 {
		return true
	}

	if len(ref.DataHashMatches) > 0 {
		return true
	}

	if ref.pathMatches {
		return true
	}

	return false
}

type LayerStats struct {
	//BlobSize         uint64 `json:"blob_size"`
	AllSize                uint64 `json:"all_size"`
	ObjectCount            uint64 `json:"object_count"`
	DirCount               uint64 `json:"dir_count"`
	FileCount              uint64 `json:"file_count"`
	LinkCount              uint64 `json:"link_count"`
	MaxFileSize            uint64 `json:"max_file_size"`
	MaxDirSize             uint64 `json:"max_dir_size"`
	DeletedCount           uint64 `json:"deleted_count"`
	DeletedDirContentCount uint64 `json:"deleted_dir_content_count"`
	DeletedDirCount        uint64 `json:"deleted_dir_count"`
	DeletedFileCount       uint64 `json:"deleted_file_count"`
	DeletedLinkCount       uint64 `json:"deleted_link_count"`
	DeletedSize            uint64 `json:"deleted_size"`
	AddedSize              uint64 `json:"added_size"`
	ModifiedSize           uint64 `json:"modified_size"`
	Utf8Count              uint64 `json:"utf8_count,omitempty"`
	Utf8Size               uint64 `json:"utf8_size,omitempty"`
	Utf8SizeHuman          string `json:"utf8_size_human,omitempty"`
	BinaryCount            uint64 `json:"binary_count,omitempty"`
	BinarySize             uint64 `json:"binary_size,omitempty"`
	BinarySizeHuman        string `json:"binary_size_human,omitempty"`
}

type PackageStats struct {
	DuplicateFileCount      uint64 `json:"duplicate_file_count"`
	DuplicateFileTotalCount uint64 `json:"duplicate_file_total_count"`
	DuplicateFileSize       uint64 `json:"duplicate_file_size"`
	DuplicateFileTotalSize  uint64 `json:"duplicate_file_total_size"`
	DuplicateFileWastedSize uint64 `json:"duplicate_file_wasted_size"`
	DeletedCount            uint64 `json:"deleted_count"`
	DeletedDirContentCount  uint64 `json:"deleted_dir_content_count"`
	DeletedDirCount         uint64 `json:"deleted_dir_count"`
	DeletedFileCount        uint64 `json:"deleted_file_count"`
	DeletedLinkCount        uint64 `json:"deleted_link_count"`
	DeletedFileSize         uint64 `json:"deleted_file_size"`
	Utf8Count               uint64 `json:"utf8_count,omitempty"`
	Utf8Size                uint64 `json:"utf8_size,omitempty"`
	Utf8SizeHuman           string `json:"utf8_size_human,omitempty"`
	BinaryCount             uint64 `json:"binary_count,omitempty"`
	BinarySize              uint64 `json:"binary_size,omitempty"`
	BinarySizeHuman         string `json:"binary_size_human,omitempty"`
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

type ChangeDataHashMatcher struct {
	Dump        bool
	DumpConsole bool
	DumpDir     string
	Hash        string //lowercase
}

type ChangePathMatcher struct {
	Dump        bool
	DumpConsole bool
	DumpDir     string
	PathPattern string
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

const (
	ContentTypeUTF8   = "utf8"
	ContentTypeBinary = "binary"
)

type ObjectMetadata struct {
	Change           ChangeType     `json:"change"`
	DirContentDelete bool           `json:"dir_content_delete,omitempty"`
	Name             string         `json:"name"`
	Size             int64          `json:"size,omitempty"`
	SizeHuman        string         `json:"size_human,omitempty"` //not used yet
	Mode             os.FileMode    `json:"mode,omitempty"`
	ModeHuman        string         `json:"mode_human,omitempty"`
	UID              int            `json:"uid"` //don't omit uid 0
	GID              int            `json:"gid"` //don't omit gid 0
	ModTime          time.Time      `json:"mod_time,omitempty"`
	ChangeTime       time.Time      `json:"change_time,omitempty"`
	LinkTarget       string         `json:"link_target,omitempty"`
	History          *ObjectHistory `json:"history,omitempty"`
	Hash             string         `json:"hash,omitempty"`
	PathMatch        bool           `json:"-"`
	LayerIndex       int            `json:"-"`
	TypeFlag         byte           `json:"-"`
	ContentType      string         `json:"content_type,omitempty"`
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
	Deleted           []int
	DeletedDirContent []int
	Added             []int
	Modified          []int
}

func newPackage() *Package {
	pkg := Package{
		LayerIDRefs:    map[string]*Layer{},
		HashReferences: map[string]map[string]*ObjectMetadata{},
	}

	return &pkg
}

func newLayer(id string, topChangesMax int) *Layer {
	topChangesCount := defaultTopObjectMax
	if topChangesMax > -1 {
		topChangesCount = topChangesMax
	}

	layer := Layer{
		ID:              id,
		Index:           -1,
		References:      map[string]*ObjectMetadata{},
		Top:             NewTopObjects(topChangesCount),
		DataMatches:     map[string][]*ChangeDataMatcher{},
		DataHashMatches: map[string]*ChangeDataHashMatcher{},
	}

	heap.Init(&(layer.Top))

	return &layer
}

func LoadPackage(archivePath string,
	imageID string,
	skipObjects bool,
	topChangesMax int,
	doHashData bool,
	doFindDuplicates bool,
	changeDataHashMatchers map[string]*ChangeDataHashMatcher,
	changePathMatchers []*ChangePathMatcher,
	changeDataMatchers map[string]*ChangeDataMatcher,
	tarUtf8 *tar.Writer,
) (*Package, error) {
	imageID = dockerutil.CleanImageID(imageID)

	cpmDumps := hasChangePathMatcherDumps(changePathMatchers)
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
					layer = newLayer(layerID, topChangesMax)
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
					layer, err = layerFromStream(
						pkg,
						hdr.Name,
						tar.NewReader(tr),
						layerID,
						topChangesMax,
						doHashData,
						doFindDuplicates,
						changeDataHashMatchers,
						changePathMatchers,
						cpmDumps,
						changeDataMatchers,
						tarUtf8,
					)
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
				object.LayerIndex = idx

				if tarUtf8 != nil {
					if object.ContentType == ContentTypeUTF8 {
						layer.Stats.Utf8Count++
						layer.Stats.Utf8Size += uint64(object.Size)
						pkg.Stats.Utf8Count++
						pkg.Stats.Utf8Size += uint64(object.Size)
					} else {
						layer.Stats.BinaryCount++
						layer.Stats.BinarySize += uint64(object.Size)
						pkg.Stats.BinaryCount++
						pkg.Stats.BinarySize += uint64(object.Size)
					}
				}

				if object.Change == ChangeUnknown {
					object.Change = ChangeAdd
					layer.Changes.Added = append(layer.Changes.Added, oidx)
					layer.Stats.AddedSize += uint64(object.Size)
				}

				if object.History == nil {
					object.History = &ObjectHistory{}
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

			if tarUtf8 != nil {
				layer.Stats.Utf8SizeHuman = humanize.Bytes(layer.Stats.Utf8Size)
				layer.Stats.BinarySizeHuman = humanize.Bytes(layer.Stats.BinarySize)
			}

		} else {
			for oidx, object := range layer.Objects {
				object.LayerIndex = idx

				if tarUtf8 != nil {
					if object.ContentType == ContentTypeUTF8 {
						layer.Stats.Utf8Count++
						layer.Stats.Utf8Size += uint64(object.Size)
						pkg.Stats.Utf8Count++
						pkg.Stats.Utf8Size += uint64(object.Size)
					} else {
						layer.Stats.BinaryCount++
						layer.Stats.BinarySize += uint64(object.Size)
						pkg.Stats.BinaryCount++
						pkg.Stats.BinarySize += uint64(object.Size)
					}
				}

				if object.Change == ChangeUnknown {
					for prevIdx := 0; prevIdx < idx; prevIdx++ {
						prevLayer := pkg.Layers[prevIdx]
						if om, ok := prevLayer.References[object.Name]; ok {
							object.Change = ChangeModify
							layer.Changes.Modified = append(layer.Changes.Modified, oidx)
							layer.Stats.ModifiedSize += uint64(object.Size)

							if om.History != nil {
								object.History = om.History
							} else {
								object.History = &ObjectHistory{}
								om.History = object.History
							}

							changeInfo := ChangeInfo{
								Layer:  layer.Index,
								Object: object,
							}

							object.History.Modifies = append(object.History.Modifies, &changeInfo)
							break
						}
					}

					if object.Change == ChangeUnknown {
						if object.History == nil {
							object.History = &ObjectHistory{}
						}

						object.History.Add = &ChangeInfo{
							Layer:  layer.Index,
							Object: object,
						}

						object.Change = ChangeAdd
						layer.Changes.Added = append(layer.Changes.Added, oidx)
						layer.Stats.AddedSize += uint64(object.Size)
					}
				}

				if object.Change == ChangeDelete {
					for prevIdx := 0; prevIdx < idx; prevIdx++ {
						prevLayer := pkg.Layers[prevIdx]
						if om, ok := prevLayer.References[object.Name]; ok {

							if om.History != nil {
								object.History = om.History
							} else {
								object.History = &ObjectHistory{}
								om.History = object.History
							}

							object.History.Delete = &ChangeInfo{
								Layer:  layer.Index,
								Object: object,
							}

							switch object.TypeFlag {
							case tar.TypeReg:
								//NOTE: counting the file size of the first instance
								layer.Stats.DeletedSize += uint64(om.Size)
								pkg.Stats.DeletedFileSize += uint64(om.Size)
							case tar.TypeDir:
								//TODO: need to count all file sizes in the dir
							}

							break
						}
					}
				}
			}
			if tarUtf8 != nil {
				layer.Stats.Utf8SizeHuman = humanize.Bytes(layer.Stats.Utf8Size)
				layer.Stats.BinarySizeHuman = humanize.Bytes(layer.Stats.BinarySize)
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

	if tarUtf8 != nil {
		pkg.Stats.Utf8SizeHuman = humanize.Bytes(pkg.Stats.Utf8Size)
		pkg.Stats.BinarySizeHuman = humanize.Bytes(pkg.Stats.BinarySize)
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

	if doFindDuplicates {
		for hash, hr := range pkg.HashReferences {
			dupCount := len(hr)
			if dupCount > 1 {
				pkg.Stats.DuplicateFileCount++
				pkg.Stats.DuplicateFileTotalCount += uint64(dupCount)
				for _, om := range hr {
					pkg.Stats.DuplicateFileSize += uint64(om.Size)
					pkg.Stats.DuplicateFileTotalSize += uint64(om.Size * int64(dupCount))
					break
				}
			} else {
				delete(pkg.HashReferences, hash)
			}
		}

		if pkg.Stats.DuplicateFileCount > 0 {
			pkg.Stats.DuplicateFileWastedSize = uint64(pkg.Stats.DuplicateFileTotalSize - pkg.Stats.DuplicateFileSize)
		}
	}

	return pkg, nil
}

func hasChangePathMatcherDumps(changePathMatchers []*ChangePathMatcher) bool {
	for _, cpm := range changePathMatchers {
		if cpm.PathPattern != "" && cpm.Dump {
			return true
		}
	}

	return false
}

func layerFromStream(
	pkg *Package,
	layerPath string,
	tr *tar.Reader,
	layerID string,
	topChangesMax int,
	doHashData bool,
	doFindDuplicates bool,
	changeDataHashMatchers map[string]*ChangeDataHashMatcher,
	changePathMatchers []*ChangePathMatcher,
	cpmDumps bool,
	changeDataMatchers map[string]*ChangeDataMatcher,
	tarUtf8 *tar.Writer,
) (*Layer, error) {

	layer := newLayer(layerID, topChangesMax)
	layer.Path = layerPath

	topChangesCount := defaultTopObjectMax
	if topChangesMax > -1 {
		topChangesCount = topChangesMax
	}

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
			Size:       hdr.Size, //todo: set .SizeHuman
			Mode:       hdr.FileInfo().Mode(),
			UID:        hdr.Uid,
			GID:        hdr.Gid,
			ModTime:    hdr.ModTime,
			ChangeTime: hdr.ChangeTime,
			TypeFlag:   hdr.Typeflag,
		}

		normalized, isDeleted, isDeletedDirContent, err := NormalizeFileObjectLayerPath(object.Name)
		if err == nil && len(object.Name) > 0 && object.Name[0] != '/' {
			object.Name = fmt.Sprintf("/%s", normalized)
		}

		object.Name = strings.ReplaceAll(object.Name, "//", "/")
		object.LinkTarget = strings.ReplaceAll(object.LinkTarget, "//", "/")

		if isDeletedDirContent {
			object.DirContentDelete = true
		}

		layer.Objects = append(layer.Objects, object)
		layer.References[object.Name] = object

		heap.Push(&(layer.Top), object)
		if layer.Top.Len() > topChangesCount {
			_ = heap.Pop(&(layer.Top))
		}

		layer.Stats.AllSize += uint64(object.Size)
		layer.Stats.ObjectCount++

		if isDeletedDirContent {
			object.Change = ChangeDelete
			idx := len(layer.Objects) - 1
			layer.Changes.Deleted = append(layer.Changes.Deleted, idx)
			layer.Changes.DeletedDirContent = append(layer.Changes.DeletedDirContent, idx)
			layer.Stats.DeletedDirContentCount++
			pkg.Stats.DeletedDirContentCount++
		} else {
			if isDeleted {
				object.Change = ChangeDelete
				idx := len(layer.Objects) - 1
				layer.Changes.Deleted = append(layer.Changes.Deleted, idx)
				layer.Stats.DeletedCount++
				pkg.Stats.DeletedCount++
				//layer.Stats.DeletedSize += uint64(object.Size)
				//NOTE:
				//This is not the real deleted size.
				//Need to find the actual object in a previous layer to know the actual size.
			}
		}

		switch hdr.Typeflag {
		case tar.TypeSymlink, tar.TypeLink:
			object.LinkTarget = hdr.Linkname
			layer.Stats.LinkCount++
			if isDeleted {
				layer.Stats.DeletedLinkCount++
				pkg.Stats.DeletedLinkCount++
			}
		case tar.TypeReg:
			layer.Stats.FileCount++
			if uint64(object.Size) > layer.Stats.MaxFileSize {
				layer.Stats.MaxFileSize = uint64(object.Size)
			}

			if isDeleted {
				layer.Stats.DeletedFileCount++
				pkg.Stats.DeletedFileCount++
			} else {
				err = inspectFile(
					object,
					tr,
					layer,
					doHashData,
					changeDataHashMatchers,
					changePathMatchers,
					cpmDumps,
					changeDataMatchers,
					tarUtf8,
				)
				if err != nil {
					log.Errorf("layerFromStream: error inspecting layer file (%s) - (%v) - %v", object.Name, layerID, err)
				} else {
					if doFindDuplicates && len(object.Hash) != 0 {
						hr, found := pkg.HashReferences[object.Hash]
						if !found {
							hr = map[string]*ObjectMetadata{}
							pkg.HashReferences[object.Hash] = hr
						}

						hr[object.Name] = object
					}
				}
			}
		case tar.TypeDir:
			layer.Stats.DirCount++
			if uint64(object.Size) > layer.Stats.MaxDirSize {
				layer.Stats.MaxDirSize = uint64(object.Size)
			}

			if isDeleted {
				layer.Stats.DeletedDirCount++
				pkg.Stats.DeletedDirCount++
			}
		}
	}

	return layer, nil
}

func getBytesHash(data []byte) string {
	hash := sha1.Sum(data)
	return hex.EncodeToString(hash[:])
}

type utf8FileInfo struct {
	name    string
	size    int64
	modtime time.Time
}

func (f *utf8FileInfo) Name() string       { return f.name }
func (f *utf8FileInfo) Size() int64        { return f.size }
func (f *utf8FileInfo) Mode() os.FileMode  { return os.ModePerm }
func (f *utf8FileInfo) ModTime() time.Time { return f.modtime }
func (f *utf8FileInfo) IsDir() bool        { return false }
func (f *utf8FileInfo) Sys() interface{}   { return nil }

func inspectFile(
	object *ObjectMetadata,
	reader io.Reader,
	layer *Layer,
	doHashData bool,
	changeDataHashMatchers map[string]*ChangeDataHashMatcher,
	changePathMatchers []*ChangePathMatcher,
	cpmDumps bool,
	changeDataMatchers map[string]*ChangeDataMatcher,
	tarUtf8 *tar.Writer,
) error {
	//TODO: refactor and enhance the OS Distro detection logic
	fullPath := object.Name
	if system.IsOSReleaseFile(fullPath) || len(changeDataMatchers) > 0 || cpmDumps {
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

		if doHashData || len(changeDataHashMatchers) > 0 {

			hash := getBytesHash(data)
			if doHashData {
				object.Hash = hash
			}
			if tarUtf8 != nil {
				if utf8.Valid(data) {
					object.ContentType = ContentTypeUTF8
					fileInfo := &utf8FileInfo{
						name:    hash,
						size:    object.Size,
						modtime: object.ModTime,
					}
					header, err := tar.FileInfoHeader(fileInfo, hash)
					if err != nil {
						return err
					}
					header.Name = hash
					err = tarUtf8.WriteHeader(header)
					if err != nil {
						return err
					}
					_, err = tarUtf8.Write(data)
					if err != nil {
						return err
					}
				} else {
					object.ContentType = ContentTypeBinary
				}
			}

			if len(changeDataHashMatchers) > 0 {
				if dhm, found := changeDataHashMatchers[hash]; found {
					//need to save to DataHashMatches to make it work without generating/saving hashes for all objects
					layer.DataHashMatches[fullPath] = dhm

					if dhm.DumpConsole {
						fmt.Printf("cmd=xray info=change.data.hash.match.start\n")
						fmt.Printf("cmd=xray info=change.data.hash.match file='%s' hash='%s')\n",
							fullPath, hash)
						fmt.Printf("%s\n", string(data))
						fmt.Printf("cmd=xray info=change.data.hash.match.end\n")
					}

					if dhm.DumpDir != "" {
						dumpPath := filepath.Join(dhm.DumpDir, fullPath)
						dirPath := fsutil.FileDir(dumpPath)
						if !fsutil.DirExists(dirPath) {
							err := os.MkdirAll(dirPath, 0755)
							if err != nil {
								fmt.Printf("cmd=xray info=change.data.hash.match.dump.error file='%s' hash='%s' target='%s' error='%s'):\n",
									fullPath, hash, dumpPath, err)
								return err
							}
						}

						err := ioutil.WriteFile(dumpPath, data, 0644)
						if err != nil {
							fmt.Printf("cmd=xray info=change.data.hash.match.dump.error file='%s' hash='%s' target='%s' error='%s'):\n",
								fullPath, hash, dumpPath, err)
							return err
						}

						fmt.Printf("cmd=xray info=change.data.hash.match.dump file='%s' hash='%s' target='%s'):\n",
							fullPath, hash, dumpPath)
					}
				}
			}
		}

		for _, cpm := range changePathMatchers {
			if cpm.PathPattern != "" {
				pmatch, err := doublestar.Match(cpm.PathPattern, fullPath)
				if err != nil {
					log.Errorf("doublestar.Match name='%s' error=%v", fullPath, err)
					continue
				}

				if !pmatch {
					continue
				}

				object.PathMatch = true
				layer.pathMatches = true

				if cpm.Dump && cpm.DumpConsole {
					fmt.Printf("cmd=xray info=change.path.match.start\n")
					fmt.Printf("cmd=xray info=change.path.match file='%s' ppattern='%s')\n",
						fullPath, cpm.PathPattern)
					fmt.Printf("%s\n", string(data))
					fmt.Printf("cmd=xray info=change.path.match.end\n")
				}

				if cpm.Dump && cpm.DumpDir != "" {
					dumpPath := filepath.Join(cpm.DumpDir, fullPath)
					dirPath := fsutil.FileDir(dumpPath)
					if !fsutil.DirExists(dirPath) {
						err := os.MkdirAll(dirPath, 0755)
						if err != nil {
							fmt.Printf("cmd=xray info=change.path.match.dump.error file='%s' ppattern='%s' target='%s' error='%s'):\n",
								fullPath, cpm.PathPattern, dumpPath, err)
							continue
						}
					}

					err := ioutil.WriteFile(dumpPath, data, 0644)
					if err != nil {
						fmt.Printf("cmd=xray info=change.path.match.dump.error file='%s' ppattern='%s' target='%s' error='%s'):\n",
							fullPath, cpm.PathPattern, dumpPath, err)
						continue
					}

					fmt.Printf("cmd=xray info=change.path.match.dump file='%s' ppattern='%s' target='%s'):\n",
						fullPath, cpm.PathPattern, dumpPath)
				}

				break
			}
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

				//TODO:
				//add a flag to do first match only
				//now we'll try to match all data patterns
			}
		}
	} else {
		for _, cpm := range changePathMatchers {
			if cpm.PathPattern != "" {
				pmatch, err := doublestar.Match(cpm.PathPattern, fullPath)
				if err != nil {
					log.Errorf("doublestar.Match name='%s' error=%v", fullPath, err)
					continue
				}

				if pmatch {
					object.PathMatch = true
					layer.pathMatches = true
					break
				}
			}
		}

		if doHashData || len(changeDataHashMatchers) > 0 {

			data, err := ioutil.ReadAll(reader)
			if err != nil {
				return err
			}

			hash := getBytesHash(data)

			object.Hash = hash
			if tarUtf8 != nil {
				if utf8.Valid(data) {
					object.ContentType = ContentTypeUTF8
					fileInfo := &utf8FileInfo{
						name:    hash,
						size:    object.Size,
						modtime: object.ModTime,
					}
					header, err := tar.FileInfoHeader(fileInfo, hash)
					if err != nil {
						return err
					}
					header.Name = hash
					err = tarUtf8.WriteHeader(header)
					if err != nil {
						return err
					}
					_, err = tarUtf8.Write(data)
					if err != nil {
						return err
					}
				} else {
					object.ContentType = ContentTypeBinary
				}
			}

			if dhm, found := changeDataHashMatchers[hash]; found {
				//need to save to DataHashMatches to make it work without generating/saving hashes for all objects
				layer.DataHashMatches[fullPath] = dhm

				if dhm.DumpConsole {
					fmt.Printf("cmd=xray info=change.data.hash.match.start\n")
					fmt.Printf("cmd=xray info=change.data.hash.match file='%s' hash='%s')\n",
						fullPath, hash)
					fmt.Printf("%s\n", string(data))
					fmt.Printf("cmd=xray info=change.data.hash.match.end\n")
				}

				if dhm.DumpDir != "" {
					dumpPath := filepath.Join(dhm.DumpDir, fullPath)
					dirPath := fsutil.FileDir(dumpPath)
					if !fsutil.DirExists(dirPath) {
						err := os.MkdirAll(dirPath, 0755)
						if err != nil {
							fmt.Printf("cmd=xray info=change.data.hash.match.dump.error file='%s' hash='%s' target='%s' error='%s'):\n",
								fullPath, hash, dumpPath, err)
							return err
						}
					}

					err := ioutil.WriteFile(dumpPath, data, 0644)
					if err != nil {
						fmt.Printf("cmd=xray info=change.data.hash.match.dump.error file='%s' hash='%s' target='%s' error='%s'):\n",
							fullPath, hash, dumpPath, err)
						return err
					}

					fmt.Printf("cmd=xray info=change.data.hash.match.dump file='%s' hash='%s' target='%s'):\n",
						fullPath, hash, dumpPath)
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
