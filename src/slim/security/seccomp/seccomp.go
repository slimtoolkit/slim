package seccomp

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"external/opencontainers/specs"
	"internal/report"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/system"
)

var archMap = map[system.ArchName]specs.Arch{
	system.ArchName386:   specs.ArchX86,
	system.ArchNameAmd64: specs.ArchX86_64,
}

func archNameToSeccompArch(name string) specs.Arch {
	if arch, ok := archMap[system.ArchName(name)]; ok == true {
		return arch
	}
	return "unknown"
}

func GenProfile(artifactLocation string, profileName string) error {
	containerReportFileName := "creport.json"
	containerReportFilePath := filepath.Join(artifactLocation, containerReportFileName)

	if _, err := os.Stat(containerReportFilePath); err != nil {
		return err
	}
	reportFile, err := os.Open(containerReportFilePath)
	if err != nil {
		return err
	}
	defer reportFile.Close()

	var creport report.ContainerReport
	if err = json.NewDecoder(reportFile).Decode(&creport); err != nil {
		return err
	}

	profilePath := filepath.Join(artifactLocation, profileName)
	log.Debug("docker-slim: saving seccomp profile to ", profilePath)

	profile := &specs.Seccomp{
		DefaultAction: specs.ActErrno,
		Architectures: []specs.Arch{archNameToSeccompArch(creport.Monitors.Pt.ArchName)},
	}

	for _, scInfo := range creport.Monitors.Pt.SyscallStats {
		profile.Syscalls = append(profile.Syscalls, &specs.Syscall{
			Name:   scInfo.Name,
			Action: specs.ActAllow,
		})
	}

	profileData, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(profilePath, profileData, 0644)
	if err != nil {
		return err
	}

	return nil
}
