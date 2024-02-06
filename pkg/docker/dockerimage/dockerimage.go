package dockerimage

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"container/heap"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bmatcuk/doublestar/v3"
	"github.com/dustin/go-humanize"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/certdiscover"
	"github.com/slimtoolkit/slim/pkg/docker/dockerutil"
	"github.com/slimtoolkit/slim/pkg/sysidentity"
	"github.com/slimtoolkit/slim/pkg/system"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
	"github.com/slimtoolkit/slim/pkg/util/jsonutil"
)

const (
	defaultTopObjectMax = 10
)

type ImageFormatType string

const (
	FormatUnknown   ImageFormatType = "unknown"
	FormatDockerV1                  = "docker.v1"
	FormatDockerOCI                 = "docker.oci"
	FormatOCI                       = "oci"
)

// todo: rename 'Package' struct
type Package struct {
	Format          ImageFormatType
	IsOCI           bool
	ManifestOCI     *oci.Manifest
	Manifest        *DockerManifestObject
	Config          *ConfigObject
	Layers          []*Layer
	LayerIDRefs     map[string]*Layer
	HashReferences  map[string]map[string]*ObjectMetadata
	Stats           PackageStats
	OSShells        map[string]*system.OSShell
	SpecialPermRefs SpecialPermsRefsInfo
	Certs           CertsRefInfo
	CACerts         CertsRefInfo
	IdentityData    *sysidentity.DataSet
}

type CertsRefInfo struct {
	Bundles         map[string]struct{} `json:"bundles,omitempty"`
	Files           map[string]struct{} `json:"files,omitempty"`
	Links           map[string]string   `json:"links,omitempty"`
	Hashes          map[string]string   `json:"hashes,omitempty"`
	PrivateKeys     map[string]struct{} `json:"private_keys,omitempty"`
	PrivateKeyLinks map[string]string   `json:"private_key_links,omitempty"`
}

type CertsInfo struct {
	Bundles         []string          `json:"bundles,omitempty"`
	Files           []string          `json:"files,omitempty"`
	Links           map[string]string `json:"links,omitempty"`
	Hashes          map[string]string `json:"hashes,omitempty"`
	PrivateKeys     []string          `json:"private_keys,omitempty"`
	PrivateKeyLinks map[string]string `json:"private_key_links,omitempty"`
}

type SpecialPermsRefsInfo struct {
	Setuid map[string]*ObjectMetadata
	Setgid map[string]*ObjectMetadata
	Sticky map[string]*ObjectMetadata
}

type SpecialPermsInfo struct {
	Setuid []string `json:"setuid,omitempty"`
	Setgid []string `json:"setgid,omitempty"`
	Sticky []string `json:"sticky,omitempty"`
}

type ImageReport struct {
	Stats        PackageStats                     `json:"stats"`
	Duplicates   map[string]*DuplicateFilesReport `json:"duplicates,omitempty"`
	SpecialPerms *SpecialPermsInfo                `json:"special_perms,omitempty"`
	OSShells     []*system.OSShell                `json:"shells,omitempty"`
	Certs        CertsInfo                        `json:"certs"`
	CACerts      CertsInfo                        `json:"ca_certs"`
	BuildInfo    *BuildKitBuildInfo               `json:"build_info,omitempty"`
	Identities   *sysidentity.Report              `json:"identities,omitempty"`
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
	UTF8Count              uint64 `json:"utf8_count,omitempty"`
	UTF8Size               uint64 `json:"utf8_size,omitempty"`
	UTF8SizeHuman          string `json:"utf8_size_human,omitempty"`
	BinaryCount            uint64 `json:"binary_count,omitempty"`
	BinarySize             uint64 `json:"binary_size,omitempty"`
	BinarySizeHuman        string `json:"binary_size_human,omitempty"`
	SetuidCount            uint64 `json:"setuid_count,omitempty"`
	SetgidCount            uint64 `json:"setgid_count,omitempty"`
	StickyCount            uint64 `json:"sticky_count,omitempty"`
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
	UTF8Count               uint64 `json:"utf8_count,omitempty"`
	UTF8Size                uint64 `json:"utf8_size,omitempty"`
	UTF8SizeHuman           string `json:"utf8_size_human,omitempty"`
	BinaryCount             uint64 `json:"binary_count,omitempty"`
	BinarySize              uint64 `json:"binary_size,omitempty"`
	BinarySizeHuman         string `json:"binary_size_human,omitempty"`
	SetuidCount             uint64 `json:"setuid_count,omitempty"`
	SetgidCount             uint64 `json:"setgid_count,omitempty"`
	StickyCount             uint64 `json:"sticky_count,omitempty"`
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

type UTF8Detector struct {
	Dump         bool
	DumpConsole  bool
	DumpDir      string
	DumpArchive  string
	MaxSizeBytes int
	Archive      *TarWriter
	Filters      []UTF8DetectorMatcher
}

type UTF8DetectorMatcher struct {
	PathPattern string
	DataPattern string
	Matcher     *regexp.Regexp
}

type TarWriter struct {
	file       *os.File
	bufferGzip *gzip.Writer
	Writer     *tar.Writer
}

func NewTarWriter(name string) (*TarWriter, error) {
	file, err := os.Create(name)
	if err != nil {
		return nil, err
	}

	bufferGzip := gzip.NewWriter(file)
	writer := tar.NewWriter(bufferGzip)

	tw := &TarWriter{
		file:       file,
		bufferGzip: bufferGzip,
		Writer:     writer,
	}

	return tw, nil
}

func (w *TarWriter) Close() error {
	if err := w.Writer.Close(); err != nil {
		return err
	}

	if err := w.bufferGzip.Close(); err != nil {
		return err
	}

	if err := w.file.Close(); err != nil {
		return err
	}

	return nil
}

func (d *UTF8Detector) Close() error {
	if d.Archive != nil {
		return d.Archive.Close()
	}

	return nil
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
	SizeHuman        string         `json:"size_human,omitempty"`
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
	Type             string         `json:"type"`
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
		Format:         FormatUnknown,
		LayerIDRefs:    map[string]*Layer{},
		HashReferences: map[string]map[string]*ObjectMetadata{},
		OSShells:       map[string]*system.OSShell{},
		SpecialPermRefs: SpecialPermsRefsInfo{
			Setuid: map[string]*ObjectMetadata{},
			Setgid: map[string]*ObjectMetadata{},
			Sticky: map[string]*ObjectMetadata{},
		},
		Certs: CertsRefInfo{
			Bundles:         map[string]struct{}{},
			Files:           map[string]struct{}{},
			Links:           map[string]string{},
			Hashes:          map[string]string{},
			PrivateKeys:     map[string]struct{}{},
			PrivateKeyLinks: map[string]string{},
		},
		CACerts: CertsRefInfo{
			Bundles:         map[string]struct{}{},
			Files:           map[string]struct{}{},
			Links:           map[string]string{},
			Hashes:          map[string]string{},
			PrivateKeys:     map[string]struct{}{},
			PrivateKeyLinks: map[string]string{},
		},
		IdentityData: sysidentity.NewDataSet(),
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

//todo: refactor, so we don't have duplicated structures

type DetectOpParam struct {
	/// Operation is enabled
	Enabled bool
	/// Dump/save raw data
	DumpRaw bool
	/// Dump raw data to console
	IsConsoleOut bool
	/// Dump raw data to directory (otherwise save to an archive file)
	IsDirOut bool
	/// Output path (directory or archive path)
	OutputPath string
	/// Input parameters for the operation
	InputParams map[string]string
}

// todo: add other processor params (passed separately for now)
type ProcessorParams struct {
	DetectIdentities     *DetectOpParam
	DetectScheduledTasks *DetectOpParam
	DetectServices       *DetectOpParam
	DetectSystemHooks    *DetectOpParam

	DetectAllCertFiles   bool
	DetectAllCertPKFiles bool
}

type LayerLocation struct {
	Position int
	Path     string
	LayerID  string
}

type LayerLocationSource string

const (
	DockerManifestLayersLocation = "ll.dm.layers"
	OCIImageManifestLocation     = "ll.oci.imagemanifest"
)

func LoadPackage(archivePath string,
	imageID string,
	skipObjects bool,
	topChangesMax int,
	doHashData bool,
	doDetectDuplicates bool,
	changeDataHashMatchers map[string]*ChangeDataHashMatcher,
	changePathMatchers []*ChangePathMatcher,
	changeDataMatchers map[string]*ChangeDataMatcher,
	utf8Detector *UTF8Detector,
	processorParams *ProcessorParams,
) (*Package, error) {
	imageID = dockerutil.CleanImageID(imageID)

	cpmDumps := hasChangePathMatcherDumps(changePathMatchers)
	dv1ConfigObjectFileName := fmt.Sprintf("%s.json", imageID)
	afile, err := os.Open(archivePath)
	if err != nil {
		log.Errorf("dockerimage.LoadPackage: os.Open error - %v", err)
		return nil, err
	}

	defer afile.Close()

	pkg := newPackage()
	layers := map[string]*Layer{}

	archiveFiles := map[string]struct{}{}
	var tarFileCount uint
	var foundOCILayout bool
	var foundBlobsDir bool
	var foundIndex bool
	var foundDockerManifest bool
	var foundDockerV1Config bool
	var foundDockerV1Layer bool
	var ociImageManifestDesc *oci.Descriptor

	tr := tar.NewReader(afile)
	for {
		hdr, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			log.Errorf("dockerimage.LoadPackage: error reading archive(%v) enumerating files - %v", archivePath, err)
			return nil, err
		}

		tarFileCount++
		if hdr == nil || hdr.Name == "" {
			log.Debugf("dockerimage.LoadPackage: ignoring bad tar header (%d)", tarFileCount)
			continue
		}

		hdr.Name = filepath.Clean(hdr.Name)
		archiveFiles[hdr.Name] = struct{}{}
		if hdr.Name == ociBlobDirName ||
			strings.HasPrefix(hdr.Name, ociBlobDirPrefix) {
			foundBlobsDir = true
		}

		switch {
		case hdr.Name == ociLayoutFileName:
			var ociLayout OCILayout
			if err := jsonFromStream(archivePath, hdr.Name, tr, &ociLayout); err != nil {
				log.Debugf("dockerimage.LoadPackage: error reading oci-layout from archive(%v/%v) - %v", archivePath, ociLayoutFileName, err)
				//not erroring right away (ok if we don't have a good oci layout if the Docker Manifest info is still there)
			} else {
				if ociLayout.Version != ociLayoutVersion {
					log.Debugf("dockerimage.LoadPackage: unexpected version in oci-layout from archive(%v/%v) - %#v", archivePath, ociLayoutFileName, ociLayout)
				}

				foundOCILayout = true
			}

		case hdr.Name == ociIndexFileName:
			var ociIndex oci.Index
			if err := jsonFromStream(archivePath, hdr.Name, tr, &ociIndex); err != nil {
				log.Debugf("dockerimage.LoadPackage: error reading oci index from archive(%v/%v) - %v", archivePath, ociIndexFileName, err)
				//not erroring right away (ok if we don't have a good oci index if the Docker Manifest info is still there)
			} else {
				if ociIndex.MediaType == oci.MediaTypeImageIndex {
					// Docker bug (ociIndex.Manifests is null when image is saved by ID, not by name)
					if len(ociIndex.Manifests) != 0 {
						// picking the first usable manifest descriptor (for now)
						// make it selectable later
						for _, md := range ociIndex.Manifests {
							md := md
							if md.MediaType == oci.MediaTypeImageManifest &&
								md.Platform != nil &&
								md.Platform.OS == DefaultOS &&
								md.Platform.Architecture == DefaultRuntimeArch() {
								ociImageManifestDesc = &md
								break
							}
						}

						if ociImageManifestDesc == nil {
							for _, md := range ociIndex.Manifests {
								md := md
								if md.MediaType == oci.MediaTypeImageManifest &&
									md.Platform != nil &&
									md.Platform.OS == DefaultOS {
									ociImageManifestDesc = &md
									break
								}
							}
						}

						if ociImageManifestDesc == nil {
							for _, md := range ociIndex.Manifests {
								md := md
								if md.MediaType == oci.MediaTypeImageManifest {
									ociImageManifestDesc = &md
									break
								}
							}

							if ociImageManifestDesc == nil {
								log.Debugf("dockerimage.LoadPackage: oci index from archive(%s/%s) has no image manifest references - '%s'",
									archivePath, ociIndexFileName, jsonutil.ToString(ociIndex))
							}
						}

						if ociImageManifestDesc != nil {
							log.Tracef("dockerimage.LoadPackage: archive(%v) - OCI Index - found image manifest = '%#v'",
								archivePath,
								ociImageManifestDesc)
						}

						//index file counts only if it's somewhat well structured
						foundIndex = true
					} else {
						log.Debugf("dockerimage.LoadPackage: malformed index file - no image manifests")
						//still ok if the Docker Manifest file has the data we need
					}
				}
			}

			log.Tracef("dockerimage.LoadPackage: oci index from archive(%v/%v) [found oci image manifest desc - '%v'] = %s",
				archivePath, ociIndexFileName, ociImageManifestDesc != nil, jsonutil.ToString(ociIndex))

		case hdr.Name == dockerManifestFileName:
			var manifests []DockerManifestObject
			if err := jsonFromStream(archivePath, hdr.Name, tr, &manifests); err != nil {
				log.Debugf("dockerimage.LoadPackage: error reading manifest file from archive(%v/%v) - %v",
					archivePath, dockerManifestFileName, err)
				//not erroring right away (ok if we don't have a good Docker Manifest if the OCI Index and OCI Image Manifest info is still there)
			} else {
				if len(manifests) != 0 {
					pkg.Manifest = &manifests[0]
					/*
						TODO: REFACTOR
						use either v1 config name
						or the config name from the OCI manifest

						for _, m := range manifests {
							if m.Config == dv1ConfigObjectFileName {
								manifest := m
								pkg.Manifest = &manifest
								break
							}
						}
					*/

					foundDockerManifest = true
				} else {
					log.Debugf("dockerimage.LoadPackage: malformed manifest file - no manifests")
					//still ok if we have the OCI image manifest (referenced in the OCI index)
				}
			}

			log.Tracef("dockerimage.LoadPackage: Docker manifest from archive(%v/%v) = %s",
				archivePath, dockerManifestFileName, jsonutil.ToString(manifests))

		case hdr.Name == dv1ConfigObjectFileName:
			var imageConfig ConfigObject
			if err := jsonFromStream(archivePath, hdr.Name, tr, &imageConfig); err != nil {
				log.Errorf("dockerimage.LoadPackage: error reading config object from archive(%v/%v) - %v", archivePath, dv1ConfigObjectFileName, err)
				return nil, err
			}

			if imageConfig.BuildInfoRaw != "" {
				imageConfig.BuildInfoDecoded, err = buildInfoDecode(imageConfig.BuildInfoRaw)
			}

			pkg.Config = &imageConfig
			foundDockerV1Config = true

		case strings.HasSuffix(hdr.Name, dockerV1LayerSuffix):
			foundDockerV1Layer = true
		}
	}

	log.Tracef("dockerimage.LoadPackage: archive(%v) - tfc=%d, archiveFiles=%d [dm=%v/dv1config=%v/dv1layer=%v/ocilayout=%v/ociindex=%v/ociblobs=%v]",
		archivePath,
		tarFileCount,
		len(archiveFiles),
		foundDockerManifest,
		foundDockerV1Config,
		foundDockerV1Layer,
		foundOCILayout,
		foundIndex,
		foundBlobsDir)

	if foundOCILayout &&
		foundBlobsDir &&
		foundIndex {
		if foundDockerManifest {
			pkg.Format = FormatDockerOCI
			pkg.IsOCI = true
		} else {
			pkg.Format = FormatOCI
			pkg.IsOCI = true
		}
	} else if foundDockerManifest &&
		foundDockerV1Config {
		pkg.Format = FormatDockerV1
	}

	log.Debugf("dockerimage.LoadPackage: pkg.Format=%s pkg.IsOCI=%v archive - (%s)",
		pkg.Format, pkg.IsOCI, archivePath)

	var configObjectFileName string
	//list of layer IDs based on their order in Docker Manifest or OCI Image Manifest
	var layerSequence []LayerLocation
	var ociLayerSequence []LayerLocation
	var layerLocationSource LayerLocationSource

	layerFileNames := map[string]struct{}{}
	ociLayerFileNames := map[string]struct{}{}
	nonLayerFileNames := map[string]string{}
	if pkg.Manifest != nil {
		if pkg.Manifest.Config != "" &&
			pkg.Manifest.Config != dv1ConfigObjectFileName {
			configObjectFileName = pkg.Manifest.Config
			log.Tracef("dockerimage.LoadPackage: archive(%v) - DM - config object path = '%s'", archivePath, configObjectFileName)
		}

		if len(pkg.Manifest.Layers) > 0 {
			layerLocationSource = DockerManifestLayersLocation

			for idx, layerPath := range pkg.Manifest.Layers {
				parts := strings.Split(layerPath, "/")
				var layerID string
				if parts[0] == ociBlobDirName {
					layerID = parts[2]
				} else {
					layerID = parts[0]
				}

				layerSequence = append(layerSequence, LayerLocation{
					Position: idx,
					Path:     layerPath,
					LayerID:  layerID,
				})

				layerFileNames[layerPath] = struct{}{}
			}

			log.Tracef("dockerimage.LoadPackage: archive(%v) - DM - layer paths [%d] = '%#v'",
				archivePath,
				len(layerFileNames),
				layerFileNames)
		} else {
			// todo: figure out the layer file names from pkg.Manifest.LayerSources
		}
	}

	// note: for a 'Docker OCI' image loading the OCI image manifest is not a "must have"
	var ociImageManifestPath string
	if ociImageManifestDesc != nil &&
		ociImageManifestDesc.Digest.String() != "" {
		parts := strings.Split(ociImageManifestDesc.Digest.String(), ":")
		if len(parts) == 2 {
			ociImageManifestPath = filepath.Join(ociBlobDirName, parts[0], parts[1])
			log.Tracef("dockerimage.LoadPackage: archive(%v) - found OCI image manifest path = '%s'",
				archivePath,
				ociImageManifestPath)

			if _, found := archiveFiles[ociImageManifestPath]; !found {
				//show an error, but don't exit
				log.Errorf("dockerimage.LoadPackage: malformed oci image manifest path from archive(%s) - %s",
					archivePath, ociImageManifestPath)
				ociImageManifestPath = ""
			}
		} else {
			log.Errorf("dockerimage.LoadPackage: malformed oci image manifest digest from archive(%s) - %s",
				archivePath, ociImageManifestDesc.Digest.String())
		}

		afile.Seek(0, 0)
		tr = tar.NewReader(afile)
		for {
			hdr, err := tr.Next()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				log.Errorf("dockerimage.LoadPackage: error reading archive(%v) enumerating files - %v", archivePath, err)
				return nil, err
			}

			if hdr == nil || hdr.Name == "" {
				log.Debugf("dockerimage.LoadPackage: ignoring bad tar header (%d)", tarFileCount)
				continue
			}

			hdr.Name = filepath.Clean(hdr.Name)

			if configObjectFileName != "" &&
				hdr.Name == configObjectFileName {
				var imageConfig ConfigObject
				if err := jsonFromStream(archivePath, hdr.Name, tr, &imageConfig); err != nil {
					log.Errorf("dockerimage.LoadPackage: error reading config object from archive(%v/%v) - %v", archivePath, configObjectFileName, err)
					return nil, err
				}

				if imageConfig.BuildInfoRaw != "" {
					imageConfig.BuildInfoDecoded, err = buildInfoDecode(imageConfig.BuildInfoRaw)
				}

				log.Tracef("dockerimage.LoadPackage: archive(%v) - loaded config object from path = '%s'",
					archivePath,
					configObjectFileName)

				//possible to have a v1 pkg.Config already
				//good to check if they are different
				pkg.Config = &imageConfig
				configObjectFileName = ""
				continue
			}

			if pkg.ManifestOCI == nil &&
				ociImageManifestPath != "" &&
				hdr.Name == ociImageManifestPath {
				var ociImageManifest oci.Manifest
				if err := jsonFromStream(archivePath, hdr.Name, tr, &ociImageManifest); err != nil {
					log.Errorf("dockerimage.LoadPackage: error reading oci image manifest from archive(%v/%v) - %v", archivePath, ociImageManifestPath, err)
					return nil, err
				}

				log.Tracef("dockerimage.LoadPackage: archive(%v) - loaded OCI Image Manifest from path = '%s'",
					archivePath,
					ociImageManifestPath)

				pkg.ManifestOCI = &ociImageManifest
				ociImageManifestPath = ""
			}

			if configObjectFileName == "" && ociImageManifestPath == "" {
				break
			}
		}
	}

	if pkg.ManifestOCI != nil &&
		pkg.ManifestOCI.Config.Digest.String() != "" {
		// when we don't have a Docker manifest
		parts := strings.Split(pkg.ManifestOCI.Config.Digest.String(), ":")
		if len(parts) == 2 {
			ociConfigObjectFileName := filepath.Join(ociBlobDirName, parts[0], parts[1])
			log.Tracef("dockerimage.LoadPackage: archive(%v) - config object path from OCI Image Manifest = '%s'",
				archivePath,
				ociConfigObjectFileName)

			if _, found := archiveFiles[ociConfigObjectFileName]; !found {
				//show the error, but don't exit
				log.Errorf("dockerimage.LoadPackage: malformed oci config object path from archive(%s) - %s",
					archivePath, ociConfigObjectFileName)
				//make it empty,so we don't attempt to extract the config again
				configObjectFileName = ""
			} else {
				if ociConfigObjectFileName != configObjectFileName {
					log.Debugf("dockerimage.LoadPackage: config object path mismatch (OCI='%s' != DM='%s') from archive(%s)",
						ociConfigObjectFileName, configObjectFileName, archivePath)
					//make sure we fetch and save the OCI config
					configObjectFileName = ociConfigObjectFileName
				} else {
					configObjectFileName = ""
				}
			}
		} else {
			log.Errorf("dockerimage.LoadPackage: malformed oci config object digest from archive(%s) - %s",
				archivePath, pkg.ManifestOCI.Config.Digest.String())
		}
	}

	if pkg.ManifestOCI != nil {
		// get layers from oci image manifest
		for idx, layerInfo := range pkg.ManifestOCI.Layers {
			// todo: add support for oci.MediaTypeImageLayerGzip and oci.MediaTypeImageLayerZstd
			if layerInfo.MediaType == oci.MediaTypeImageLayer &&
				layerInfo.Digest.String() != "" {
				parts := strings.Split(layerInfo.Digest.String(), ":")
				if len(parts) == 2 {
					layerFileName := filepath.Join(ociBlobDirName, parts[0], parts[1])
					if _, found := archiveFiles[layerFileName]; !found {
						//show error, but don't exit (exit if we have --strict-image-format)
						log.Errorf("dockerimage.LoadPackage: malformed oci layer path from archive(%s) - %s",
							archivePath, layerFileName)
					} else {
						ociLayerSequence = append(ociLayerSequence, LayerLocation{
							Position: idx,
							Path:     layerFileName,
							LayerID:  parts[1],
						})

						ociLayerFileNames[layerFileName] = struct{}{}

						//cross reference the OCI layers with the Docker Manifest layers (if we have them)

						if len(layerFileNames) > 0 {
							if _, layerFound := layerFileNames[layerFileName]; !layerFound {
								log.Debugf("dockerimage.LoadPackage: oci layer path (%s) should be known already - archive(%s)",
									layerFileName, archivePath)
							}
						}

						if len(layerSequence) >= len(ociLayerSequence) {
							lsIdx := len(ociLayerSequence) - 1
							if layerSequence[lsIdx].Path != ociLayerSequence[lsIdx].Path ||
								layerSequence[lsIdx].Position != ociLayerSequence[lsIdx].Position ||
								layerSequence[lsIdx].LayerID != ociLayerSequence[lsIdx].LayerID {
								log.Debugf("dockerimage.LoadPackage: layerFileName=%s / OCI layer and DM layer sequence record mismatch lsIdx=%d ('%+v' / '%+v') - archive(%s)",
									layerFileName, lsIdx, layerSequence[lsIdx], ociLayerSequence[lsIdx], archivePath)
							}
						} else {
							log.Debugf("dockerimage.LoadPackage: layerFileName=%s / OCI layer and DM layer sequence mismatch (%d / %d) - archive(%s)",
								layerFileName, len(layerSequence), len(ociLayerSequence), archivePath)
						}
					}
				} else {
					log.Errorf("dockerimage.LoadPackage: malformed oci layer digest from archive(%s) - %s",
						archivePath, layerInfo.Digest.String())
				}
			} else if layerInfo.MediaType != oci.MediaTypeImageLayer {
				if layerInfo.Digest.String() != "" {
					parts := strings.Split(layerInfo.Digest.String(), ":")
					if len(parts) == 2 {
						layerFileName := filepath.Join(ociBlobDirName, parts[0], parts[1])
						nonLayerFileNames[layerFileName] = layerInfo.MediaType
					} else {
						log.Errorf("dockerimage.LoadPackage: malformed oci layer digest from archive(%s) - %s",
							archivePath, layerInfo.Digest.String())
					}
				} else {
					log.Debugf("dockerimage.LoadPackage: non-image layer, no digest '%s' - archive(%s)",
						jsonutil.ToString(layerInfo), archivePath)
				}
			}
		}
	}

	// a 'simple' way to decide when to use the OCI layer metadata (todo: review later)
	if len(ociLayerFileNames) > len(layerFileNames) {
		layerFileNames = ociLayerFileNames
		layerSequence = ociLayerSequence
		layerLocationSource = OCIImageManifestLocation
	}

	// now load the layers
	afile.Seek(0, 0)
	tr = tar.NewReader(afile)
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
		if configObjectFileName != "" &&
			hdr.Name == configObjectFileName {
			var imageConfig ConfigObject
			if err := jsonFromStream(archivePath, hdr.Name, tr, &imageConfig); err != nil {
				log.Errorf("dockerimage.LoadPackage: error reading oci config object from archive(%v/%v) - %v", archivePath, configObjectFileName, err)
				return nil, err
			}

			if imageConfig.BuildInfoRaw != "" {
				imageConfig.BuildInfoDecoded, err = buildInfoDecode(imageConfig.BuildInfoRaw)
			}

			log.Tracef("dockerimage.LoadPackage: archive(%v) - loaded config object from path = '%s'",
				archivePath,
				configObjectFileName)

			//pkg.Config might alread point to a config (v1 or DM)
			//todo: check the difference
			pkg.Config = &imageConfig
			configObjectFileName = ""
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeReg, tar.TypeSymlink:
			switch {
			case strings.HasSuffix(hdr.Name, dockerV1LayerSuffix):
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
						doDetectDuplicates,
						changeDataHashMatchers,
						changePathMatchers,
						cpmDumps,
						changeDataMatchers,
						utf8Detector,
						processorParams,
					)
					if err != nil {
						log.Errorf("dockerimage.LoadPackage: error reading layer from archive(%v/%v) - %v", archivePath, hdr.Name, err)
						return nil, err
					}
				}

				layers[layerID] = layer
				log.Debugf("dockerimage.LoadPackage: saved v1 layer '%s' from archive - %s",
					layerID, archivePath)

			default:
				if _, found := layerFileNames[hdr.Name]; found {
					// todo: test if we can get symlinks...
					parts := strings.SplitN(hdr.Name, "/", 3)
					layerID := parts[2]
					layer, err := layerFromStream(
						pkg,
						hdr.Name,
						tar.NewReader(tr),
						layerID,
						topChangesMax,
						doHashData,
						doDetectDuplicates,
						changeDataHashMatchers,
						changePathMatchers,
						cpmDumps,
						changeDataMatchers,
						utf8Detector,
						processorParams,
					)
					if err != nil {
						log.Errorf("dockerimage.LoadPackage: error reading oci layer from archive(%s/%s) - %v", archivePath, hdr.Name, err)
						return nil, err
					}
					layers[layerID] = layer
					log.Debugf("dockerimage.LoadPackage: saved layer '%s' from archive - %s",
						layerID, archivePath)
				}
			}
		}
	}

	if pkg.Format == FormatOCI {
		if pkg.ManifestOCI == nil {
			return nil, fmt.Errorf("dockerimage.LoadPackage: missing OCI Image Manifest object for image ID=%s / archive='%s' / files=%+v",
				imageID, archivePath, archiveFiles)
		}
	} else {
		if pkg.Manifest == nil {
			return nil, fmt.Errorf("dockerimage.LoadPackage: missing manifest object for image ID=%s / archive='%s' / files=%+v",
				imageID, archivePath, archiveFiles)
		}
	}

	if pkg.Config == nil {
		return nil, fmt.Errorf("dockerimage.LoadPackage: missing image config object for image ID=%s / archive='%s' / files=%+v",
			imageID, archivePath, archiveFiles)
	}

	if len(layers) == 0 {
		//todo: display non-image layer metadata if there are any
		//todo: save non-image layer metadata/data
		log.Debugf("dockerimage.LoadPackage: no image layers in archive - %s (%+v)",
			archivePath, layerFileNames)
	}

	if len(nonLayerFileNames) > 0 {
		log.Debugf("dockerimage.LoadPackage: non-image layers in archive - %s (%s)",
			archivePath, jsonutil.ToString(nonLayerFileNames))
	}

	log.Debugf("dockerimage.LoadPackage: layerLocationSource=%v layerSequence='%#v' - archive='%s'",
		layerLocationSource, layerSequence, archivePath)

	for idx, layerLocationInfo := range layerSequence {
		layer, ok := layers[layerLocationInfo.LayerID]
		if !ok {
			log.Errorf("dockerimage.LoadPackage: error missing layer (idx=%d layerPath=%s layerID=%s) archive=%s",
				idx, layerLocationInfo.Path, layerLocationInfo.LayerID, archivePath)
			return nil, fmt.Errorf("dockerimage.LoadPackage: missing layer (%v) for image ID - %v", layerLocationInfo.Path, imageID)
		}

		layer.Index = idx
		//adding layers based on their manifest order
		pkg.Layers = append(pkg.Layers, layer)
		if len(pkg.Layers)-1 != layer.Index {
			return nil, fmt.Errorf("dockerimage.LoadPackage: layer index mismatch - %v / %v", len(pkg.Layers)-1, layer.Index)
		}

		if layerLocationInfo.Path != layer.Path {
			return nil, fmt.Errorf("dockerimage.LoadPackage: layer path mismatch - %v / %v", layerLocationInfo.Path, layer.Path)
		}

		if idx == 0 {
			for oidx, object := range layer.Objects {
				object.LayerIndex = idx

				if utf8Detector != nil {
					switch object.ContentType {
					case ContentTypeUTF8:
						layer.Stats.UTF8Count++
						layer.Stats.UTF8Size += uint64(object.Size)
						pkg.Stats.UTF8Count++
						pkg.Stats.UTF8Size += uint64(object.Size)
					case ContentTypeBinary:
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

				if object.Change == ChangeAdd ||
					object.Change == ChangeModify {
					if shellInfo, found := pkg.OSShells[object.Name]; found {
						if exeInfo, rfound := layer.References[shellInfo.ExePath]; rfound {
							shellInfo.Verified = true
							if exeInfo.LinkTarget != "" {
								shellInfo.LinkPath = exeInfo.LinkTarget
							}
						}
					}
				}
			}

			if utf8Detector != nil {
				layer.Stats.UTF8SizeHuman = humanize.Bytes(layer.Stats.UTF8Size)
				layer.Stats.BinarySizeHuman = humanize.Bytes(layer.Stats.BinarySize)
			}

		} else {
			for oidx, object := range layer.Objects {
				object.LayerIndex = idx

				if utf8Detector != nil {
					switch object.ContentType {
					case ContentTypeUTF8:
						layer.Stats.UTF8Count++
						layer.Stats.UTF8Size += uint64(object.Size)
						pkg.Stats.UTF8Count++
						pkg.Stats.UTF8Size += uint64(object.Size)
					case ContentTypeBinary:
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

				if object.Change == ChangeAdd ||
					object.Change == ChangeModify {
					if shellInfo, found := pkg.OSShells[object.Name]; found {
						if exeInfo, rfound := layer.References[shellInfo.ExePath]; rfound {
							shellInfo.Verified = true
							if exeInfo.LinkTarget != "" {
								shellInfo.LinkPath = exeInfo.LinkTarget
							}
						}
					}
				}
			}
			if utf8Detector != nil {
				layer.Stats.UTF8SizeHuman = humanize.Bytes(layer.Stats.UTF8Size)
				layer.Stats.BinarySizeHuman = humanize.Bytes(layer.Stats.BinarySize)
			}
		}

		pkg.LayerIDRefs[layerLocationInfo.LayerID] = layer

		if pkg.Config.RootFS != nil && idx < len(pkg.Config.RootFS.DiffIDs) {
			diffID := pkg.Config.RootFS.DiffIDs[idx]
			layer.FSDiffID = diffID
		} else {
			log.Debugf("dockerimage.LoadPackage: no FS diff for layer index %v", idx)
		}
	}

	if utf8Detector != nil {
		pkg.Stats.UTF8SizeHuman = humanize.Bytes(pkg.Stats.UTF8Size)
		pkg.Stats.BinarySizeHuman = humanize.Bytes(pkg.Stats.BinarySize)
	}

	if len(pkg.Layers) > 0 {
		currentLayerIndex := 0
		isFirstLayer := true
		for hidx := range pkg.Config.History {
			if !pkg.Config.History[hidx].EmptyLayer {
				if currentLayerIndex < (len(pkg.Layers)-1) &&
					!isFirstLayer {
					currentLayerIndex++
				}
			}

			currentLayer := pkg.Layers[currentLayerIndex]

			if currentLayer != nil {
				pkg.Config.History[hidx].LayerFSDiffID = currentLayer.FSDiffID
				pkg.Config.History[hidx].LayerID = currentLayer.ID
				pkg.Config.History[hidx].LayerIndex = currentLayer.Index
			}

			if !pkg.Config.History[hidx].EmptyLayer && isFirstLayer {
				isFirstLayer = false
			}
		}
	}

	if doDetectDuplicates {
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

func (ref *Package) ProcessIdentityData() *sysidentity.Report {
	if ref.IdentityData == nil {
		return nil
	}

	report, err := sysidentity.NewReportFromData(ref.IdentityData)
	if err != nil {
		log.Errorf("dockerimage.Package.ProcessIdentityData: error - %v", err)
		return nil
	}

	return report
}

func hasChangePathMatcherDumps(changePathMatchers []*ChangePathMatcher) bool {
	for _, cpm := range changePathMatchers {
		if cpm.PathPattern != "" && cpm.Dump {
			return true
		}
	}

	return false
}

func linkTargetToFullPath(fullPath, target string) string {
	if filepath.IsAbs(target) {
		return target
	}

	if target == "." {
		return ""
	}

	d := filepath.Dir(fullPath)

	return filepath.Clean(filepath.Join(d, target))
}

func layerFromStream(
	pkg *Package,
	layerPath string,
	tr *tar.Reader,
	layerID string,
	topChangesMax int,
	doHashData bool,
	doDetectDuplicates bool,
	changeDataHashMatchers map[string]*ChangeDataHashMatcher,
	changePathMatchers []*ChangePathMatcher,
	cpmDumps bool,
	changeDataMatchers map[string]*ChangeDataMatcher,
	utf8Detector *UTF8Detector,
	processorParams *ProcessorParams,
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
			Size:       hdr.Size,
			SizeHuman:  humanize.Bytes(uint64(hdr.Size)),
			Mode:       hdr.FileInfo().Mode(),
			UID:        hdr.Uid,
			GID:        hdr.Gid,
			ModTime:    hdr.ModTime,
			ChangeTime: hdr.ChangeTime,
			TypeFlag:   hdr.Typeflag,
			Type:       ObjectTypeFromTarType(hdr.Typeflag),
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

			nameOnly := filepath.Base(object.Name)
			if certdiscover.IsCertHashName(nameOnly) {
				if certdiscover.IsCertPKDirPath(object.Name) {
					pkg.CACerts.Hashes[object.Name] = object.LinkTarget
				} else if certdiscover.IsCertDirPath(object.Name) {
					pkg.Certs.Hashes[object.Name] = object.LinkTarget
				}
			} else {
				linkFullPath := linkTargetToFullPath(object.Name, object.LinkTarget)
				if certdiscover.IsCertFile(object.Name) {
					pkg.Certs.Bundles[object.Name] = struct{}{}
					if linkFullPath != "" {
						pkg.Certs.Bundles[linkFullPath] = struct{}{}
					}
				} else if certdiscover.IsAppCertFile(object.Name) {
					pkg.Certs.Bundles[object.Name] = struct{}{}
					if linkFullPath != "" {
						pkg.Certs.Bundles[linkFullPath] = struct{}{}
					}
				} else if certdiscover.IsCACertFile(object.Name) {
					pkg.CACerts.Bundles[object.Name] = struct{}{}
					if linkFullPath != "" {
						pkg.CACerts.Bundles[linkFullPath] = struct{}{}
					}
				} else if certdiscover.IsCACertPKFile(object.Name) {
					pkg.CACerts.PrivateKeys[object.Name] = struct{}{}
					if linkFullPath != "" {
						pkg.CACerts.PrivateKeys[linkFullPath] = struct{}{}
					}
				} else {
					//NOTE: Links require post-processing
					if certdiscover.IsCertDir(object.Name) {
						pkg.Certs.Links[object.Name] = linkFullPath
					} else if certdiscover.IsCertPKDir(object.Name) {
						pkg.Certs.PrivateKeyLinks[object.Name] = linkFullPath
					} else if certdiscover.IsCACertDir(object.Name) {
						pkg.CACerts.Links[object.Name] = linkFullPath
					} else if certdiscover.IsCACertPKDir(object.Name) {
						pkg.CACerts.PrivateKeyLinks[object.Name] = linkFullPath
					} else if certdiscover.IsCertDirPath(object.Name) {
						pkg.Certs.Links[object.Name] = linkFullPath
					} else if certdiscover.IsCACertDirPath(object.Name) {
						pkg.CACerts.Links[object.Name] = linkFullPath
					} else if certdiscover.IsCertPKDirPath(object.Name) {
						pkg.Certs.PrivateKeyLinks[object.Name] = linkFullPath
					} else if certdiscover.IsCACertPKDirPath(object.Name) {
						pkg.CACerts.PrivateKeyLinks[object.Name] = linkFullPath
					}
				}
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
				//TODO:
				//do unique pkg stats counts,
				//so we don't count the same thing multiple times
				//across different layers
				if fsutil.FileModeIsSetuid(object.Mode) {
					layer.Stats.SetuidCount++
					pkg.Stats.SetuidCount++
					pkg.SpecialPermRefs.Setuid[object.Name] = object
				}

				if fsutil.FileModeIsSetgid(object.Mode) {
					layer.Stats.SetgidCount++
					pkg.Stats.SetgidCount++
					pkg.SpecialPermRefs.Setgid[object.Name] = object
				}

				if fsutil.FileModeIsSticky(object.Mode) {
					layer.Stats.StickyCount++
					pkg.Stats.StickyCount++
					pkg.SpecialPermRefs.Sticky[object.Name] = object
				}

				err = inspectFile(
					object,
					tr,
					pkg,
					layer,
					doHashData,
					changeDataHashMatchers,
					changePathMatchers,
					cpmDumps,
					changeDataMatchers,
					utf8Detector,
					processorParams,
				)
				if err != nil {
					log.Errorf("layerFromStream: error inspecting layer file (%s) - (%v) - %v", object.Name, layerID, err)
				} else {
					if doDetectDuplicates && len(object.Hash) != 0 {
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
			} else {
				//TODO:
				//do unique pkg stats counts,
				//so we don't count the same thing multiple times
				//across different layers
				if fsutil.FileModeIsSticky(object.Mode) {
					layer.Stats.StickyCount++
					pkg.Stats.StickyCount++
					pkg.SpecialPermRefs.Sticky[object.Name] = object
				}
			}
		}
	}

	return layer, nil
}

func getStreamHash(reader io.Reader) (string, error) {
	hasher := sha1.New()

	_, err := io.Copy(hasher, reader)
	if err != nil {
		log.Errorf("getStreamHash: error=%v", err)
		return "", err
	}

	hash := hasher.Sum(nil)
	return hex.EncodeToString(hash[:]), nil
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
	pkg *Package,
	layer *Layer,
	doHashData bool,
	changeDataHashMatchers map[string]*ChangeDataHashMatcher,
	changePathMatchers []*ChangePathMatcher,
	cpmDumps bool,
	changeDataMatchers map[string]*ChangeDataMatcher,
	utf8Detector *UTF8Detector,
	processorParams *ProcessorParams,
) error {
	//TODO: refactor and enhance the OS Distro detection logic
	fullPath := object.Name

	var cdhmDumps bool
	for _, dhm := range changeDataHashMatchers {
		if dhm.Dump {
			cdhmDumps = true
			break
		}
	}

	var isKnownCertFile bool
	if certdiscover.IsCertFile(fullPath) {
		pkg.Certs.Bundles[fullPath] = struct{}{}
		isKnownCertFile = true
	} else if certdiscover.IsAppCertFile(fullPath) {
		pkg.Certs.Bundles[fullPath] = struct{}{}
		isKnownCertFile = true
	} else if certdiscover.IsCACertFile(fullPath) {
		pkg.CACerts.Bundles[fullPath] = struct{}{}
		isKnownCertFile = true
	} else if certdiscover.IsCACertPKFile(fullPath) {
		pkg.CACerts.PrivateKeys[fullPath] = struct{}{}
		isKnownCertFile = true
	}

	if (processorParams.DetectIdentities.Enabled &&
		sysidentity.IsSourceFile(fullPath)) ||
		system.IsOSReleaseFile(fullPath) ||
		system.IsOSShellsFile(fullPath) ||
		len(changeDataMatchers) > 0 ||
		cpmDumps ||
		cdhmDumps ||
		utf8Detector != nil ||
		(!isKnownCertFile && processorParams.DetectAllCertFiles) ||
		(!isKnownCertFile && processorParams.DetectAllCertPKFiles) {
		data, err := io.ReadAll(reader)
		if err != nil {
			return err
		}

		if processorParams.DetectIdentities.Enabled &&
			sysidentity.IsSourceFile(fullPath) {
			pkg.IdentityData.AddData(fullPath, data)
		}

		if !isKnownCertFile {
			if processorParams.DetectAllCertFiles {
				//NOTE:
				//not limiting detection to the main cert directories,
				//but checking the CA cert dir prefix to know where to put it
				if certdiscover.IsCertData(data) {
					if certdiscover.IsCACertDirPath(fullPath) {
						pkg.CACerts.Files[fullPath] = struct{}{}
					} else {
						pkg.Certs.Files[fullPath] = struct{}{}
					}
				}
			}

			if processorParams.DetectAllCertPKFiles {
				//NOTE: not limiting detection to the main cert private key directories
				if certdiscover.IsPrivateKeyData(data) {
					if certdiscover.IsCACertPKDirPath(fullPath) {
						pkg.CACerts.PrivateKeys[fullPath] = struct{}{}
					} else {
						pkg.Certs.PrivateKeys[fullPath] = struct{}{}
					}
				}
			}
		}

		if system.IsOSShellsFile(fullPath) {
			shellsList, _ := system.NewOSShellsFromData(data)
			for _, shellInfo := range shellsList {
				if shellInfo.Reference != "" {
					pkg.OSShells[shellInfo.Reference] = shellInfo
				} else {
					pkg.OSShells[shellInfo.ExePath] = shellInfo
				}
			}
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

		var hash string
		if doHashData ||
			len(changeDataHashMatchers) > 0 ||
			utf8Detector != nil {
			hash = getBytesHash(data)
		}

		if doHashData {
			object.Hash = hash
		}

		if len(hash) > 0 && utf8Detector != nil {
			if utf8.Valid(data) {
				object.ContentType = ContentTypeUTF8
				if utf8Detector.Dump {
					if utf8Detector.Archive != nil {
						if utf8Detector.MaxSizeBytes != 0 && object.Size > int64(utf8Detector.MaxSizeBytes) {
							fileInfo := &utf8FileInfo{
								name:    hash,
								size:    int64(utf8Detector.MaxSizeBytes),
								modtime: object.ModTime,
							}
							header, err := tar.FileInfoHeader(fileInfo, hash)
							if err != nil {
								return err
							}
							header.Name = hash
							err = utf8Detector.Archive.Writer.WriteHeader(header)
							if err != nil {
								return err
							}
							_, err = utf8Detector.Archive.Writer.Write(data[:utf8Detector.MaxSizeBytes])
							if err != nil {
								return err
							}
						} else {
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
							err = utf8Detector.Archive.Writer.WriteHeader(header)
							if err != nil {
								return err
							}
							_, err = utf8Detector.Archive.Writer.Write(data)
							if err != nil {
								return err
							}
						}
					}

					if utf8Detector.DumpConsole {
						fmt.Printf("cmd=xray info=detect.utf8.match.start\n")
						fmt.Printf("cmd=xray info=detect.utf8.match file='%s')\n", fullPath)
						fmt.Printf("%s\n", string(data))
						fmt.Printf("cmd=xray info=detect.utf8.match.end\n")
					}

					if utf8Detector.DumpDir != "" {
						dumpPath := filepath.Join(utf8Detector.DumpDir, fullPath)
						dirPath := fsutil.FileDir(dumpPath)
						if !fsutil.DirExists(dirPath) {
							err := os.MkdirAll(dirPath, 0755)
							if err != nil {
								fmt.Printf("cmd=xray info=detect.utf8.match.dump.error file='%s' target='%s' error='%s'):\n",
									fullPath, dumpPath, err)
								return err
							}
						}

						err := os.WriteFile(dumpPath, data, 0644)
						if err != nil {
							fmt.Printf("cmd=xray info=detect.utf8.match.dump.error file='%s' target='%s' error='%s'):\n",
								fullPath, dumpPath, err)
							return err
						}

						fmt.Printf("cmd=xray info=detect.utf8.match.dump file='%s' target='%s'):\n",
							fullPath, dumpPath)
					}
				}
			} else {
				object.ContentType = ContentTypeBinary
			}
		}

		if len(hash) > 0 && len(changeDataHashMatchers) > 0 {
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

					err := os.WriteFile(dumpPath, data, 0644)
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

					err := os.WriteFile(dumpPath, data, 0644)
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

						err := os.WriteFile(dumpPath, data, 0644)
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

		var hash string
		if doHashData || len(changeDataHashMatchers) > 0 {
			var err error
			hash, err = getStreamHash(reader)
			if err != nil {
				log.Errorf("inspectFile: getStreamHash error - name='%s' error=%v", fullPath, err)
				return err
			}
		}

		if doHashData {
			object.Hash = hash
		}

		if len(hash) > 0 && len(changeDataHashMatchers) > 0 {
			if dhm, found := changeDataHashMatchers[hash]; found {
				//need to save to DataHashMatches to make it work without generating/saving hashes for all objects
				layer.DataHashMatches[fullPath] = dhm

				if dhm.Dump {
					log.Errorf("inspectFile: should not dump - %#v", dhm)
				}
			}
		}
	}
	return nil
}

func jsonFromStream(source string, name string, reader io.Reader, data interface{}) error {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	sr := strings.NewReader(string(raw))
	log.Tracef("dockerimage.LoadPackage.jsonFromStream: name='%s' data[%d]='%s' source='%s'",
		name, len(raw), string(raw), source)

	return json.NewDecoder(sr).Decode(data)
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
				return io.ReadAll(tr)
			}
		}
	}

	return nil, fmt.Errorf("no file - %s", filePath)
}

func LoadManifestObject(archivePath, imageID string) (*DockerManifestObject, error) {
	return nil, nil
}

func LoadConfigObject(archivePath, imageID string) (*ConfigObject, error) {
	return nil, nil
}

func LoadLayer(archivePath, imageID, layerID string) (*Layer, error) {
	return nil, nil
}
