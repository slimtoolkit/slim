package system

import (
	"bufio"
	"bytes"
	"os"
	"strings"

	"github.com/slimtoolkit/slim/pkg/util/fsutil"
)

const (
	OSShellsFile = "/etc/shells"
)

// Common linux shell exe paths
const (
	OSShellAshExePath    = "/bin/ash"
	OSShellBourneExePath = "/bin/sh"
	OSShellKornExePath   = "/bin/ksh93"
	//usually a link to /bin/ksh93
	OSShellRestrictedKornExePath = "/bin/rksh93"
	//usually a link to /etc/alternatives/ksh
	//which links to /bin/ksh93
	OSShellKorn2ExePath = "/bin/ksh"
	OSShellBashExePath  = "/bin/bash"
	//usually a link to /bin/bash
	OSShellRestrictedBashExePath = "/bin/rbash"
	OSShellDashExePath           = "/bin/dash"
	OSShellZExePath              = "/bin/zsh"
	//usually a link to /bin/zsh
	OSShellZ2ExePath       = "/usr/bin/zsh"
	OSShellCExePath        = "/bin/csh"
	OSShellTCExePath       = "/bin/tcsh"
	OSShellFishExePath     = "/usr/bin/fish"
	OSShellElvishExePath   = "/usr/bin/elvish"
	OSShellXonshExePath    = "/usr/bin/xonsh"
	OSShellYashExePath     = "/usr/bin/yash"
	OSShellGitShellExePath = "/usr/bin/git-shell"
	OSShellScreenExePath   = "/usr/bin/screen"
	OSShellTmuxExePath     = "/usr/bin/tmux"
)

var OSShellReferences = map[string]string{
	OSShellBourneExePath:         OSShellBourneExePath,
	OSShellKornExePath:           OSShellKornExePath,
	OSShellKorn2ExePath:          OSShellKornExePath,
	OSShellRestrictedKornExePath: OSShellKornExePath,
	OSShellBashExePath:           OSShellBashExePath,
	OSShellRestrictedBashExePath: OSShellBashExePath,
	OSShellDashExePath:           OSShellDashExePath,
	OSShellZExePath:              OSShellZExePath,
	OSShellZ2ExePath:             OSShellZExePath,
	OSShellCExePath:              OSShellCExePath,
	OSShellTCExePath:             OSShellTCExePath,
	OSShellFishExePath:           OSShellFishExePath,
	OSShellElvishExePath:         OSShellElvishExePath,
	OSShellXonshExePath:          OSShellXonshExePath,
	OSShellYashExePath:           OSShellYashExePath,
	OSShellAshExePath:            OSShellAshExePath,
	OSShellGitShellExePath:       OSShellGitShellExePath,
	OSShellScreenExePath:         OSShellScreenExePath,
	OSShellTmuxExePath:           OSShellTmuxExePath,
}

var OSShells = map[string]OSShell{
	OSShellBourneExePath: {
		FullName:  "Bourne shell",
		ShortName: "sh",
		ExePath:   OSShellBourneExePath,
	},
	OSShellKornExePath: {
		FullName:  "Korn shell",
		ShortName: "ksh",
		ExePath:   OSShellKornExePath,
	},
	OSShellBashExePath: {
		FullName:  "Bash shell",
		ShortName: "bash",
		ExePath:   OSShellBashExePath,
	},
	OSShellDashExePath: {
		FullName:  "Dash shell",
		ShortName: "dash",
		ExePath:   OSShellDashExePath,
	},
	OSShellZExePath: {
		FullName:  "Z shell",
		ShortName: "zsh",
		ExePath:   OSShellZExePath,
	},
	OSShellCExePath: {
		FullName:  "C shell",
		ShortName: "csh",
		ExePath:   OSShellCExePath,
	},
	OSShellTCExePath: {
		FullName:  "TC shell",
		ShortName: "tcsh",
		ExePath:   OSShellTCExePath,
	},
	OSShellFishExePath: {
		FullName:  "Fish shell",
		ShortName: "fish",
		ExePath:   OSShellFishExePath,
	},
	OSShellElvishExePath: {
		FullName:  "Elvish shell",
		ShortName: "elvish",
		ExePath:   OSShellElvishExePath,
	},
	OSShellXonshExePath: {
		FullName:  "Xonsh shell",
		ShortName: "xonsh",
		ExePath:   OSShellXonshExePath,
	},
	OSShellYashExePath: {
		FullName:  "Yash shell",
		ShortName: "yash",
		ExePath:   OSShellYashExePath,
	},
	OSShellAshExePath: {
		FullName:  "Ash shell",
		ShortName: "ash",
		ExePath:   OSShellAshExePath,
	},
	//Restricted login shell for Git-only SSH access
	//installed with git
	OSShellGitShellExePath: {
		FullName:  "Git shell",
		ShortName: "git-shell",
		ExePath:   OSShellGitShellExePath,
	},
	OSShellScreenExePath: {
		FullName:  "Screen",
		ShortName: "screen",
		ExePath:   OSShellScreenExePath,
	},
	OSShellTmuxExePath: {
		FullName:  "Tmux",
		ShortName: "tmux",
		ExePath:   OSShellTmuxExePath,
	},
}

type OSShell struct {
	FullName  string `json:"full_name"`
	ShortName string `json:"short_name,omitempty"`
	ExePath   string `json:"exe_path"`
	LinkPath  string `json:"link_path,omitempty"`
	Reference string `json:"reference,omitempty"`
	Verified  bool   `json:"verified,omitempty"`
}

func IsOSShellsFile(name string) bool {
	if name == OSShellsFile {
		return true
	}

	return false
}

func IsShellExePath(name string) bool {
	_, found := OSShellReferences[name]
	if found {
		return true
	}

	return false
}

func LookupShellByExePath(name string) *OSShell {
	mainPath, found := OSShellReferences[name]
	if !found {
		return nil
	}

	if info, found := OSShells[mainPath]; found {
		if name != mainPath {
			info.Reference = name
		}

		return &info
	}

	return nil
}

func parseOSShellsLine(line string) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "#") {
		return ""
	}

	return line
}

func ParseOSShells(raw []byte) []*OSShell {
	var shells []*OSShell
	r := bytes.NewReader(raw)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		exePath := parseOSShellsLine(scanner.Text())
		if exePath != "" {
			shellInfo := LookupShellByExePath(exePath)
			if shellInfo != nil {
				info := *shellInfo
				shells = append(shells, &info)
			} else {
				unknown := &OSShell{
					FullName: "Unknown shell",
					ExePath:  exePath,
				}

				shells = append(shells, unknown)
			}
		}
	}

	return shells
}

func NewOSShellsFromData(raw []byte) ([]*OSShell, error) {
	shells := ParseOSShells(raw)
	return shells, nil
}

func NewOSShells(verify bool) ([]*OSShell, error) {
	raw, err := os.ReadFile(OSShellsFile)
	if err == nil {
		return nil, err
	}

	result, err := NewOSShellsFromData(raw)
	if err != nil {
		return nil, err
	}

	if verify {
		for _, info := range result {
			if fsutil.Exists(info.ExePath) &&
				!fsutil.IsDir(info.ExePath) {
				info.Verified = true

				if fsutil.IsSymlink(info.ExePath) {
					if linkRef, err := os.Readlink(info.ExePath); err == nil {
						info.LinkPath = linkRef
					}
				}
			}
		}
	}

	return result, nil
}
