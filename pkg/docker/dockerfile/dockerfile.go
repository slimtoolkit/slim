package dockerfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/slimtoolkit/slim/pkg/consts"
	v "github.com/slimtoolkit/slim/pkg/version"
)

// note: dup (todo: refactor)
const (
	//MAINTAINER:
	instPrefixMaintainer = "MAINTAINER "
	//ENTRYPOINT:
	instTypeEntrypoint   = "ENTRYPOINT"
	instPrefixEntrypoint = "ENTRYPOINT "
	//CMD:
	instTypeCmd   = "CMD"
	instPrefixCmd = "CMD "
	//USER:
	instTypeUser   = "USER"
	instPrefixUser = "USER "
	//EXPOSE:
	instTypeExpose   = "EXPOSE"
	instPrefixExpose = "EXPOSE "
	//WORKDIR:
	instTypeWorkdir   = "WORKDIR"
	instPrefixWorkdir = "WORKDIR "
	//HEALTHCHECK:
	instTypeHealthcheck   = "HEALTHCHECK"
	instPrefixHealthcheck = "HEALTHCHECK "
	//ONBUILD:
	instTypeOnbuild = "ONBUILD"
	//RUN:
	instTypeRun   = "RUN"
	instPrefixRun = "RUN "
	//ADD:
	instTypeAdd = "ADD"
	//COPY:
	instTypeCopy = "COPY"
)

// GenerateFromInfo builds and saves a Dockerfile file object
func GenerateFromInfo(
	location string,
	volumes map[string]struct{},
	workingDir string,
	env []string,
	labels map[string]string,
	user string,
	exposedPorts map[docker.Port]struct{},
	entrypoint []string,
	cmd []string,
	hasData bool,
	tarData bool) error {

	dockerfileLocation := filepath.Join(location, "Dockerfile")

	var dfData bytes.Buffer
	dfData.WriteString("FROM scratch\n")

	dsInfoLabel := fmt.Sprintf("LABEL %s=\"%s\"\n", consts.DSLabelVersion, v.Current())
	dfData.WriteString(dsInfoLabel)

	if len(labels) > 0 {
		for name, value := range labels {
			var encoded bytes.Buffer
			encoder := json.NewEncoder(&encoded)
			encoder.SetEscapeHTML(false)
			encoder.Encode(value)
			labelInfo := fmt.Sprintf("LABEL %s=%s\n", name, encoded.String())
			dfData.WriteString(labelInfo)
		}
		dfData.WriteByte('\n')
	}

	if len(env) > 0 {
		for _, envInfo := range env {
			if envParts := strings.SplitN(envInfo, "=", 2); len(envParts) > 1 {
				dfData.WriteString("ENV ")
				envLine := fmt.Sprintf("%s \"%s\"", envParts[0], envParts[1])
				dfData.WriteString(envLine)
				dfData.WriteByte('\n')
			}
		}
		dfData.WriteByte('\n')
	}

	if len(volumes) > 0 {
		var volumeList []string
		for volumeName := range volumes {
			volumeList = append(volumeList, strconv.Quote(volumeName))
		}

		volumeInst := fmt.Sprintf("VOLUME [%s]", strings.Join(volumeList, ","))
		dfData.WriteString(volumeInst)
		dfData.WriteByte('\n')
	}

	if hasData {
		addData := "COPY files /\n"
		if tarData {
			addData = "ADD files.tar /\n"
		}

		dfData.WriteString(addData)
	}

	if workingDir != "" {
		dfData.WriteString(instPrefixWorkdir)
		dfData.WriteString(workingDir)
		dfData.WriteByte('\n')
	}

	if user != "" {
		dfData.WriteString(instPrefixUser)
		dfData.WriteString(user)
		dfData.WriteByte('\n')
	}

	if len(exposedPorts) > 0 {
		for portInfo := range exposedPorts {
			dfData.WriteString(instPrefixExpose)
			dfData.WriteString(string(portInfo))
			dfData.WriteByte('\n')
		}
	}

	if len(entrypoint) > 0 {
		//TODO: need to make sure the generated ENTRYPOINT is compatible with the original behavior
		var quotedEntryPoint []string
		for idx := range entrypoint {
			quotedEntryPoint = append(quotedEntryPoint, strconv.Quote(entrypoint[idx]))
		}

		dfData.WriteString("ENTRYPOINT [")
		dfData.WriteString(strings.Join(quotedEntryPoint, ","))
		dfData.WriteByte(']')
		dfData.WriteByte('\n')
	}

	if len(cmd) > 0 {
		//TODO: need to make sure the generated CMD is compatible with the original behavior
		var quotedCmd []string
		for idx := range cmd {
			quotedCmd = append(quotedCmd, strconv.Quote(cmd[idx]))
		}
		dfData.WriteString("CMD [")
		dfData.WriteString(strings.Join(quotedCmd, ","))
		dfData.WriteByte(']')
		dfData.WriteByte('\n')
	}

	return os.WriteFile(dockerfileLocation, dfData.Bytes(), 0644)
}
