package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cloudimmunity/go-dockerclientx"
	"github.com/docker/go-connections/nat"

	"github.com/cloudimmunity/docker-slim/master/config"
)

//based on expose opt parsing in Docker
func parseDockerExposeOpt(values []string) (map[docker.Port]struct{}, error) {
	exposedPorts := map[docker.Port]struct{}{}

	for _, raw := range values {
		if strings.Contains(raw, ":") {
			return nil, fmt.Errorf("Invalid EXPOSE format: %s", raw)
		}

		proto, ports := nat.SplitProtoPort(raw)
		startPort, endPort, err := nat.ParsePortRange(ports)
		if err != nil {
			return nil, fmt.Errorf("Invalid port range in EXPOSE: %s / error: %s", raw, err)
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
			if allImageOverrides[part] {
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
		return strings.Fields(value), nil
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
			return nil, fmt.Errorf("Invalid volume mount format: %s", raw)
		}

		parts := strings.Split(raw, ":")
		if (len(parts) > 3) ||
			(len(parts[0]) < 1) ||
			(len(parts[1]) < 1) ||
			((len(parts) == 3) && (len(parts[2]) < 1)) {
			return nil, fmt.Errorf("Invalid volume mount format: %s", raw)
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
