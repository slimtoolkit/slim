package artifact

import (
	"strings"
)

var ShellNames = []string{
	"bash",
	"sh",
}

var ShellCommands = []string{
	"ls",
	"pwd",
	"cd",
	"ps",
	"head",
	"tail",
	"cat",
	"more",
	"find",
	"grep",
	"awk",
	"env",
}

var FilteredPaths = map[string]struct{}{
	"/":     {},
	"/proc": {},
	"/sys":  {},
	"/dev":  {},
}

var FileteredPathPrefixList = []string{
	"/proc/",
	"/sys/",
	"/dev/",
}

func IsFilteredPath(name string) bool {
	switch name {
	case "/", "/proc", "/sys", "/dev":
		return true
	}

	for _, prefix := range FileteredPathPrefixList {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	return false
}
