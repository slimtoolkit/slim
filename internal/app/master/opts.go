package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cloudimmunity/go-dockerclientx"
	"github.com/docker/go-connections/nat"
	"github.com/google/shlex"

	"github.com/docker-slim/docker-slim/internal/app/master/config"
)

//based on expose opt parsing in Docker
func parseDockerExposeOpt(values []string) (map[docker.Port]struct{}, error) {
	exposedPorts := map[docker.Port]struct{}{}

	for _, raw := range values {
		if strings.Contains(raw, ":") {
			return nil, fmt.Errorf("invalid EXPOSE format: %s", raw)
		}

		proto, ports := nat.SplitProtoPort(raw)
		startPort, endPort, err := nat.ParsePortRange(ports)
		if err != nil {
			return nil, fmt.Errorf("invalid port range in EXPOSE: %s / error: %s", raw, err)
		}

		for i := startPort; i <= endPort; i++ {
			portInfo, err := nat.NewPort(proto, strconv.FormatUint(i, 10))
			if err != nil {
				return nil, err
			}

			exposedPorts[docker.Port(portInfo)] = struct{}{}
		}
	}
	return exposedPorts, nil
}

func isOneSpace(value string) bool {
	if len(value) > 0 && utf8.RuneCountInString(value) == 1 {
		r, _ := utf8.DecodeRuneInString(value)
		if r != utf8.RuneError && unicode.IsSpace(r) {
			return true
		}
	}

	return false
}

var allImageOverrides = map[string]bool{
	"entrypoint": true,
	"cmd":        true,
	"workdir":    true,
	"env":        true,
	"expose":     true,
}

func parseImageOverrides(value string) map[string]bool {
	switch value {
	case "":
		return map[string]bool{}
	case "all":
		return allImageOverrides
	default:
		parts := strings.Split(value, ",")
		overrides := map[string]bool{}
		for _, part := range parts {
			part = strings.ToLower(part)
			if _, ok := allImageOverrides[part]; ok {
				overrides[part] = true
			}
		}
		return overrides
	}
}

func parseExec(value string) ([]string, error) {
	if value == "" {
		return []string{}, nil
	}

	if value[0] != '[' {
		return shlex.Split(value)
	}

	var parts []string
	if err := json.Unmarshal([]byte(value), &parts); err != nil {
		return nil, err
	}

	return parts, nil
}

func parseVolumeMounts(values []string) (map[string]config.VolumeMount, error) {
	volumeMounts := map[string]config.VolumeMount{}

	for _, raw := range values {
		if !strings.Contains(raw, ":") {
			return nil, fmt.Errorf("invalid volume mount format: %s", raw)
		}

		parts := strings.Split(raw, ":")
		if (len(parts) > 3) ||
			(len(parts[0]) < 1) ||
			(len(parts[1]) < 1) ||
			((len(parts) == 3) && (len(parts[2]) < 1)) {
			return nil, fmt.Errorf("invalid volume mount format: %s", raw)
		}

		mount := config.VolumeMount{
			Source:      parts[0],
			Destination: parts[1],
			Options:     "rw",
		}

		if len(parts) == 3 {
			mount.Options = parts[2]
		}

		volumeMounts[mount.Source] = mount
	}
	return volumeMounts, nil
}

func parsePaths(values []string) map[string]bool {
	paths := map[string]bool{}

	for _, value := range values {
		paths[value] = true
	}

	return paths
}

func parsePathsFile(filePath string) (map[string]bool, error) {
	paths := map[string]bool{}

	if filePath == "" {
		return paths, nil
	}

	fullPath, err := filepath.Abs(filePath)
	if err != nil {
		return paths, err
	}

	if _, err := os.Stat(fullPath); err != nil {
		return paths, err
	}

	fileData, err := ioutil.ReadFile(fullPath) //[]byte
	if err != nil {
		return paths, err
	}

	if len(fileData) == 0 {
		return paths, nil
	}

	lines := strings.Split(string(fileData), "\n")

	for _, line := range lines {
		line := strings.TrimSpace(line)
		if len(line) != 0 {
			paths[line] = true
		}
	}

	return paths, nil
}

func parseHTTPProbes(values []string) ([]config.HTTPProbeCmd, error) {
	probes := []config.HTTPProbeCmd{}

	for _, raw := range values {
		sepCount := strings.Count(raw, ":")
		switch sepCount {
		case 0:
			if raw == "" || !isResource(raw) {
				return nil, fmt.Errorf("invalid HTTP probe command resource: %+v", raw)
			}

			probes = append(probes, config.HTTPProbeCmd{Protocol: "http", Method: "GET", Resource: raw})
		case 1:
			parts := strings.SplitN(raw, ":", 2)

			if parts[0] != "" && !isMethod(parts[0]) {
				return nil, fmt.Errorf("invalid HTTP probe command method: %+v", raw)
			}

			if parts[1] == "" || !isResource(parts[1]) {
				return nil, fmt.Errorf("invalid HTTP probe command resource: %+v", raw)
			}

			probes = append(probes, config.HTTPProbeCmd{Protocol: "http", Method: strings.ToUpper(parts[0]), Resource: parts[1]})
		case 2:
			parts := strings.SplitN(raw, ":", 3)

			if parts[0] != "" && !isProto(parts[0]) {
				return nil, fmt.Errorf("invalid HTTP probe command protocol: %+v", raw)
			}

			if parts[1] != "" && !isMethod(parts[1]) {
				return nil, fmt.Errorf("invalid HTTP probe command method: %+v", raw)
			}

			if parts[2] == "" || !isResource(parts[2]) {
				return nil, fmt.Errorf("invalid HTTP probe command resource: %+v", raw)
			}

			probes = append(probes, config.HTTPProbeCmd{Protocol: parts[0], Method: strings.ToUpper(parts[1]), Resource: parts[2]})
		default:
			return nil, fmt.Errorf("invalid HTTP probe command: %s", raw)
		}
	}

	return probes, nil
}

func parseHTTPProbesFile(filePath string) ([]config.HTTPProbeCmd, error) {
	probes := []config.HTTPProbeCmd{}

	if filePath != "" {
		fullPath, err := filepath.Abs(filePath)
		if err != nil {
			return nil, err
		}

		if _, err := os.Stat(fullPath); err != nil {
			return nil, err
		}

		configFile, err := os.Open(fullPath)
		if err != nil {
			return nil, err
		}
		defer configFile.Close()

		var configs config.HTTPProbeCmds
		if err = json.NewDecoder(configFile).Decode(&configs); err != nil {
			return nil, err
		}

		for _, cmd := range configs.Commands {
			if cmd.Protocol != "" && !isProto(cmd.Protocol) {
				return nil, fmt.Errorf("invalid HTTP probe command protocol: %+v", cmd)
			}

			if cmd.Method != "" && !isMethod(cmd.Method) {
				return nil, fmt.Errorf("invalid HTTP probe command method: %+v", cmd)
			}

			if cmd.Method == "" {
				cmd.Method = "GET"
			}

			cmd.Method = strings.ToUpper(cmd.Method)

			if cmd.Resource == "" || !isResource(cmd.Resource) {
				return nil, fmt.Errorf("invalid HTTP probe command resource: %+v", cmd)
			}

			if cmd.Port != 0 && !isPortNum(cmd.Port) {
				return nil, fmt.Errorf("invalid HTTP probe command port: %v", cmd)
			}

			probes = append(probes, cmd)
		}
	}

	return probes, nil
}

func isProto(value string) bool {
	switch strings.ToLower(value) {
	case "http", "https":
		return true
	default:
		return false
	}
}

func isMethod(value string) bool {
	switch strings.ToUpper(value) {
	case "HEAD", "GET", "POST", "PUT", "DELETE", "PATCH":
		return true
	default:
		return false
	}
}

func isResource(value string) bool {
	if value != "" && value[0] == '/' {
		return true
	}

	return false
}

func isPortNum(value int) bool {
	if 1 <= value && value <= 65535 {
		return true
	}

	return false
}

func parseHTTPProbesPorts(portList string) ([]uint16, error) {
	var ports []uint16

	if portList == "" {
		return ports, nil
	}

	parts := strings.Split(portList, ",")
	for _, part := range parts {
		port, err := strconv.ParseUint(part, 10, 16)
		if err != nil {
			return nil, err
		}

		ports = append(ports, uint16(port))
	}

	return ports, nil
}
