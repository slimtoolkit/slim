// Package instruction describes the Docker instruction data model.
package instruction

import (
	"strings"
)

// All supported instruction names
var DOCKER_INSTRUCTION_NAMES map[string]bool = map[string]bool{
		"Add": true,
		"Arg": true,
		"Cmd": true,
		"Copy": true,
		"Entrypoint": true,
		"Env": true,
		"Expose": true,
		"From": true,
		"Healthcheck": true,
		"Label": true,
		"Maintainer": true,
		"Onbuild": true,
		"Run": true,
		"Shell": true,
		"StopSignal": true,
		"User": true,
		"Volume": true,
		"Workdir": true,
}
	

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


func IsKnown(name string) bool {
	name = strings.ToLower(name)
	_, ok := DOCKER_INSTRUCTION_NAMES[name]
		return ok
}

func SupportsJSONForm() []string {
	return DOCKER_INSTRUCTION_NAMES
}
