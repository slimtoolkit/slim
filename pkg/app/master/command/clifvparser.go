package command

//Flag value parsers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/docker/go-connections/nat"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app/master/config"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/sysenv"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
)

const (
	DefaultStateArchiveVolumeName = "slim-state"
)

func IsInContainer(flag bool) (bool, bool) {
	if flag {
		return true, sysenv.HasDSImageFlag()
	}

	return sysenv.InDSContainer()
}

func ArchiveState(flag string, inContainer bool) string {
	switch flag {
	case "":
		switch inContainer {
		case true:
			return DefaultStateArchiveVolumeName
		default:
			return ""
		}
	case "off":
		return ""
	default:
		return flag //should validate if it can be a Docker volume name
	}
}

// based on expose opt parsing in Docker
func ParseDockerExposeOpt(values []string) (map[docker.Port]struct{}, error) {
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

func ParsePortBindings(values []string) (map[docker.Port][]docker.PortBinding, error) {
	portBindings := map[docker.Port][]docker.PortBinding{}

	for _, raw := range values {
		var (
			hostIP   = ""
			hostPort = ""
			portKey  = ""
		)

		parts := strings.Split(raw, ":")
		//format:
		// port
		// hostPort:containerPort
		// hostIP:hostPort:containerPort
		// hostIP::containerPort
		switch len(parts) {
		case 1:
			portKey = fmt.Sprintf("%s/tcp", parts[0])
			hostPort = parts[0]
		case 2:
			hostPort = parts[0]
			if strings.Contains(parts[1], "/") {
				portKey = parts[1]
			} else {
				portKey = fmt.Sprintf("%s/tcp", parts[1])
			}
		case 3:
			hostIP = parts[0]
			if len(parts[1]) > 0 {
				hostPort = parts[1]
			} else {
				hostPort = parts[2]
			}

			if strings.Contains(parts[2], "/") {
				portKey = parts[2]
			} else {
				portKey = fmt.Sprintf("%s/tcp", parts[2])
			}
		default:
			return nil, fmt.Errorf("invalid publish-port: %s", raw)
		}

		portBindings[docker.Port(portKey)] = []docker.PortBinding{{
			HostIP:   hostIP,
			HostPort: hostPort,
		}}
	}

	return portBindings, nil
}

func IsOneSpace(value string) bool {
	if len(value) > 0 && utf8.RuneCountInString(value) == 1 {
		r, _ := utf8.DecodeRuneInString(value)
		if r != utf8.RuneError && unicode.IsSpace(r) {
			return true
		}
	}

	return false
}

var AllImageOverrides = map[string]bool{
	"entrypoint": true,
	"cmd":        true,
	"workdir":    true,
	"env":        true,
	"expose":     true,
	"volume":     true,
	"label":      true,
}

func ParseImageOverrides(value string) map[string]bool {
	switch value {
	case "":
		return map[string]bool{}
	case "all":
		return AllImageOverrides
	default:
		parts := strings.Split(value, ",")
		overrides := map[string]bool{}
		for _, part := range parts {
			part = strings.ToLower(part)
			if _, ok := AllImageOverrides[part]; ok {
				overrides[part] = true
			}
		}
		return overrides
	}
}

func ParseExec(value string) ([]string, error) {
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

func ParseTokenSet(values []string) (map[string]struct{}, error) {
	tokens := map[string]struct{}{}
	for _, token := range values {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		tokens[token] = struct{}{}
	}

	return tokens, nil
}

func ParseTokenMap(values []string) (map[string]string, error) {
	tokens := map[string]string{}
	for _, token := range values {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		parts := strings.SplitN(token, "=", 2)
		if len(parts) != 2 {
			continue
		}

		tokens[parts[0]] = parts[1]
	}

	return tokens, nil
}

func ParseCheckTags(values []string) (map[string]string, error) {
	tags := map[string]string{}
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		if !strings.Contains(raw, ":") {
			return nil, fmt.Errorf("invalid check tag format: %s", raw)
		}

		parts := strings.Split(raw, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid check tag format: %s", raw)
		}

		tags[parts[0]] = parts[1]
	}

	return tags, nil
}

func ParseTokenSetFile(filePath string) (map[string]struct{}, error) {
	tokens := map[string]struct{}{}

	if filePath == "" {
		return tokens, nil
	}

	fullPath, err := filepath.Abs(filePath)
	if err != nil {
		return tokens, err
	}

	_, err = os.Stat(fullPath)
	if err != nil {
		return tokens, err
	}

	fileData, err := os.ReadFile(fullPath) //[]byte
	if err != nil {
		return tokens, err
	}

	if len(fileData) == 0 {
		return tokens, nil
	}

	lines := strings.Split(string(fileData), "\n")

	for _, token := range lines {
		token = strings.TrimSpace(token)
		if len(token) != 0 {
			tokens[token] = struct{}{}
		}
	}

	return tokens, nil
}

func ParseVolumeMounts(values []string) (map[string]config.VolumeMount, error) {
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

		//NOTE: also need to support volume bindings
		//with the same source, but different destinations
		volumeMounts[mount.Source] = mount
	}
	return volumeMounts, nil
}

func ParseVolumeMountsAsList(values []string) ([]config.VolumeMount, error) {
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

		key := fmt.Sprintf("%s:%s", mount.Source, mount.Destination)
		volumeMounts[key] = mount
	}

	var volumeList []config.VolumeMount
	for _, m := range volumeMounts {
		volumeList = append(volumeList, m)
	}

	return volumeList, nil
}

func ParsePathPerms(raw string) (string, *fsutil.AccessInfo, error) {
	access := fsutil.NewAccessInfo()
	//note: will work for ASCII (todo: make it work for unicode)
	//
	//DATA FORMAT:
	//
	//    filePath
	//    filePath:octalFilemodeFlags#uid
	//    filePath:octalFilemodeFlags#uid#gid
	//
	//Filemode bits: perms and extra bits (sticky, setuid, setgid)

	sepIdx := strings.LastIndex(raw, ":")
	if sepIdx == -1 || sepIdx == (len(raw)-1) {
		return raw, nil, nil
	}

	pathStr := raw[0:sepIdx]
	metaStr := raw[sepIdx+1:]

	metaParts := strings.Split(metaStr, "#")

	var permBitsStr string
	var extraBitsStr string

	fileModeStr := metaParts[0]
	if len(fileModeStr) > 3 {
		access.PermsOnly = false
		if len(fileModeStr) > 4 {
			fileModeStr = fileModeStr[len(fileModeStr)-4:]
		}
		extraBitsStr = fileModeStr[0:1]
		permBitsStr = fileModeStr[1:]
	} else {
		access.PermsOnly = true
		permBitsStr = fileModeStr
	}

	permsNum, err := strconv.ParseUint(permBitsStr, 8, 32)
	if err != nil {
		return "", nil, err
	}

	access.Flags = os.FileMode(permsNum)

	if len(extraBitsStr) > 0 {
		extraBits, err := strconv.ParseUint(extraBitsStr, 8, 32)
		if err != nil {
			return "", nil, err
		}

		access.Flags |= fsutil.FileModeExtraBitsUnix2Go(uint32(extraBits))
	}

	if len(metaParts) > 1 {
		uidNum, err := strconv.ParseInt(metaParts[1], 10, 32)
		if err == nil && uidNum > -1 {
			access.UID = int(uidNum)
		}
	}

	if len(metaParts) > 2 {
		gidNum, err := strconv.ParseInt(metaParts[2], 10, 32)
		if err == nil && gidNum > -1 {
			access.GID = int(gidNum)
		}
	}

	return pathStr, access, nil
}

func ParsePaths(values []string) map[string]*fsutil.AccessInfo {
	const op = "commands.ParsePaths"
	paths := map[string]*fsutil.AccessInfo{}

	for _, raw := range values {
		pathStr, access, err := ParsePathPerms(raw)
		if err != nil {
			log.WithFields(log.Fields{
				"op":    op,
				"line":  raw,
				"error": err,
			}).Debug("skipping.line")
			continue
		}

		paths[pathStr] = access
	}

	return paths
}

func ValidateFiles(names []string) ([]string, map[string]error) {
	found := []string{}
	errors := map[string]error{}

	for _, name := range names {
		if name == "" {
			continue
		}

		fullPath, err := filepath.Abs(name)
		if err != nil {
			errors[name] = err
			continue
		}

		_, err = os.Stat(fullPath)
		if err != nil {
			errors[name] = err
			continue
		}

		found = append(found, name)
	}

	return found, errors
}

func ParsePathsFile(filePath string) (map[string]*fsutil.AccessInfo, error) {
	const op = "commands.ParsePathsFile"
	paths := map[string]*fsutil.AccessInfo{}

	if filePath == "" {
		return paths, nil
	}

	fullPath, err := filepath.Abs(filePath)
	if err != nil {
		return paths, err
	}

	_, err = os.Stat(fullPath)
	if err != nil {
		return paths, err
	}

	fileData, err := os.ReadFile(fullPath) //[]byte
	if err != nil {
		return paths, err
	}

	if len(fileData) == 0 {
		return paths, nil
	}

	lines := strings.Split(string(fileData), "\n")
	log.WithFields(log.Fields{
		"op":          op,
		"file.path":   filePath,
		"full.path":   fullPath,
		"lines.count": len(lines),
	}).Trace("data")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) != 0 {
			pathStr, access, err := ParsePathPerms(line)
			if err != nil {
				log.WithFields(log.Fields{
					"op":        op,
					"file.path": filePath,
					"full.path": fullPath,
					"line":      line,
					"error":     err,
				}).Debug("skipping.line")
				continue
			}

			paths[pathStr] = access
		}
	}

	return paths, nil
}

// ///
func ParsePathsCreportFile(filePath string) (map[string]*fsutil.AccessInfo, error) {
	const op = "commands.ParsePathsCreportFile"
	paths := map[string]*fsutil.AccessInfo{}

	if filePath == "" {
		return paths, nil
	}

	fullPath, err := filepath.Abs(filePath)
	if err != nil {
		return paths, err
	}

	_, err = os.Stat(fullPath)
	if err != nil {
		return paths, err
	}

	fileData, err := os.ReadFile(fullPath) //[]byte
	if err != nil {
		return paths, err
	}

	if len(fileData) == 0 {
		return paths, nil
	}

	var creport report.ContainerReport
	if err = json.NewDecoder(bytes.NewReader(fileData)).Decode(&creport); err != nil {
		return paths, err
	}

	for _, finfo := range creport.Image.Files {
		if finfo == nil || finfo.FilePath == "" {
			continue
		}

		paths[finfo.FilePath] = nil
	}

	return paths, nil
}

/////

func ParseHTTPProbes(values []string) ([]config.HTTPProbeCmd, error) {
	probes := []config.HTTPProbeCmd{}

	for _, raw := range values {
		var crawl bool
		parts := strings.Split(raw, ":")
		if parts[0] == "crawl" {
			crawl = true
			parts = parts[1:]
		}

		proto := "http"
		method := "GET"
		resource := "/"

		//sepCount := strings.Count(raw, ":")
		switch len(parts) {
		case 0:
		case 1:
			if parts[0] == "" || !isResource(parts[0]) {
				return nil, fmt.Errorf("invalid HTTP probe command resource: %+v", raw)
			}

			resource = parts[0]
		case 2:
			if parts[0] != "" && !isMethod(parts[0]) {
				return nil, fmt.Errorf("invalid HTTP probe command method: %+v", raw)
			}

			method = strings.ToUpper(parts[0])

			if parts[1] == "" || !isResource(parts[1]) {
				return nil, fmt.Errorf("invalid HTTP probe command resource: %+v", raw)
			}

			resource = parts[1]
		case 3:
			if parts[0] != "" && !config.IsProto(parts[0]) {
				return nil, fmt.Errorf("invalid HTTP probe command protocol: %+v", raw)
			}

			proto = strings.ToLower(parts[0])

			if parts[1] != "" && !isMethod(parts[1]) {
				return nil, fmt.Errorf("invalid HTTP probe command method: %+v", raw)
			}

			method = strings.ToUpper(parts[1])

			if parts[2] == "" || !isResource(parts[2]) {
				return nil, fmt.Errorf("invalid HTTP probe command resource: %+v", raw)
			}

			resource = parts[2]

		default:
			return nil, fmt.Errorf("invalid HTTP probe command: %s", raw)
		}

		cmd := config.HTTPProbeCmd{
			Protocol: proto,
			Method:   method,
			Resource: resource,
			Crawl:    crawl,
		}
		probes = append(probes, cmd)
	}

	return probes, nil
}

func ParseHTTPProbesFile(filePath string) ([]config.HTTPProbeCmd, error) {
	probes := []config.HTTPProbeCmd{}

	if filePath != "" {
		fullPath, err := filepath.Abs(filePath)
		if err != nil {
			return nil, err
		}

		_, err = os.Stat(fullPath)
		if err != nil {
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
			if cmd.Protocol != "" && !config.IsProto(cmd.Protocol) {
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

			if cmd.BodyFile != "" {
				bfFullPath, err := filepath.Abs(cmd.BodyFile)
				if err != nil {
					return nil, err
				}

				_, err = os.Stat(bfFullPath)
				if err != nil {
					return nil, err
				}

				cmd.BodyFile = bfFullPath

				//the body data file should be ok to load
				//will load the data at runtime
			}

			probes = append(probes, cmd)
		}
	}

	return probes, nil
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

func ParseHTTPProbesPorts(portList string) ([]uint16, error) {
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

func ParseHTTPProbeExecFile(filePath string) ([]string, error) {
	var appCalls []string

	if filePath == "" {
		return appCalls, nil
	}

	fullPath, err := filepath.Abs(filePath)
	if err != nil {
		return appCalls, err
	}

	_, err = os.Stat(fullPath)
	if err != nil {
		return appCalls, err
	}

	fileData, err := os.ReadFile(fullPath)
	if err != nil {
		return appCalls, err
	}

	if len(fileData) == 0 {
		return appCalls, nil
	}

	lines := strings.Split(string(fileData), "\n")

	for _, appCall := range lines {
		appCall = strings.TrimSpace(appCall)
		if len(appCall) != 0 {
			appCalls = append(appCalls, appCall)
		}
	}

	return appCalls, nil
}

func ParseLinesWithCommentsFile(filePath string) ([]string, error) {
	var output []string

	if filePath == "" {
		return output, nil
	}

	fullPath, err := filepath.Abs(filePath)
	if err != nil {
		return output, err
	}

	_, err = os.Stat(fullPath)
	if err != nil {
		return output, err
	}

	fileData, err := os.ReadFile(fullPath)
	if err != nil {
		return output, err
	}

	if len(fileData) == 0 {
		return output, nil
	}

	lines := strings.Split(string(fileData), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) != 0 && !strings.HasPrefix(line, "#") {
			output = append(output, line)
		}
	}

	return output, nil
}

func IsTrueStr(value string) bool {
	if strings.ToLower(value) == "true" {
		return true
	}

	return false
}

func ParseEnvFile(filePath string) ([]string, error) {
	var output []string

	if filePath == "" {
		return output, nil
	}

	fullPath, err := filepath.Abs(filePath)
	if err != nil {
		return output, err
	}
	_, err = os.Stat(fullPath)
	if err != nil {
		return output, err
	}

	fileData, err := os.ReadFile(fullPath)
	if err != nil {
		return output, err
	}

	if len(fileData) == 0 {
		return output, nil
	}

	lines := strings.Split(string(fileData), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 0 {
			//env var format validation is done separately
			output = append(output, line)
		}
	}
	return output, nil
}
