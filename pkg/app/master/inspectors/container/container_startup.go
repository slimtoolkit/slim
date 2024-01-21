package container

import (
	"strings"
)

var defaultShellFormPrefix = []string{"/bin/sh", "-c"}

func hasPrefixSlice(input []string, prefix []string) bool {
	if len(prefix) > len(input) {
		return false
	}

	for idx, val := range prefix {
		if input[idx] != val {
			return false
		}
	}

	return true
}

func BuildStartupCommand(
	entrypoint []string,
	cmd []string,
	shell []string,
	clearEntrypoint bool,
	newEntrypoint []string,
	clearCmd bool,
	newCmd []string) []string {
	var output []string

	if len(shell) == 0 {
		shell = defaultShellFormPrefix
	}

	// Dockerfile CMD and runtime cmd params
	// are ignored when entrypoint is in shell form
	entryIsShellForm := hasPrefixSlice(entrypoint, shell)

	//note: need to refactor this messy algorithm (keeping it as-is for now)...
	if !clearEntrypoint && !clearCmd && len(newEntrypoint) == 0 && len(newCmd) == 0 {
		//if entrypoint is in shell format then ignore cmd
		output = append(output, entrypoint...)
		if !entryIsShellForm {
			output = append(output, cmd...)
		}
	} else {
		if len(newEntrypoint) > 0 || clearEntrypoint {
			output = append(output, newEntrypoint...)

			if len(newCmd) > 0 {
				output = append(output, newCmd...)
			}
			//note: not using CMD from image if there's an override for ENTRYPOINT
		} else {
			output = append(output, entrypoint...)

			if len(newCmd) > 0 || clearCmd {
				output = append(output, newCmd...)
			} else {
				if !entryIsShellForm {
					output = append(output, cmd...)
				}
			}
		}
	}

	emptyIdx := -1
	for idx, val := range output {
		val = strings.TrimSpace(val)
		if val != "" {
			break
		}

		emptyIdx = idx
	}

	if emptyIdx > -1 {
		output = output[emptyIdx+1:]
	}

	return output
}
