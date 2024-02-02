package command

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/slimtoolkit/slim/pkg/util/fsutil"
)

// Message errors
var (
	ErrUnknownMessage = errors.New("unknown command type")
)

const (
	ResponseStatusOk    = "ok"
	ResponseStatusError = "error"
)

// Response contains the command response status information
type Response struct {
	Status string `json:"status"`
}

// MessageName is a message ID type
type MessageName string

// Supported messages
const (
	StartMonitorName   MessageName = "cmd.monitor.start"
	StopMonitorName    MessageName = "cmd.monitor.stop"
	ShutdownSensorName MessageName = "cmd.sensor.shutdown"
)

// Message represents the message interface
type Message interface {
	GetName() MessageName
}

// StartMonitor contains the start monitor command fields
type StartMonitor struct {
	ObfuscateMetadata            bool                          `json:"obfuscate_metadata"`
	RTASourcePT                  bool                          `json:"rta_source_ptrace"`
	AppName                      string                        `json:"app_name"`
	AppArgs                      []string                      `json:"app_args,omitempty"`
	AppEntrypoint                []string                      `json:"app_entrypoint,omitempty"`
	AppCmd                       []string                      `json:"app_cmd,omitempty"`
	AppUser                      string                        `json:"app_user,omitempty"`
	AppStdoutToFile              bool                          `json:"app_stdout_to_file"`
	AppStderrToFile              bool                          `json:"app_stderr_to_file"`
	RunTargetAsUser              bool                          `json:"run_tas_user,omitempty"`
	ReportOnMainPidExit          bool                          `json:"report_on_main_pid_exit"`
	KeepPerms                    bool                          `json:"keep_perms,omitempty"`
	Perms                        map[string]*fsutil.AccessInfo `json:"perms,omitempty"`
	Excludes                     []string                      `json:"excludes,omitempty"`
	ExcludeVarLockFiles          bool                          `json:"exclude_varlock_files,omitempty"`
	Preserves                    map[string]*fsutil.AccessInfo `json:"preserves,omitempty"`
	Includes                     map[string]*fsutil.AccessInfo `json:"includes,omitempty"`
	IncludeBins                  []string                      `json:"include_bins,omitempty"`
	IncludeDirBinsList           map[string]*fsutil.AccessInfo `json:"include_dir_bins_list,omitempty"`
	IncludeExes                  []string                      `json:"include_exes,omitempty"`
	IncludeShell                 bool                          `json:"include_shell,omitempty"`
	IncludeWorkdir               string                        `json:"include_workdir,omitempty"`
	IncludeCertAll               bool                          `json:"include_cert_all,omitempty"`
	IncludeCertBundles           bool                          `json:"include_cert_bundles,omitempty"`
	IncludeCertDirs              bool                          `json:"include_cert_dirs,omitempty"`
	IncludeCertPKAll             bool                          `json:"include_cert_pk_all,omitempty"`
	IncludeCertPKDirs            bool                          `json:"include_cert_pk_dirs,omitempty"`
	IncludeNew                   bool                          `json:"include_new,omitempty"`
	IncludeSSHClient             bool                          `json:"include_ssh_client,omitempty"`
	IncludeOSLibsNet             bool                          `json:"include_oslibs_net,omitempty"`
	IncludeZoneInfo              bool                          `json:"include_zoneinfo,omitempty"`
	IncludeAppNuxtDir            bool                          `json:"include_app_nuxt_dir,omitempty"`
	IncludeAppNuxtBuildDir       bool                          `json:"include_app_nuxt_build,omitempty"`
	IncludeAppNuxtDistDir        bool                          `json:"include_app_nuxt_dist,omitempty"`
	IncludeAppNuxtStaticDir      bool                          `json:"include_app_nuxt_static,omitempty"`
	IncludeAppNuxtNodeModulesDir bool                          `json:"include_app_nuxt_nm,omitempty"`
	IncludeAppNextDir            bool                          `json:"include_app_next_dir,omitempty"`
	IncludeAppNextBuildDir       bool                          `json:"include_app_next_build,omitempty"`
	IncludeAppNextDistDir        bool                          `json:"include_app_next_dist,omitempty"`
	IncludeAppNextStaticDir      bool                          `json:"include_app_next_static,omitempty"`
	IncludeAppNextNodeModulesDir bool                          `json:"include_app_next_nm,omitempty"`
	IncludeNodePackages          []string                      `json:"include_node_packages,omitempty"`
}

// GetName returns the command message ID for the start monitor command
func (m *StartMonitor) GetName() MessageName {
	return StartMonitorName
}

// StopMonitor contains the stop monitor command fields
type StopMonitor struct {
}

// GetName returns the command message ID for the stop monitor command
func (m *StopMonitor) GetName() MessageName {
	return StopMonitorName
}

// ShutdownSensor contains the 'shutdown sensor' command fields
type ShutdownSensor struct{}

// GetName returns the command message ID for the 'shutdown sensor' command
func (m *ShutdownSensor) GetName() MessageName {
	return ShutdownSensorName
}

type messageWrapper struct {
	Name MessageName     `json:"name"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Encode encodes the message instance to a JSON buffer object
func Encode(m Message) ([]byte, error) {
	obj := messageWrapper{
		Name: m.GetName(),
	}

	switch v := m.(type) {
	case *StartMonitor:
		var b bytes.Buffer
		encoder := json.NewEncoder(&b)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(v); err != nil {
			return nil, err
		}

		obj.Data = b.Bytes()
	case *StopMonitor:
	case *ShutdownSensor:
	default:
		return nil, ErrUnknownMessage
	}

	return json.Marshal(&obj)
}

// Decode decodes JSON data into a message instance
func Decode(data []byte) (Message, error) {
	var wrapper messageWrapper
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}

	switch wrapper.Name {
	case StartMonitorName:
		var cmd StartMonitor
		if err := json.Unmarshal(wrapper.Data, &cmd); err != nil {
			return nil, err
		}

		return &cmd, nil
	case StopMonitorName:
		return &StopMonitor{}, nil
	case ShutdownSensorName:
		return &ShutdownSensor{}, nil
	default:
		return nil, ErrUnknownMessage
	}
}
