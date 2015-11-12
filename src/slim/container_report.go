package main

import (
	"bytes"
	"encoding/json"
	"os"
)

//TODO: REFACTOR :)

type artifactType int

const (
	dirArtifactType     = 1
	fileArtifactType    = 2
	symlinkArtifactType = 3
	unknownArtifactType = 99
)

var artifactTypeNames = map[artifactType]string{
	dirArtifactType:     "Dir",
	fileArtifactType:    "File",
	symlinkArtifactType: "Symlink",
	unknownArtifactType: "Unknown",
}

func (t artifactType) String() string {
	return artifactTypeNames[t]
}

var artifactTypeValues = map[string]artifactType{
	"Dir":     dirArtifactType,
	"File":    fileArtifactType,
	"Symlink": symlinkArtifactType,
	"Unknown": unknownArtifactType,
}

func getArtifactTypeValue(s string) artifactType {
	return artifactTypeValues[s]
}

type processInfo struct {
	Pid       int32  `json:"pid"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Cmd       string `json:"cmd"`
	Cwd       string `json:"cwd"`
	Root      string `json:"root"`
	ParentPid int32  `json:"ppid"`
}

type fileInfo struct {
	EventCount   uint32 `json:"event_count"`
	FirstEventID uint32 `json:"first_eid"`
	Name         string `json:"-"`
	ReadCount    uint32 `json:"reads,omitempty"`
	WriteCount   uint32 `json:"writes,omitempty"`
	ExeCount     uint32 `json:"execs,omitempty"`
}

type fanMonitorReport struct {
	MonitorPid       int                             `json:"monitor_pid"`
	MonitorParentPid int                             `json:"monitor_ppid"`
	EventCount       uint32                          `json:"event_count"`
	MainProcess      *processInfo                    `json:"main_process"`
	Processes        map[string]*processInfo         `json:"processes"`
	ProcessFiles     map[string]map[string]*fileInfo `json:"process_files"`
}

type syscallStatInfo struct {
	Number uint64 `json:"num"`
	Name   string `json:"name"`
	Count  uint64 `json:"count"`
}

type ptMonitorReport struct {
	SyscallCount uint64                     `json:"syscall_count"`
	SyscallNum   uint32                     `json:"syscall_num"`
	SyscallStats map[string]syscallStatInfo `json:"syscall_stats"`
}

type artifactProps struct {
	FileType artifactType    `json:"-"` //todo
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

func (p *artifactProps) UnmarshalJSON(data []byte) error {
	type artifactPropsType artifactProps
	props := &struct {
		FileTypeStr string `json:"file_type"`
		*artifactPropsType
	}{
		artifactPropsType: (*artifactPropsType)(p),
	}

	if err := json.Unmarshal(data, &props); err != nil {
		return err
	}
	p.FileType = getArtifactTypeValue(props.FileTypeStr)

	return nil
}

type imageReport struct {
	Files []*artifactProps `json:"files"`
}

type monitorReports struct {
	Fan *fanMonitorReport `json:"fan"`
	Pt *ptMonitorReport `json:"pt"`
}

type containerReport struct {
	Monitors monitorReports `json:"monitors"`
	Image   imageReport     `json:"image"`
}

func permSetFromFlags(flags map[string]bool) string {
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
