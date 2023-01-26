// Package instruction describes the Docker instruction data model.
package instruction

import (
	"strings"
)

// All supported instruction names
const (
	Add         = "add"
	Arg         = "arg"
	Cmd         = "cmd"
	Copy        = "copy"
	Entrypoint  = "entrypoint"
	Env         = "env"
	Expose      = "expose"
	From        = "from"
	Healthcheck = "healthcheck"
	Label       = "label"
	Maintainer  = "maintainer"
	Onbuild     = "onbuild"
	Run         = "run"
	Shell       = "shell"
	StopSignal  = "stopsignal"
	User        = "user"
	Volume      = "volume"
	Workdir     = "workdir"
)

type Field struct {
	GlobalIndex int      `json:"start_index"`
	StageIndex  int      `json:"stage_index"`
	StageID     int      `json:"stage_id"`
	RawData     string   `json:"-"`
	RawLines    []string `json:"raw_lines"`
	StartLine   int      `json:"start_line"`
	EndLine     int      `json:"end_line"`
	Name        string   `json:"name"`
	Flags       []string `json:"flags,omitempty"`
	Args        []string `json:"args,omitempty"`
	ArgsRaw     string   `json:"args_raw,omitempty"`
	IsJSONForm  bool     `json:"is_json"`
	IsOnBuild   bool     `json:"is_onbuild,omitempty"`
	IsValid     bool     `json:"is_valid"`
	Errors      []string `json:"errors,omitempty"`
}

type Format struct {
	Name               string
	SupportsFlags      bool //todo: add a list allowed flags
	SupportsJSONForm   bool
	SupportsNameValues bool
	RequiresNameValues bool
	SupportsSubInst    bool
	IsDepricated       bool
}

// Specs is a map of all available instructions and their format info (by name)
var Specs = map[string]Format{
	Add: {
		Name:             Add,
		SupportsFlags:    true,
		SupportsJSONForm: true,
	},
	Arg: {
		Name:               Arg,
		SupportsNameValues: true,
	},
	Cmd: {
		Name:             Cmd,
		SupportsJSONForm: true,
	},
	Copy: {
		Name:             Copy,
		SupportsFlags:    true,
		SupportsJSONForm: true,
	},
	Entrypoint: {
		Name:             Entrypoint,
		SupportsJSONForm: true,
	},
	Env: {
		Name:               Env,
		RequiresNameValues: true,
	},
	Expose: {
		Name: Expose,
	},
	From: {
		Name:          From,
		SupportsFlags: true,
	},
	Healthcheck: {
		Name:             Healthcheck,
		SupportsJSONForm: true,
	},
	Label: {
		Name:               Label,
		RequiresNameValues: true,
	},
	Maintainer: {
		Name:         Maintainer,
		IsDepricated: true,
	},
	Onbuild: {
		Name:            Label,
		SupportsSubInst: true,
	},
	Run: {
		Name:             Run,
		SupportsJSONForm: true,
	},
	Shell: {
		Name:             Shell,
		SupportsJSONForm: true,
	},
	StopSignal: {
		Name: StopSignal,
	},
	User: {
		Name: User,
	},
	Volume: {
		Name:             Volume,
		SupportsJSONForm: true,
	},
	Workdir: {
		Name: Workdir,
	},
}

func IsKnown(name string) bool {
	name = strings.ToLower(name)
	_, ok := Specs[name]
	return ok
}

func SupportsJSONForm() []string {
	var names []string
	for _, spec := range Specs {
		names = append(names, spec.Name)
	}

	return names
}
