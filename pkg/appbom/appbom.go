/*
There are two ways to provide the Go executable hash to "appbom":

1. "go generate" and "embed"
2. "-ldflags"

Using "go generate" to hash the Go binary:

go generate ./...
go generate github.com/slimtoolkit/slim/pkg/appbom

With "go generate" you also need to use embedding (enabled by default).
If you can't use "embed" you can disable it with the "appbom_noembed" tag:

go build -tags appbom_noembed

If you disable embedding then you'll need to pass the Go executable hash using "-ldflags":

Mac:

go build -ldflags "-X github.com/slimtoolkit/slim/pkg/appbom.GoBinHash=sha256:$(shasum -a 256 $(go env GOROOT)/bin/go | head -c 64)"

Linux:

go build -ldflags "-X github.com/slimtoolkit/slim/pkg/appbom.GoBinHash=sha256:$(sha256sum $(go env GOROOT)/bin/go | head -c 64)"

You can use "-ldflags" instead of go generate/embed if that approach works better for you.
*/
package appbom

//go:generate go run gobinhasher.go

import (
	"fmt"
	"path/filepath"
	"runtime/debug"
)

// Known Settings key names
const (
	SettingBuildMode = "-buildmode" // the buildmode flag used
	SettingCompiler  = "-compiler"  // the compiler toolchain flag used
	SettingTags      = "-tags"
	SettingTrimPath  = "-trimpath"
	SettingLdFlags   = "-ldflags"
	SettingMod       = "-mod"

	SettingEnvVarCgoEnabled  = "CGO_ENABLED"  // the effective CGO_ENABLED environment variable
	SettingEnvVarCgoCFlags   = "CGO_CFLAGS"   // the effective CGO_CFLAGS environment variable
	SettingEnvVarCgoCppFlags = "CGO_CPPFLAGS" // the effective CGO_CPPFLAGS environment variable
	SettingEnvVarCgoCxxFlags = "CGO_CXXFLAGS" // the effective CGO_CXXFLAGS environment variable
	SettingEnvVarCgoLdFlags  = "CGO_LDFLAGS"  // the effective CGO_LDFLAGS environment variable
	SettingEnvVarGoOs        = "GOOS"         // the operating system target
	SettingEnvVarGoArch      = "GOARCH"       // the architecture target

	// the architecture feature level for GOARCH
	SettingEnvVarGoAmd64  = "GOAMD64"
	SettingEnvVarGoArm64  = "GOARM64"
	SettingEnvVarGoArm    = "GOARM"
	SettingEnvVarGo386    = "GO386"
	SettingEnvVarGoPpc64  = "GOPPC64"
	SettingEnvVarGoMips   = "GOMIPS"
	SettingEnvVarGoMips64 = "GOMIPS64"
	SettingEnvVarGoWasm   = "GOWASM"

	SettingVcsType     = "vcs"          // the version control system for the source tree where the build ran
	SettingVcsRevision = "vcs.revision" // the revision identifier for the current commit or checkout
	SettingVcsTime     = "vcs.time"     // the modification time associated with vcs.revision, in RFC3339 format
	SettingVcsModified = "vcs.modified" // true or false indicating whether the source tree had local modifications
)

//todo: expose the setting description as a 'details'/'summary' field

type MainPackageInfo struct {
	Path    string `json:"path"` //todo: add 'main'
	Version string `json:"version"`
}

type PackageInfo struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Hash       string `json:"hash"` // 'h1' algo is sha256: https://go.dev/ref/mod#go-sum-files
	Path       string `json:"path"`
	ReplacedBy string `json:"replaced_by,omitempty"`
}

type SourceControlInfo struct {
	Type            string `json:"type"`
	Revision        string `json:"revision"`
	RevisionTime    string `json:"revision_time"`
	HasLocalChanges bool   `json:"has_local_changes"`
}

type ParamType string

const (
	PTFlag   ParamType = "flag"
	PTEnvVar           = "envvar"
)

type ParamName string

const (
	PNBuildMode   = ParamName(SettingBuildMode)
	PNCompiler    = ParamName(SettingCompiler)
	PNTags        = ParamName(SettingTags)
	PNTrimPath    = ParamName(SettingTrimPath)
	PNLdFlags     = ParamName(SettingLdFlags)
	PNMod         = ParamName(SettingMod)
	PNCgoEnabled  = ParamName(SettingEnvVarCgoEnabled)
	PNCgoCFlags   = ParamName(SettingEnvVarCgoCFlags)
	PNCgoCppFlags = ParamName(SettingEnvVarCgoCppFlags)
	PNCgoCxxFlags = ParamName(SettingEnvVarCgoCxxFlags)
	PNCgoLdFlags  = ParamName(SettingEnvVarCgoLdFlags)
	PNGoOs        = ParamName(SettingEnvVarGoOs)
	PNGoArch      = ParamName(SettingEnvVarGoArch)

	PNGoAmd64  = ParamName(SettingEnvVarGoAmd64)
	PNGoArm64  = ParamName(SettingEnvVarGoArm64)
	PNGoArm    = ParamName(SettingEnvVarGoArm)
	PNGo386    = ParamName(SettingEnvVarGo386)
	PNGoPpc64  = ParamName(SettingEnvVarGoPpc64)
	PNGoMips   = ParamName(SettingEnvVarGoMips)
	PNGoMips64 = ParamName(SettingEnvVarGoMips64)
	PNGoWasm   = ParamName(SettingEnvVarGoWasm)
)

type ParamInfo struct {
	Name        ParamName `json:"name"`
	Type        ParamType `json:"type"`
	Value       string    `json:"value"`
	Description string    `json:"description,omitempty"`
}

type BuildParams struct {
	BuildMode   *ParamInfo `json:"build_mode,omitempty"`
	Compiler    *ParamInfo `json:"compiler,omitempty"`
	CgoEnabled  *ParamInfo `json:"cgo_enabled,omitempty"`
	CgoCFlags   *ParamInfo `json:"cgo_cflags,omitempty"`
	CgoCppFlags *ParamInfo `json:"cgo_cppflags,omitempty"`
	CgoCxxFlags *ParamInfo `json:"cgo_cxxflags,omitempty"`
	CgoLdFlags  *ParamInfo `json:"cgo_ldflags,omitempty"`
	Os          *ParamInfo `json:"os,omitempty"`
	Arch        *ParamInfo `json:"arch,omitempty"`
	ArchFeature *ParamInfo `json:"arch_feature,omitempty"`
}

type Info struct {
	BuilderHash   string             `json:"builder_hash,omitempty"`
	Runtime       string             `json:"runtime"`
	Entrypoint    MainPackageInfo    `json:"entrypoint"`
	BuildParams   BuildParams        `json:"build_params"`
	OtherParams   map[string]string  `json:"other_params,omitempty"`
	SourceControl *SourceControlInfo `json:"source_control,omitempty"`
	Includes      []*PackageInfo     `json:"includes,omitempty"`
}

func Get() *Info {
	raw, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}

	info := &Info{
		BuilderHash: goBinHash,
		Runtime:     raw.GoVersion,
		OtherParams: map[string]string{},
	}

	if raw.Path != "" {
		if raw.Path == "command-line-arguments" {
			info.Entrypoint.Path = "unknown.prefix/main"
		} else {
			info.Entrypoint.Path = fmt.Sprintf("%s/main", raw.Path)
		}
	}

	if raw.Main.Path != "" {
		info.Entrypoint.Path = fmt.Sprintf("%s/main", raw.Main.Path)
	}

	info.Entrypoint.Version = raw.Main.Version

	for _, kv := range raw.Settings {
		switch kv.Key {
		case SettingVcsType:
			if info.SourceControl == nil {
				info.SourceControl = &SourceControlInfo{}
			}

			info.SourceControl.Type = kv.Value
		case SettingVcsRevision:
			if info.SourceControl == nil {
				info.SourceControl = &SourceControlInfo{}
			}

			info.SourceControl.Revision = kv.Value
		case SettingVcsTime:
			if info.SourceControl == nil {
				info.SourceControl = &SourceControlInfo{}
			}

			info.SourceControl.RevisionTime = kv.Value
		case SettingVcsModified:
			if info.SourceControl == nil {
				info.SourceControl = &SourceControlInfo{}
			}

			switch kv.Value {
			case "true", "TRUE":
				info.SourceControl.HasLocalChanges = true
			}

		case SettingBuildMode:
			info.BuildParams.BuildMode = &ParamInfo{
				Name:  PNBuildMode,
				Type:  PTFlag,
				Value: kv.Value,
			}
		case SettingCompiler:
			info.BuildParams.Compiler = &ParamInfo{
				Name:  PNCompiler,
				Type:  PTFlag,
				Value: kv.Value,
			}
		case SettingTags:
			info.BuildParams.Compiler = &ParamInfo{
				Name:  PNTags,
				Type:  PTFlag,
				Value: kv.Value,
			}
		case SettingTrimPath:
			info.BuildParams.Compiler = &ParamInfo{
				Name:  PNTrimPath,
				Type:  PTFlag,
				Value: kv.Value,
			}

		case SettingLdFlags:
			info.BuildParams.Compiler = &ParamInfo{
				Name:  PNLdFlags,
				Type:  PTFlag,
				Value: kv.Value,
			}
		case SettingMod:
			info.BuildParams.Compiler = &ParamInfo{
				Name:  PNMod,
				Type:  PTFlag,
				Value: kv.Value,
			}
		case SettingEnvVarCgoEnabled:
			info.BuildParams.CgoEnabled = &ParamInfo{
				Name:  PNCgoEnabled,
				Type:  PTEnvVar,
				Value: kv.Value,
			}
		case SettingEnvVarCgoCFlags:
			info.BuildParams.CgoCFlags = &ParamInfo{
				Name:  PNCgoCFlags,
				Type:  PTEnvVar,
				Value: kv.Value,
			}
		case SettingEnvVarCgoCppFlags:
			info.BuildParams.CgoCppFlags = &ParamInfo{
				Name:  PNCgoCppFlags,
				Type:  PTEnvVar,
				Value: kv.Value,
			}
		case SettingEnvVarCgoCxxFlags:
			info.BuildParams.CgoCxxFlags = &ParamInfo{
				Name:  PNCgoCxxFlags,
				Type:  PTEnvVar,
				Value: kv.Value,
			}
		case SettingEnvVarCgoLdFlags:
			info.BuildParams.CgoLdFlags = &ParamInfo{
				Name:  PNCgoLdFlags,
				Type:  PTEnvVar,
				Value: kv.Value,
			}
		case SettingEnvVarGoOs:
			info.BuildParams.Os = &ParamInfo{
				Name:  PNGoOs,
				Type:  PTEnvVar,
				Value: kv.Value,
			}
		case SettingEnvVarGoArch:
			info.BuildParams.Arch = &ParamInfo{
				Name:  PNGoArch,
				Type:  PTEnvVar,
				Value: kv.Value,
			}

		// the architecture feature level for GOARCH
		case SettingEnvVarGoAmd64:
			info.BuildParams.ArchFeature = &ParamInfo{
				Name:  PNGoAmd64,
				Type:  PTEnvVar,
				Value: kv.Value,
			}
		case SettingEnvVarGoArm64:
			info.BuildParams.ArchFeature = &ParamInfo{
				Name:  PNGoArm64,
				Type:  PTEnvVar,
				Value: kv.Value,
			}
		case SettingEnvVarGoArm:
			info.BuildParams.ArchFeature = &ParamInfo{
				Name:  PNGoArm,
				Type:  PTEnvVar,
				Value: kv.Value,
			}
		case SettingEnvVarGo386:
			info.BuildParams.ArchFeature = &ParamInfo{
				Name:  PNGo386,
				Type:  PTEnvVar,
				Value: kv.Value,
			}
		case SettingEnvVarGoPpc64:
			info.BuildParams.ArchFeature = &ParamInfo{
				Name:  PNGoPpc64,
				Type:  PTEnvVar,
				Value: kv.Value,
			}
		case SettingEnvVarGoMips:
			info.BuildParams.ArchFeature = &ParamInfo{
				Name:  PNGoMips,
				Type:  PTEnvVar,
				Value: kv.Value,
			}
		case SettingEnvVarGoMips64:
			info.BuildParams.ArchFeature = &ParamInfo{
				Name:  PNGoMips64,
				Type:  PTEnvVar,
				Value: kv.Value,
			}
		case SettingEnvVarGoWasm:
			info.BuildParams.ArchFeature = &ParamInfo{
				Name:  PNGoWasm,
				Type:  PTEnvVar,
				Value: kv.Value,
			}

		default:
			info.OtherParams[kv.Key] = kv.Value
		}
	}

	includeMap := map[string]*PackageInfo{}
	for _, depData := range raw.Deps {
		if _, found := includeMap[depData.Path]; found {
			continue
		}

		pkg := &PackageInfo{
			Name:    filepath.Base(depData.Path),
			Path:    depData.Path,
			Version: depData.Version,
			Hash:    depData.Sum,
		}

		includeMap[depData.Path] = pkg

		for depData.Replace != nil {
			pkg.ReplacedBy = depData.Replace.Path

			if _, found := includeMap[depData.Replace.Path]; found {
				break
			}

			pkg = &PackageInfo{
				Name:    filepath.Base(depData.Replace.Path),
				Path:    depData.Replace.Path,
				Version: depData.Replace.Version,
				Hash:    depData.Replace.Sum,
			}

			includeMap[depData.Replace.Path] = pkg

			depData.Replace = depData.Replace.Replace
		}
	}

	for _, pkg := range includeMap {
		info.Includes = append(info.Includes, pkg)
	}

	return info
}

//TODO: have an option to add package metadata (from deps.dev and osv.dev)
