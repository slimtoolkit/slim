package report

import (
	"bytes"
	"encoding/json"
	"os"
)

//TODO: REFACTOR :)

type ArtifactType int

const (
	DirArtifactType     = 1
	FileArtifactType    = 2
	SymlinkArtifactType = 3
	UnknownArtifactType = 99
)

var artifactTypeNames = map[ArtifactType]string{
	DirArtifactType:     "Dir",
	FileArtifactType:    "File",
	SymlinkArtifactType: "Symlink",
	UnknownArtifactType: "Unknown",
}

func (t ArtifactType) String() string {
	return artifactTypeNames[t]
}

var artifactTypeValues = map[string]ArtifactType{
	"Dir":     DirArtifactType,
	"File":    FileArtifactType,
	"Symlink": SymlinkArtifactType,
	"Unknown": UnknownArtifactType,
}

func GetArtifactTypeValue(s string) ArtifactType {
	return artifactTypeValues[s]
}

type ProcessInfo struct {
	Pid       int32  `json:"pid"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Cmd       string `json:"cmd"`
	Cwd       string `json:"cwd"`
	Root      string `json:"root"`
	ParentPid int32  `json:"ppid"`
}

type FileInfo struct {
	EventCount   uint32 `json:"event_count"`
	FirstEventID uint32 `json:"first_eid"`
	Name         string `json:"-"`
	ReadCount    uint32 `json:"reads,omitempty"`
	WriteCount   uint32 `json:"writes,omitempty"`
	ExeCount     uint32 `json:"execs,omitempty"`
}

type FanMonitorReport struct {
	MonitorPid       int                             `json:"monitor_pid"`
	MonitorParentPid int                             `json:"monitor_ppid"`
	EventCount       uint32                          `json:"event_count"`
	MainProcess      *ProcessInfo                    `json:"main_process"`
	Processes        map[string]*ProcessInfo         `json:"processes"`
	ProcessFiles     map[string]map[string]*FileInfo `json:"process_files"`
}

type SyscallStatInfo struct {
	Number uint64 `json:"num"`
	Name   string `json:"name"`
	Count  uint64 `json:"count"`
}

type PtMonitorReport struct {
	SyscallCount uint64                     `json:"syscall_count"`
	SyscallNum   uint32                     `json:"syscall_num"`
	SyscallStats map[string]SyscallStatInfo `json:"syscall_stats"`
}

type ArtifactProps struct {
	FileType ArtifactType    `json:"-"` //todo
	FilePath string          `json:"file_path"`
	Mode     os.FileMode     `json:"-"` //todo
	ModeText string          `json:"mode"`
	LinkRef  string          `json:"link_ref,omitempty"`
	Flags    map[string]bool `json:"flags,omitempty"`
	DataType string          `json:"data_type,omitempty"`
	FileSize int64           `json:"file_size"`
	Sha1Hash string          `json:"sha1_hash,omitempty"`
	AppType  string          `json:"app_type,omitempty"`
}

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

type ImageReport struct {
	Files []*ArtifactProps `json:"files"`
}

type MonitorReports struct {
	Fan *FanMonitorReport `json:"fan"`
	Pt  *PtMonitorReport  `json:"pt"`
}

type ContainerReport struct {
	Monitors MonitorReports `json:"monitors"`
	Image    ImageReport    `json:"image"`
}

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
