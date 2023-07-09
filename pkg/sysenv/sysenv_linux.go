package sysenv

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"regexp"
	"strings"

	"github.com/syndtr/gocapability/capability"
)

const (
	procSelfCgroup  = "/proc/self/cgroup"
	dockerEnvPath   = "/.dockerenv"
	dsImageFlagPath = "/.ds.container.d3e2c84f976743bdb92a7044ef12e381"
)

func HasDSImageFlag() bool {
	_, err := os.Stat(dsImageFlagPath)
	return err == nil
}

func HasDockerEnvPath() bool {
	_, err := os.Stat(dockerEnvPath)
	return err == nil
}

func HasContainerCgroups() bool {
	if bdata, err := os.ReadFile(procSelfCgroup); err == nil {
		return strings.Contains(string(bdata), ":/docker/")
	}

	return false
}

func InDSContainer() (bool, bool) {
	isDSImage := HasDSImageFlag()
	inContainer := InContainer()

	return inContainer, isDSImage
}

func InContainer() bool {
	if HasDockerEnvPath() {
		return true
	}

	if HasContainerCgroups() {
		return true
	}

	return false
}

func IsPrivileged() bool {
	mode, err := SeccompMode(0)
	if err != nil {
		fmt.Printf("sysenv.IsPrivileged: error - %v\n", err)
		return false
	}

	if WithAllCapabilities() &&
		mode == SeccompMNDisabled {
		return true
	}

	return false
}

func WithAllCapabilities() bool {
	caps, err := capability.NewPid(0)
	if err != nil {
		fmt.Printf("sysenv.WithAllCapabilities: error - %v\n", err)
		return false
	}

	if caps.Full(capability.PERMITTED) &&
		caps.Full(capability.EFFECTIVE) {
		return true
	}

	return false
}

func Capabilities(pid int) (map[string]struct{}, map[string]struct{}, error) {
	caps, err := capability.NewPid(pid)
	if err != nil {
		return nil, nil, err
	}

	all := capability.List()

	active := map[string]struct{}{}
	for _, cap := range all {
		if caps.Get(capability.EFFECTIVE, cap) {
			active[cap.String()] = struct{}{}
		}
	}

	max := map[string]struct{}{}
	for _, cap := range all {
		if caps.Get(capability.PERMITTED, cap) {
			max[cap.String()] = struct{}{}
		}
	}

	return active, max, nil
}

func IsDefaultCapSet(set map[string]struct{}) bool {
	if len(set) != len(DefaultCapStrings) {
		return false
	}

	for k := range set {
		if _, ok := DefaultCapStrings[k]; !ok {
			return false
		}
	}

	return true
}

//default Docker container capabilities: 14
//https://docs.docker.com/engine/reference/run/#runtime-privilege-and-linux-capabilities

var DefaultCapStrings = map[string]struct{}{
	capability.CAP_SETPCAP.String():          {},
	capability.CAP_MKNOD.String():            {},
	capability.CAP_AUDIT_WRITE.String():      {},
	capability.CAP_CHOWN.String():            {},
	capability.CAP_NET_RAW.String():          {},
	capability.CAP_DAC_OVERRIDE.String():     {},
	capability.CAP_FOWNER.String():           {},
	capability.CAP_FSETID.String():           {},
	capability.CAP_KILL.String():             {},
	capability.CAP_SETGID.String():           {},
	capability.CAP_SETUID.String():           {},
	capability.CAP_NET_BIND_SERVICE.String(): {},
	capability.CAP_SYS_CHROOT.String():       {},
	capability.CAP_SETFCAP.String():          {},
}

var DefaultCapNums = map[capability.Cap]string{
	capability.CAP_SETPCAP:          capability.CAP_SETPCAP.String(),
	capability.CAP_MKNOD:            capability.CAP_MKNOD.String(),
	capability.CAP_AUDIT_WRITE:      capability.CAP_AUDIT_WRITE.String(),
	capability.CAP_CHOWN:            capability.CAP_CHOWN.String(),
	capability.CAP_NET_RAW:          capability.CAP_NET_RAW.String(),
	capability.CAP_DAC_OVERRIDE:     capability.CAP_DAC_OVERRIDE.String(),
	capability.CAP_FOWNER:           capability.CAP_FOWNER.String(),
	capability.CAP_FSETID:           capability.CAP_FSETID.String(),
	capability.CAP_KILL:             capability.CAP_KILL.String(),
	capability.CAP_SETGID:           capability.CAP_SETGID.String(),
	capability.CAP_SETUID:           capability.CAP_SETUID.String(),
	capability.CAP_NET_BIND_SERVICE: capability.CAP_NET_BIND_SERVICE.String(),
	capability.CAP_SYS_CHROOT:       capability.CAP_SYS_CHROOT.String(),
	capability.CAP_SETFCAP:          capability.CAP_SETFCAP.String(),
}

type SeccompModeName string

const (
	SMDisabled        string          = "0"
	SeccompMNDisabled SeccompModeName = "disabled"
	SMStrict          string          = "1"
	SeccompMNStrict   SeccompModeName = "strict"
	SMFiltering       string          = "2"
	SeccompMFiltering SeccompModeName = "filtering"
)

var seccompModes = map[string]SeccompModeName{
	SMDisabled:  SeccompMNDisabled,
	SMStrict:    SeccompMNStrict,
	SMFiltering: SeccompMFiltering,
}

const procStatusPat = "/proc/%s/status"

func SeccompMode(pid int) (SeccompModeName, error) {
	fname := procFileName(pid, "status")
	fdata, err := fileData(fname)
	if err != nil {
		return "", err
	}

	mode := procSeccompMode(fdata)
	if mode != "" {
		return mode, nil
	}

	mode = sysSeccompMode()
	return mode, nil
}

func procSeccompMode(data string) SeccompModeName {
	modeNum := procStatusField(data, "Seccomp:")
	if mode, ok := seccompModes[modeNum]; ok {
		return mode
	}

	return ""
}

const procStatusFieldValueRegexStr = ":(.*)"

var procStatusFieldValueMatcher = regexp.MustCompile(procStatusFieldValueRegexStr)

func procStatusField(data, fname string) string {
	if !strings.HasSuffix(fname, ":") {
		fname = fmt.Sprintf("%s:", fname)
	}

	lines := strings.Split(data, "\n")
	for _, line := range lines {
		line := cleanLine(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, fname) {
			matches := procStatusFieldValueMatcher.FindStringSubmatch(line)
			if len(matches) > 1 {
				return strings.TrimSpace(matches[1])
			}
		}
	}

	return ""
}

func cleanLine(line string) string {
	return strings.TrimSpace(line)
}

func sysSeccompMode() SeccompModeName {
	if err := unix.Prctl(unix.PR_GET_SECCOMP, 0, 0, 0, 0); err != unix.EINVAL {
		if err := unix.Prctl(unix.PR_SET_SECCOMP, unix.SECCOMP_MODE_FILTER, 0, 0, 0); err != unix.EINVAL {
			return SeccompMNStrict
		}
	}

	return SeccompMNDisabled
}

func fileData(fname string) (string, error) {
	bdata, err := os.ReadFile(fname)
	if err != nil {
		return "", err
	}

	data := strings.TrimSpace(string(bdata))
	return data, nil
}

func procFileName(pid int, name string) string {
	target := "self"
	if pid > 0 {
		target = fmt.Sprintf("%d", pid)
	}

	return fmt.Sprintf("/proc/%s/%s", target, name)
}
