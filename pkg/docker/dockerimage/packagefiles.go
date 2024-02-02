package dockerimage

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	log "github.com/sirupsen/logrus"
)

const (
	DirType        = "dir"
	FileType       = "file"
	SymlinkType    = "symlink"
	HardlinkType   = "hardlink"
	OtherType      = "other"
	UnknownType    = "unknown"
	UnexpectedType = "unexpected"
)

func ObjectTypeFromTarType(flag byte) string {
	switch flag {
	case tar.TypeDir:
		return DirType
	case tar.TypeReg, tar.TypeRegA:
		return FileType
	case tar.TypeSymlink:
		return SymlinkType
	case tar.TypeLink:
		return HardlinkType
	default:
		return fmt.Sprintf("other(%v)", flag)
	}
}

func TarHeaderTypeName(flag byte) string {
	switch flag {
	case tar.TypeReg:
		return "tar.TypeReg"
	case tar.TypeRegA:
		return "tar.TypeRegA"
	case tar.TypeLink:
		return "tar.TypeLink"
	case tar.TypeSymlink:
		return "tar.TypeSymlink"
	case tar.TypeChar:
		return "tar.TypeChar"
	case tar.TypeBlock:
		return "tar.TypeBlock"
	case tar.TypeDir:
		return "tar.TypeDir"
	case tar.TypeFifo:
		return "tar.TypeFifo"
	default:
		return fmt.Sprintf("other(%v)", flag)
	}
}

type PackageFiles struct {
	img       v1.Image
	imgDigest string
	imgID     string
}

type LayerMetadata struct {
	Index     int    `json:"index,omitempty"`
	Digest    string `json:"digest,omitempty"`
	DiffID    string `json:"diff_id,omitempty"`
	MediaType string `json:"media_type,omitempty"`
	Size      int64  `json:"size,omitempty"`
}

type FileSelectorType string

const (
	FSTAll        FileSelectorType = "fst.all"
	FSTDigest     FileSelectorType = "fst.digest"
	FSTDiffID     FileSelectorType = "fst.diffid"
	FSTIndex      FileSelectorType = "fst.index"
	FSTIndexRange FileSelectorType = "fst.index.range"
)

type FileSelector struct {
	Type              FileSelectorType
	Key               string
	IndexKey          int
	IndexEndKey       int
	ReverseIndexRange bool
	RawNames          bool
	NoDirs            bool
	Deleted           bool
}

type FileMetadata struct {
	Type       string      `json:"type,omitempty"`
	IsDir      bool        `json:"is_dir,omitempty"`
	IsDelete   bool        `json:"is_delete,omitempty"`
	IsOpq      bool        `json:"is_opq,omitempty"`
	Name       string      `json:"name,omitempty"`
	RawName    string      `json:"raw_name,omitempty"`
	Size       int64       `json:"size,omitempty"`
	Mode       os.FileMode `json:"mode,omitempty"`
	UID        int         `json:"uid"`
	GID        int         `json:"gid"`
	ModTime    time.Time   `json:"mod_time,omitempty"`
	ChangeTime time.Time   `json:"change_time,omitempty"`
}

type LayerFiles struct {
	Layer *LayerMetadata  `json:"layer"`
	Files []*FileMetadata `json:"files"`
}

func NewPackageFiles(archivePath string) (*PackageFiles, error) {
	img, err := tarball.ImageFromPath(archivePath, nil)
	if err != nil {
		return nil, err
	}

	digest, err := img.Digest()
	if err != nil {
		return nil, err
	}

	cfgName, err := img.ConfigName()
	if err != nil {
		return nil, err
	}

	ref := &PackageFiles{
		img:       img,
		imgDigest: digest.String(),
		imgID:     cfgName.String(),
	}

	return ref, nil
}

func (ref *PackageFiles) ImageDigest() string {
	return ref.imgDigest
}

func (ref *PackageFiles) ImageID() string {
	return ref.imgID
}

func (ref *PackageFiles) ListImageHistory() ([]XHistory, error) {
	configFile, err := ref.img.ConfigFile()
	if err != nil {
		return nil, err
	}

	var list []XHistory
	for _, record := range configFile.History {
		info := XHistory{
			Created:    record.Created.Time,
			CreatedBy:  record.CreatedBy,
			Comment:    record.Comment,
			EmptyLayer: record.EmptyLayer,
			Author:     record.Author,
		}

		list = append(list, info)
	}

	return list, nil
}

func (ref *PackageFiles) LayerCount() int {
	layers, _ := ref.img.Layers()
	return len(layers)
}

func (ref *PackageFiles) ListLayerMetadata() ([]*LayerMetadata, error) {
	layers, err := ref.img.Layers()
	if err != nil {
		return nil, err
	}

	var list []*LayerMetadata
	for idx, layer := range layers {
		digest, _ := layer.Digest()
		diffID, _ := layer.DiffID()
		size, _ := layer.Size()
		mediaType, _ := layer.MediaType()

		layerInfo := &LayerMetadata{
			Index:     idx,
			Digest:    digest.String(),
			DiffID:    diffID.String(),
			MediaType: string(mediaType),
			Size:      size,
		}

		list = append(list, layerInfo)
	}

	return list, nil
}

func (ref *PackageFiles) ListLayerFiles(selectors []FileSelector) ([]*LayerFiles, error) {
	//layersMetadata (by index)
	layersMetadata, err := ref.ListLayerMetadata()
	if err != nil {
		return nil, err
	}

	layers, err := ref.img.Layers()
	if err != nil {
		return nil, err
	}

	layersMetadataByDigest := map[string]*LayerMetadata{}
	layersMetadataByDiffID := map[string]*LayerMetadata{}
	for _, lmInfo := range layersMetadata {
		if lmInfo.Digest != "" {
			layersMetadataByDigest[lmInfo.Digest] = lmInfo
		}

		if lmInfo.DiffID != "" {
			layersMetadataByDiffID[lmInfo.DiffID] = lmInfo
		}
	}

	layerSelectors := map[string]FileSelector{}
	for _, selector := range selectors {
		switch selector.Type {
		case FSTAll:
			for k := range layersMetadataByDiffID {
				//todo: create a deep selector copy
				layerSelectors[k] = selector
			}
		case FSTDigest:
			if selector.Key != "" {
				if info, found := layersMetadataByDigest[selector.Key]; found {
					layerSelectors[info.DiffID] = selector
				}
			}
		case FSTDiffID:
			if selector.Key != "" {
				if info, found := layersMetadataByDiffID[selector.Key]; found {
					layerSelectors[info.DiffID] = selector
				}
			}
		case FSTIndex:
			if (selector.IndexKey >= 0) &&
				(selector.IndexKey < len(layersMetadata)) {
				layerSelectors[layersMetadata[selector.IndexKey].DiffID] = selector
			}
		case FSTIndexRange:
			if selector.ReverseIndexRange {
				if (selector.IndexKey >= 0) &&
					(selector.IndexKey < len(layersMetadata)) &&
					(selector.IndexEndKey >= 0) &&
					(selector.IndexEndKey < len(layersMetadata)) {
					if selector.IndexEndKey > selector.IndexKey {
						for i := selector.IndexKey; i <= selector.IndexEndKey; i++ {
							rindex := len(layersMetadata) - 1 - i
							if rindex >= 0 {
								layerSelectors[layersMetadata[rindex].DiffID] = selector
							}
						}
					}
				}
			} else {
				if (selector.IndexKey >= 0) &&
					(selector.IndexKey < len(layersMetadata)) {
					if selector.IndexEndKey < 0 {
						//means 'until the last index'
						selector.IndexEndKey = len(layersMetadata) - 1
					}

					if (selector.IndexEndKey >= 0) &&
						(selector.IndexEndKey < len(layersMetadata)) {
						if selector.IndexEndKey > selector.IndexKey {
							for i := selector.IndexKey; i <= selector.IndexEndKey; i++ {
								layerSelectors[layersMetadata[i].DiffID] = selector
							}
						}
					}
				}
			}
		}
	}

	layerFilesByDiff := map[string]*LayerFiles{}
	for _, layer := range layers {
		diffID, _ := layer.DiffID()
		diffIDStr := diffID.String()
		layerSelector, found := layerSelectors[diffIDStr]
		if !found {
			continue
		}

		layerInfo, found := layersMetadataByDiffID[diffIDStr]
		if !found {
			continue
		}

		currentLayerFiles := &LayerFiles{
			Layer: layerInfo,
		}

		ucr, err := layer.Uncompressed()
		if err != nil {
			return nil, err
		}

		defer ucr.Close()
		ltr := tar.NewReader(ucr)

		for {
			layerHeader, err := ltr.Next()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return nil, err
			}

			info := getFileMetadata(layerHeader)
			if info == nil {
				continue
			}

			if layerSelector.NoDirs && info.IsDir {
				continue
			}

			if !layerSelector.Deleted && (info.IsDelete || info.IsOpq) {
				continue
			}

			currentLayerFiles.Files = append(currentLayerFiles.Files, info)
		}

		layerFilesByDiff[diffIDStr] = currentLayerFiles
	}

	var layerFiles []*LayerFiles
	for _, v := range layerFilesByDiff {
		layerFiles = append(layerFiles, v)
	}

	return layerFiles, nil
}

func getFileMetadata(header *tar.Header) *FileMetadata {
	if header == nil || header.Name == "" {
		return nil
	}

	var isDir bool
	if header.Typeflag == tar.TypeDir {
		isDir = true
	}

	fileName := filepath.Clean(header.Name)
	if fileName != "" && fileName[0] != '/' {
		fileName = fmt.Sprintf("/%s", fileName)
	}

	rawFileName := fileName
	bname := filepath.Base(fileName)
	dname := filepath.Dir(fileName)
	isDelete := strings.HasPrefix(bname, whPrefix)
	var isOpq bool
	if isDelete {
		if bname == whOpaqueDir {
			isd := header.Typeflag == tar.TypeDir
			log.Debugf("dockerimage.getFileMetadata - special name: %s (header.Typeflag=%v/%s) / isd=%v", bname, header.Typeflag, TarHeaderTypeName(header.Typeflag), isd)
			isOpq = true
			isDelete = false
			fileName = dname
		} else {
			bname = bname[len(whPrefix):]

			if header.Typeflag != tar.TypeDir {
				fileName = filepath.Join(dname, bname)
			}
		}
	}

	result := &FileMetadata{
		Type:       ObjectTypeFromTarType(header.Typeflag),
		IsDir:      isDir,
		IsDelete:   isDelete,
		IsOpq:      isOpq,
		Name:       fileName,
		Size:       header.Size,
		Mode:       header.FileInfo().Mode(),
		UID:        header.Uid,
		GID:        header.Gid,
		ModTime:    header.ModTime,
		ChangeTime: header.ChangeTime,
	}

	if isDelete || isOpq {
		result.RawName = rawFileName
	}

	return result
}

// TODO: use already defined constants
const (
	whPrefix          = ".wh."
	whOpaqueDirSuffix = ".wh..opq"
	whOpaqueDir       = whPrefix + whOpaqueDirSuffix
)

//FileDataFromTar
//FileReaderFromTar

func IsLayerMediaType(value types.MediaType) bool {
	switch value {
	case types.DockerLayer,
		types.DockerUncompressedLayer,
		types.OCILayer,
		types.OCIUncompressedLayer,
		types.OCILayerZStd:
		return true
	}

	return false
}

/*

NOTE:
TAR FILE LIST INCLUDES EXTRAS
LIKE INTERMEDIATE DIRECTORIES,
WHITEOUT FILES,
DIRECTORY WHITEOUT FILES
NEED TO CLEAN UP THOSE THINGS WHEN GENERATING THE FILE LIST
ALSO HAVE A MODE TO INCLUDE/EXCLUDE INTERMEDIATE DIRECTORIES


INPUT: IMAGE TAR FILE PATH
OPTIONS/FLAGS (WHAT I WANT? / CAPABILITIES:
* List files for the last X layers (note: this is a "range" call) - MVP (should i retain any extra meta data for layers, etc?)
* List files for one specific layer (how to select? options: index, digest, fsdiffid)
* List files for a set of layers
* List all files - MAYBE
 (return a map of layers with lists/maps of files in each???) OR
 (return a global map of files with layer as a property???) OR
 (both or configurable to have either of these three options???)

*/
