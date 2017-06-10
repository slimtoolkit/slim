package system

import (
	"syscall"
)

type SystemInfo struct {
	Sysname    string
	Nodename   string
	Release    string
	Version    string
	Machine    string
	Domainname string
}

func newSystemInfo() SystemInfo {
	var sysInfo SystemInfo
	var unameInfo syscall.Utsname

	if err := syscall.Uname(&unameInfo); err != nil {
		return sysInfo
	}

	sysInfo.Sysname = nativeCharsToString(unameInfo.Sysname)
	sysInfo.Nodename = nativeCharsToString(unameInfo.Nodename)
	sysInfo.Release = nativeCharsToString(unameInfo.Release)
	sysInfo.Version = nativeCharsToString(unameInfo.Version)
	sysInfo.Machine = nativeCharsToString(unameInfo.Machine)
	sysInfo.Domainname = nativeCharsToString(unameInfo.Domainname)

	return sysInfo
}

var defaultSysInfo = newSystemInfo()

func GetSystemInfo() SystemInfo {
	return defaultSysInfo
}
