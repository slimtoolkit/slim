package dockerimage

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// pkg/system/architecture.go has architecture enums,
// but the enums are not always identical
const (
	OSLinux   = "linux"
	DefaultOS = OSLinux

	ArchARM     = "arm"
	ArchARM64   = "arm64"
	ArchAMD64   = "amd64"
	DefaultArch = ArchAMD64
)

func DefaultRuntimeArch() string {
	return runtime.GOARCH
}

// focusing on the primary OS and architectures (ignoring the rest)

var ArchVarMap = map[string][]string{
	ArchARM:   {"", "v6", "v7", "v8"},
	ArchARM64: {"", "v8"},
	ArchAMD64: {""},
}

func IsValidArchitecture(arch string, variant string) bool {
	for varch, vvariants := range ArchVarMap {
		if varch == arch {
			for _, vvariant := range vvariants {
				if vvariant == variant {
					return true
				}
			}
			return false
		}
	}

	return false
}

var OSArchMap = map[string][]string{
	OSLinux: {
		"386",
		ArchAMD64,
		ArchARM,
		ArchARM64,
		"ppc64",
		"ppc64le",
		"mips64",
		"mips64le",
		"s390x",
		"riscv64"},
}

func IsValidPlatform(os string, arch string) bool {
	for vos, varchs := range OSArchMap {
		if vos == os {
			for _, varch := range varchs {
				if varch == arch {
					return true
				}
			}
			return false
		}
	}

	return false
}

// v1 and oci-based Manifest structure is the same* / compatible
// but config and layer paths are different
type DockerManifestObject struct {
	// v1 format:  "IMAGE_ID.json" (no sha256 prefix in IMAGE_ID)
	// oci format: "blobs/sha256/DIGEST_NO_PREFIX"
	Config   string
	RepoTags []string `json:",omitempty"` //["user/repo:tag"]
	// v1 format:  "LAYER_ID/layer.tar" (no sha256 prefix in LAYER_ID)
	// oci format: "blobs/sha256/DIGEST" (no sha256 prefix in DIGEST)
	Layers []string

	// newer fields

	Parent string `json:",omitempty"`
	// DiffID map (where DiffID does have the "sha256:" prefix)
	LayerSources map[string]BlobDescriptor `json:",omitempty"`
}

// can reuse the 'Descriptor' for 'BlobDescriptor' from github.com/opencontainers/image-spec/specs-go/v1

type BlobDescriptor struct {
	MediaType string `json:"mediaType,omitempty"`
	Size      int64  `json:"size,omitempty"`
	Digest    string `json:"digest,omitempty"`

	// extra fields
	Annotations map[string]string `json:"annotations,omitempty"`
	URLs        []string          `json:"urls,omitempty"`
	Platform    *Platform         `json:"platform,omitempty"`
}

// todo: can reuse 'Platform' from github.com/opencontainers/image-spec/specs-go/v1

type Platform struct {
	Architecture string   `json:"architecture"`
	OS           string   `json:"os"`
	OSVersion    string   `json:"os.version,omitempty"`
	OSFeatures   []string `json:"os.features,omitempty"`
	Variant      string   `json:"variant,omitempty"`
}

// data structures from https://github.com/moby/moby/blob/master/image

type V1ConfigObject struct {
	// ID is a unique 64 character identifier of the image
	ID string `json:"id,omitempty"`
	// Parent is the ID of the parent image
	Parent string `json:"parent,omitempty"`
	// Comment is the commit message that was set when committing the image
	Comment string `json:"comment,omitempty"`
	// Created is the timestamp at which the image was created
	Created time.Time `json:"created"`
	// Container is the id of the container used to commit
	Container string `json:"container,omitempty"`
	// ContainerConfig is the configuration of the container that is committed into the image
	ContainerConfig ContainerConfig `json:"container_config,omitempty"`
	// DockerVersion specifies the version of Docker that was used to build the image
	DockerVersion string `json:"docker_version,omitempty"`
	// Author is the name of the author that was specified when committing the image
	Author string `json:"author,omitempty"`
	// Config is the configuration of the container received from the client
	Config *ContainerConfig `json:"config,omitempty"`
	// Architecture is the hardware that the image is built and runs on
	Architecture string `json:"architecture,omitempty"`
	// Variant is the CPU architecture variant (presently ARM-only)
	Variant string `json:"variant,omitempty"`
	// OS is the operating system used to build and run the image
	OS string `json:"os,omitempty"`
	// Size is the total size of the image including all layers it is composed of
	Size int64 `json:",omitempty"`
}

type ConfigObject struct {
	V1ConfigObject
	Parent     string     `json:"parent,omitempty"` //nolint:govet
	RootFS     *RootFS    `json:"rootfs,omitempty"`
	History    []XHistory `json:"history,omitempty"`
	OSVersion  string     `json:"os.version,omitempty"`
	OSFeatures []string   `json:"os.features,omitempty"`

	//buildkit build info
	BuildInfoRaw     string `json:"moby.buildkit.buildinfo.v1,omitempty"`
	BuildInfoDecoded *BuildKitBuildInfo
}

//data structures from https://github.com/moby/moby/blob/master/image/rootfs.go

const TypeLayers = "layers"

type RootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids,omitempty"`
}

//data structures from https://github.com/moby/moby/blob/master/image/image.go

// XHistory augments the standard History struct with extra layer info
type XHistory struct {
	// Created is the timestamp at which the image was created
	Created time.Time `json:"created"`
	// Author is the name of the author that was specified when committing the image
	Author string `json:"author,omitempty"`
	// CreatedBy keeps the Dockerfile command used while building the image
	CreatedBy string `json:"created_by,omitempty"`
	// Comment is the commit message that was set when committing the image
	Comment string `json:"comment,omitempty"`
	// EmptyLayer is set to true if this history item did not generate a
	// layer. Otherwise, the history item is associated with the next
	// layer in the RootFS section.
	EmptyLayer bool `json:"empty_layer,omitempty"`

	//extra fields

	LayerID       string `json:"layer_id,omitempty"`
	LayerIndex    int    `json:"layer_index"`
	LayerFSDiffID string `json:"layer_fsdiff_id,omitempty"`
}

//data structures from https://github.com/moby/moby/blob/master/api/types/container/config.go

type ContainerConfig struct {
	Hostname        string              // Hostname
	Domainname      string              // Domainname
	User            string              // User that will run the command(s) inside the container, also support user:group
	AttachStdin     bool                // Attach the standard input, makes possible user interaction
	AttachStdout    bool                // Attach the standard output
	AttachStderr    bool                // Attach the standard error
	ExposedPorts    map[string]struct{} `json:",omitempty"` // List of exposed ports
	Tty             bool                // Attach standard streams to a tty, including stdin if it is not closed.
	OpenStdin       bool                // Open stdin
	StdinOnce       bool                // If true, close stdin after the 1 attached client disconnects.
	Env             []string            // List of environment variable to set in the container
	Cmd             []string            // Command to run when starting the container
	Healthcheck     *HealthConfig       `json:",omitempty"` // Healthcheck describes how to check the container is healthy
	ArgsEscaped     bool                `json:",omitempty"` // True if command is already escaped (meaning treat as a command line) (Windows specific).
	Image           string              // Name of the image as it was passed by the operator (e.g. could be symbolic)
	Volumes         map[string]struct{} // List of volumes (mounts) used for the container
	WorkingDir      string              // Current directory (PWD) in the command will be launched
	Entrypoint      []string            // Entrypoint to run when starting the container
	NetworkDisabled bool                `json:",omitempty"` // Is network disabled
	MacAddress      string              `json:",omitempty"` // Mac Address of the container
	OnBuild         []string            // ONBUILD metadata that were defined on the image Dockerfile
	Labels          map[string]string   // List of labels set to this container
	StopSignal      string              `json:",omitempty"` // Signal to stop a container
	StopTimeout     *int                `json:",omitempty"` // Timeout (in seconds) to stop a container
	Shell           []string            `json:",omitempty"` // Shell for shell-form of RUN, CMD, ENTRYPOINT
}

//data structures from https://github.com/moby/moby/blob/master/api/types/container/config.go

type HealthConfig struct {
	// Test is the test to perform to check that the container is healthy.
	// An empty slice means to inherit the default.
	// The options are:
	// {} : inherit healthcheck
	// {"NONE"} : disable healthcheck
	// {"CMD", args...} : exec arguments directly
	// {"CMD-SHELL", command} : run command with system's default shell
	Test []string `json:",omitempty"`

	// Zero means to inherit. Durations are expressed as integer nanoseconds.
	Interval    time.Duration `json:",omitempty"` // Interval is the time to wait between checks.
	Timeout     time.Duration `json:",omitempty"` // Timeout is the time to wait before considering the check to have hung.
	StartPeriod time.Duration `json:",omitempty"` // The start period for the container to initialize before the retries starts to count down.

	// Retries is the number of consecutive failures needed to consider a container as unhealthy.
	// Zero means inherit.
	Retries int `json:",omitempty"`
}

//data structures from https://github.com/moby/buildkit/blob/master/util/buildinfo/types/types.go

type BuildKitBuildInfo struct {
	// Frontend defines the frontend used to build.
	Frontend string `json:"frontend,omitempty"`
	// Attrs defines build request attributes.
	Attrs map[string]*string `json:"attrs,omitempty"`
	// Sources defines build dependencies.
	Sources []*BuildSource `json:"sources,omitempty"`
	// Deps defines context dependencies.
	Deps map[string]BuildKitBuildInfo `json:"deps,omitempty"`
}

type BuildSource struct {
	// Type defines the SourceType source type (docker-image, git, http).
	Type SourceType `json:"type,omitempty"`
	// Ref is the reference of the source.
	Ref string `json:"ref,omitempty"`
	// Alias is a special field used to match with the actual source ref
	// because frontend might have already transformed a string user typed
	// before generating LLB.
	Alias string `json:"alias,omitempty"`
	// Pin is the source digest.
	Pin string `json:"pin,omitempty"`
}

type SourceType string

const (
	SourceTypeDockerImage SourceType = "docker-image"
	SourceTypeGit         SourceType = "git"
	SourceTypeHTTP        SourceType = "http"
)

func buildInfoDecode(encoded string) (*BuildKitBuildInfo, error) {
	if encoded == "" {
		return nil, nil
	}

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	var info BuildKitBuildInfo
	if err = json.Unmarshal(raw, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// consts from https://github.com/moby/moby/blob/master/pkg/archive/whiteouts.go

// WhiteoutPrefix prefix means file is a whiteout. If this is followed by a
// filename this means that file has been removed from the base layer.
const WhiteoutPrefix = ".wh."

// WhiteoutMetaPrefix prefix means whiteout has a special meaning and is not
// for removing an actual file. Normally these files are excluded from exported
// archives.
const WhiteoutMetaPrefix = WhiteoutPrefix + WhiteoutPrefix

// WhiteoutLinkDir is a directory AUFS uses for storing hardlink links to other
// layers. Normally these should not go into exported archives and all changed
// hardlinks should be copied to the top layer.
const WhiteoutLinkDir = WhiteoutMetaPrefix + "plnk"

// WhiteoutOpaqueDir file means directory has been made opaque - meaning
// readdir calls to this directory do not follow to lower layers.
const WhiteoutOpaqueDir = WhiteoutMetaPrefix + ".opq"

func IsDeletedFileObject(path string) bool {
	name := filepath.Base(path)
	return strings.HasPrefix(name, WhiteoutPrefix)
}

func NormalizeFileObjectLayerPath(path string) (string, bool, bool, error) {
	isDeleted := false
	name := filepath.Base(path)
	if name == WhiteoutOpaqueDir {
		//return a fake wildcard delete path for now (to make it easy to detect)
		return filepath.Join(filepath.Dir(path), "*"), true, true, nil
	}

	if strings.HasPrefix(name, WhiteoutPrefix) {
		restored := name[len(WhiteoutPrefix):]
		dir := filepath.Dir(path)
		isDeleted = true
		path = filepath.Join(dir, restored)
	}

	return path, isDeleted, false, nil
}
