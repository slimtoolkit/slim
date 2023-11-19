package seccomp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/system"
	"github.com/slimtoolkit/slim/pkg/third_party/opencontainers/specs"

	log "github.com/sirupsen/logrus"
)

var archMap = map[system.ArchName]specs.Arch{
	system.ArchName386:   specs.ArchX86,
	system.ArchNameAmd64: specs.ArchX86_64,
	system.ArchNameArm32: specs.ArchARM,
	system.ArchNameArm64: specs.ArchAARCH64,
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
	"rt_sigreturn",
	"rt_sigsuspend",
	"exit_group",
	"kill", //extra calls
	"sendmsg",
	"wait4",
	"setitimer",
	"unlink",
	"dup3",   //needed for some tty detection/setup logic (doesn't always get picked up)
	"getcwd", //safe to add
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

	if !creport.Monitors.Pt.Enabled {
		log.Debug("seccomp.GenProfile: not generating seccomp profile (PT mon disabled, no syscall info)")
		return nil
	}

	profilePath := filepath.Join(artifactLocation, profileName)
	log.Debug("seccomp.GenProfile: saving seccomp profile to ", profilePath)

	profile := &specs.Seccomp{
		DefaultAction: specs.ActErrno,
		Architectures: []specs.Arch{
			archNameToSeccompArch(creport.Monitors.Pt.ArchName),
		},
	}

	nameResolver := system.CallNameResolver(system.ArchName(creport.Monitors.Pt.ArchName))
	if nameResolver != nil {
		for _, xcall := range extraCalls {
			if cnum, ok := nameResolver(xcall); ok {
				cnKey := fmt.Sprintf("%d", cnum)
				if _, ok := creport.Monitors.Pt.SyscallStats[cnKey]; !ok {
					creport.Monitors.Pt.SyscallStats[cnKey] = report.SyscallStatInfo{Name: xcall}
				}
			}
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

	err = os.WriteFile(profilePath, profileData, 0644)
	if err != nil {
		return err
	}

	return nil
}
