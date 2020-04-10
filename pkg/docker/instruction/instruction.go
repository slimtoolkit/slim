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
	GlobalIndex int
	StageIndex  int
	StageID     int
	Raw         string
	StartLine   int
	EndLine     int
	Name        string
	Flags       []string
	Args        []string
	ArgsRaw     string
	IsJSONForm  bool
	IsOnBuild   bool
	IsValid     bool
	Errors      []string
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
