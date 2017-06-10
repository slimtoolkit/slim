package system

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"strings"
)

type KernelFeatures struct {
	Raw   map[string]string
	Error string
}

func (p *KernelFeatures) IsConfigured(name string) bool {
	if _, ok := p.Raw[name]; ok {
		return true
	}

	return false
}

func (p *KernelFeatures) RawValue(name string) (string, error) {
	if val, ok := p.Raw[name]; ok {
		return val, nil
	}

	return "", ErrNotConfigured
}

func (p *KernelFeatures) IsFlag(name string) (bool, error) {
	if val, ok := p.Raw[name]; ok {
		if (val == "y") || (val == "m") {
			return true, nil
		}

		return false, nil
	}

	return false, ErrNotConfigured
}

func (p *KernelFeatures) isFlagSet(name string, flag string) (bool, error) {
	if val, ok := p.Raw[name]; ok {
		if val == flag {
			return true, nil
		}

		return false, nil
	}

	return false, ErrNotConfigured
}

func (p *KernelFeatures) IsCompiled(name string) (bool, error) {
	return p.isFlagSet(name, "y")
}

func (p *KernelFeatures) IsLoadable(name string) (bool, error) {
	return p.isFlagSet(name, "m")
}

func NewKernelFeatures() (KernelFeatures, error) {
	return NewKernelFeaturesWithProps("")
}

func NewKernelFeaturesWithProps(location string) (KernelFeatures, error) {
	var kfeatures KernelFeatures

	if location == "" {
		bootConfigWithVersion := fmt.Sprintf("/boot/config-%s", defaultSysInfo.Machine)
		kernelConfigLocations := []string{
			"/proc/config.gz",
			"/boot/config",
		}

		kernelConfigLocations = append(kernelConfigLocations, bootConfigWithVersion)

		location = findKernelConfigs(kernelConfigLocations)
		if location == "" {
			kfeatures.Error = ErrNoConfigs.Error()
			return kfeatures, ErrNoConfigs
		}
	} else {
		if !fileExists(location) {
			kfeatures.Error = os.ErrNotExist.Error()
			return kfeatures, os.ErrNotExist
		}
	}

	rawFeatures, err := readKernelFeatures(location)
	if err != nil {
		kfeatures.Error = err.Error()
		return kfeatures, err
	}

	kfeatures.Raw = rawFeatures

	return kfeatures, nil
}

var DefaultKernelFeatures, _ = NewKernelFeatures()

func findKernelConfigs(locations []string) string {
	for _, loc := range locations {
		if fileExists(loc) {
			return loc
		}
	}

	return ""
}

func fileExists(filePath string) bool {
	fileInfo, err := os.Stat(filePath)
	if err == nil && (fileInfo.Mode().IsRegular()) {
		return true
	}

	return false
}

func readKernelFeatures(filename string) (map[string]string, error) {
	freader, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer freader.Close()

	areader, err := gzip.NewReader(freader)
	if err != nil {
		return nil, err
	}
	defer areader.Close()

	var lines []string

	scanner := bufio.NewScanner(areader)
	kernelFeatures := map[string]string{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line[0] == '#' {
			//todo: extract section metadata from comments
			continue
		}

		lineParts := strings.Split(line, "=")
		if len(lineParts) == 2 {
			flagKey := strings.TrimSpace(lineParts[0])
			flagValue := strings.TrimSpace(lineParts[1])
			flagValue = strings.Trim(flagValue, "\"")

			lines = append(lines, line)
			kernelFeatures[flagKey] = flagValue
		}
	}

	if err = scanner.Err(); err != nil {
		//if err == bufio.ErrTooLong {
		//	log.Println("line length error:", err)
		//} else {
		//	log.Println("other scanner error:", err)
		//}
		return kernelFeatures, err
	}

	return kernelFeatures, nil
}
