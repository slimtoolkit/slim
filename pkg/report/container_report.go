package report

import (
	"bytes"
	"encoding/json"
	"os"
)

// ArtifactType is an artifact type ID
type ArtifactType int

// Artifact type ID constants
const (
	DirArtifactType     ArtifactType = 1
	FileArtifactType    ArtifactType = 2
	SymlinkArtifactType ArtifactType = 3
	UnknownArtifactType ArtifactType = 99
)

const (
	DirArtifactTypeName        = "dir"
	FileArtifactTypeName       = "file"
	SymlinkArtifactTypeName    = "symlink"
	HardlinkArtifactTypeName   = "hardlink"
	UnknownArtifactTypeName    = "unknown"
	UnexpectedArtifactTypeName = "unexpected"
)

// DefaultContainerReportFileName is the default container report file name
const DefaultContainerReportFileName = "creport.json"

var artifactTypeNames = map[ArtifactType]string{
	DirArtifactType:     DirArtifactTypeName,
	FileArtifactType:    FileArtifactTypeName,
	SymlinkArtifactType: SymlinkArtifactTypeName,
	UnknownArtifactType: UnknownArtifactTypeName,
}

// String converts the artifact type ID to a string
func (t ArtifactType) String() string {
	return artifactTypeNames[t]
}

var artifactTypeValues = map[string]ArtifactType{
	DirArtifactTypeName:     DirArtifactType,
	FileArtifactTypeName:    FileArtifactType,
	SymlinkArtifactTypeName: SymlinkArtifactType,
	UnknownArtifactTypeName: UnknownArtifactType,
}

// GetArtifactTypeValue maps an artifact type name to an artifact type ID
func GetArtifactTypeValue(s string) ArtifactType {
	return artifactTypeValues[s]
}

// ProcessInfo contains various process object metadata
type ProcessInfo struct {
	Pid       int32  `json:"pid"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Cmd       string `json:"cmd"`
	Cwd       string `json:"cwd"`
	Root      string `json:"root"`
	ParentPid int32  `json:"ppid"`
}

// FileInfo contains various file object and activity metadata
type FileInfo struct {
	EventCount   uint32 `json:"event_count"`
	FirstEventID uint32 `json:"first_eid"`
	Name         string `json:"-"`
	ReadCount    uint32 `json:"reads,omitempty"`
	WriteCount   uint32 `json:"writes,omitempty"`
	ExeCount     uint32 `json:"execs,omitempty"`
}

// FanMonitorReport is a file monitoring report
type FanMonitorReport struct {
	MonitorPid       int                             `json:"monitor_pid"`
	MonitorParentPid int                             `json:"monitor_ppid"`
	EventCount       uint32                          `json:"event_count"`
	MainProcess      *ProcessInfo                    `json:"main_process"`
	Processes        map[string]*ProcessInfo         `json:"processes"`
	ProcessFiles     map[string]map[string]*FileInfo `json:"process_files"`
}

// PeMonitorReport is a processing monitoring report
type PeMonitorReport struct {
	Children map[int][]int
	Parents  map[int]int
}

// SyscallStatInfo contains various system call activity metadata
type SyscallStatInfo struct {
	Number uint32 `json:"num"`
	Name   string `json:"name"`
	Count  uint64 `json:"count"`
}

// PtMonitorReport contains various process execution metadata
type PtMonitorReport struct {
	Enabled      bool                       `json:"enabled"`
	ArchName     string                     `json:"arch_name"`
	SyscallCount uint64                     `json:"syscall_count"`
	SyscallNum   uint32                     `json:"syscall_num"`
	SyscallStats map[string]SyscallStatInfo `json:"syscall_stats"`
	FSActivity   map[string]*FSActivityInfo `json:"fs_activity"`
}

type FSActivityInfo struct {
	OpsAll       uint64           `json:"ops_all"`
	OpsCheckFile uint64           `json:"ops_checkfile"`
	Syscalls     map[int]struct{} `json:"syscalls"`
	Pids         map[int]struct{} `json:"pids"`
	IsSubdir     bool             `json:"is_subdir"`
}

// ArtifactProps contains various file system artifact properties
type ArtifactProps struct {
	FileType   ArtifactType    `json:"-"` //todo
	FilePath   string          `json:"file_path"`
	Mode       os.FileMode     `json:"-"` //todo
	ModeText   string          `json:"mode"`
	LinkRef    string          `json:"link_ref,omitempty"`
	Flags      map[string]bool `json:"flags,omitempty"`
	DataType   string          `json:"data_type,omitempty"`
	FileSize   int64           `json:"file_size"`
	Sha1Hash   string          `json:"sha1_hash,omitempty"`
	AppType    string          `json:"app_type,omitempty"`
	FileInode  uint64          `json:"-"` //todo
	FSActivity *FSActivityInfo `json:"-"`
}

// UnmarshalJSON decodes artifact property data
func (p *ArtifactProps) UnmarshalJSON(data []byte) error {
	type artifactPropsType ArtifactProps
	props := &struct {
		FileTypeStr string `json:"file_type"`
		*artifactPropsType
	}{
		artifactPropsType: (*artifactPropsType)(p),
	}

	if err := json.Unmarshal(data, &props); err != nil {
		return err
	}
	p.FileType = GetArtifactTypeValue(props.FileTypeStr)

	return nil
}

// MarshalJSON encodes artifact property data
func (p *ArtifactProps) MarshalJSON() ([]byte, error) {
	type artifactPropsType ArtifactProps
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(
		&struct {
			FileTypeStr string `json:"file_type"`
			*artifactPropsType
		}{
			FileTypeStr:       p.FileType.String(),
			artifactPropsType: (*artifactPropsType)(p),
		})

	return out.Bytes(), err
}

// ImageReport contains image report fields
type ImageReport struct {
	Files []*ArtifactProps `json:"files"`
}

// MonitorReports contains monitoring report fields
type MonitorReports struct {
	Fan *FanMonitorReport `json:"fan"`
	Pt  *PtMonitorReport  `json:"pt"`
}

// SystemReport provides a basic system report for the container environment
type SystemReport struct {
	Type    string     `json:"type"`
	Release string     `json:"release"`
	Distro  DistroInfo `json:"distro"`
}

// SensorReport provides a basic sensor report for the container environment
type SensorReport struct {
	Version string   `json:"version"`
	Args    []string `json:"args"`
}

// StartCommandReport provides a basic start command report for the container environment
type StartCommandReport struct {
	AppName       string   `json:"app_name"`
	AppArgs       []string `json:"app_args,omitempty"`
	AppEntrypoint []string `json:"app_entrypoint,omitempty"`
	AppCmd        []string `json:"app_cmd,omitempty"`
	AppUser       string   `json:"app_user,omitempty"`
}

// ContainerReport contains container report fields
type ContainerReport struct {
	StartCommand *StartCommandReport `json:"start_command"`
	Sensor       *SensorReport       `json:"sensor"`
	System       SystemReport        `json:"system"`
	Monitors     MonitorReports      `json:"monitors"`
	Image        ImageReport         `json:"image"`
}

// PermSetFromFlags maps artifact flags to permissions
func PermSetFromFlags(flags map[string]bool) string {
	var b bytes.Buffer
	if flags["R"] {
		b.WriteString("r")
	}

	if flags["W"] {
		b.WriteString("w")
	}

	if flags["X"] {
		b.WriteString("ix")
	}

	return b.String()
}
