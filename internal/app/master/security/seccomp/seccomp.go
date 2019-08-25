package seccomp

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/system"
	"github.com/docker-slim/docker-slim/pkg/third_party/opencontainers/specs"

	log "github.com/Sirupsen/logrus"
)

var archMap = map[system.ArchName]specs.Arch{
	system.ArchName386:   specs.ArchX86,
	system.ArchNameAmd64: specs.ArchX86_64,
}

func archNameToSeccompArch(name string) specs.Arch {
	if arch, ok := archMap[system.ArchName(name)]; ok {
		return arch
	}
	return "unknown"
}

var extraCalls = []string{
	"openat",
	"getdents64",
	"capget",
	"capset",
	"chdir",
	"setuid",
	"setgroups",
	"setgid",
	"prctl",
	"fchown",
	"getppid",
	"getpid",
	"getuid",
	"getgid",
	"epoll_pwait",
	"newfstatat",
	"exit",
	"stat",
	"lstat",
	"write",
	"futex",
	"execve", //always detected, but it's still one of the syscalls Docker itself needs
}

// GenProfile creates a SecComp profile
func GenProfile(artifactLocation string, profileName string) error {
	containerReportFilePath := filepath.Join(artifactLocation, report.DefaultContainerReportFileName)

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

	for _, xcall := range extraCalls {
		if _, ok := creport.Monitors.Pt.SyscallStats[xcall]; !ok {
			creport.Monitors.Pt.SyscallStats[xcall] = report.SyscallStatInfo{Name: xcall}
		}
	}

	scSpec := specs.Syscall{
		Action: specs.ActAllow,
	}

	for _, scInfo := range creport.Monitors.Pt.SyscallStats {
		scSpec.Names = append(scSpec.Names, scInfo.Name)
	}

	profile.Syscalls = append(profile.Syscalls, &scSpec)

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
