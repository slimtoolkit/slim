package report

// DefaultContainerReportFileName is the default Monitor Data Event Log file name
const DefaultMonDelFileName = "mondel.ndjson"

// Event source
const (
	MDESourceDel = ".del" //Data Event Logger event
	MDESourceFan = "m.fa" //FaNotify monitor event
	MDESourcePT  = "m.pt" //PTrace monitor event
)

// Event types
const (
	MDETypeArtifact = "a" //Artifact event type
	MDETypeProcess  = "p" //Process event type
	MDETypeState    = "s" //State event
)

// Operation types
const (
	OpTypeRead  = "r"
	OpTypeWrite = "w"
	OpTypeExec  = "x"
	OpTypeCheck = "c"
)

type MonitorDataEvent struct {
	Timestamp int64  `json:"ts"`
	SeqNumber uint64 `json:"sn"`
	Source    string `json:"s"`
	Type      string `json:"t"`
	Pid       int32  `json:"p,omitempty"`
	ParentPid int32  `json:"pp,omitempty"`
	Artifact  string `json:"a,omitempty"`  // used for exe path for process events
	OpType    string `json:"o,omitempty"`  // operation type
	Op        string `json:"op,omitempty"` // operation
	OpNum     uint32 `json:"n,omitempty"`
	WorkDir   string `json:"w,omitempty"`
	Root      string `json:"r,omitempty"`
	Cmd       string `json:"c,omitempty"`
	State     string `json:"st,omitempty"`
}
