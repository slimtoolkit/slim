package system

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

func newSystemInfo() SystemInfo {
	var sysInfo SystemInfo

	sysInfo.Sysname = runtime.GOOS
	sysInfo.Nodename, _ = os.Hostname()

	if machineInfo, err := syscall.Sysctl("hw.machine"); err == nil {
		sysInfo.Machine = machineInfo
	}

	if releaseInfo, err := syscall.Sysctl("kern.osrelease"); err == nil {
		rparts := strings.SplitN(releaseInfo, ".", 3)
		if len(rparts) == 3 {
			major, _ := strconv.ParseUint(rparts[0], 10, 64)
			minor, _ := strconv.ParseUint(rparts[1], 10, 64)

			sysInfo.Distro = DistroInfo{
				DisplayName: osName(major, minor),
			}
		}

		sysInfo.Release = releaseInfo
	}

	if versionInfo, err := syscall.Sysctl("kern.version"); err == nil {
		vparts := strings.SplitN(versionInfo, ":", 2)
		if len(vparts) == 2 {
			sysInfo.Version = vparts[1]
		}
	}

	if buildInfo, err := syscall.Sysctl("kern.osversion"); err == nil {
		sysInfo.OsBuild = buildInfo
	}

	return sysInfo
}

var defaultSysInfo = newSystemInfo()

func GetSystemInfo() SystemInfo {
	return defaultSysInfo
}

func osName(major, minor uint64) string {
	if info, ok := osNames[major]; ok {
		return fmt.Sprintf("%v (%v%v)", info.name, info.numPrefix, minor)
	}

	return "other"
}

// Mac OS X version names and numbers:
// https://support.apple.com/en-us/HT201260
// https://en.wikipedia.org/wiki/MacOS_version_history
var osNames = map[uint64]struct {
	name, numPrefix string
}{
	4:  {"Cheetah", "10.0."},
	5:  {"Puma", "10.1."},
	6:  {"Jaguar", "10.2."},
	7:  {"Panther", "10.3."},
	8:  {"Tiger", "10.4."},
	9:  {"Leopard", "10.5."},
	10: {"Snow Leopard", "10.6."},
	11: {"Lion", "10.7."},
	12: {"Mountain Lion", "10.8."},
	13: {"Mavericks", "10.9."},
	14: {"Yosemite", "10.10."},
	15: {"El Capitan", "10.11."},
	16: {"Sierra", "10.12."},
	17: {"High Sierra", "10.13."},
	18: {"Mojave", "10.14."},
	19: {"Catalina", "10.15."},
	20: {"Big Sur", "11."},
	21: {"Monterey", "12."},
	22: {"Ventura", "13."},
	23: {"Sonoma", "14."},
}
