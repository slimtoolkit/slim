package system

import (
	"fmt"
	"os"
	"syscall"
)

func newSystemInfo() SystemInfo {
	var sysInfo SystemInfo
	var unameInfo syscall.Utsname

	if err := syscall.Uname(&unameInfo); err != nil {
		return sysInfo
	}

	sysInfo.Sysname = nativeCharsToString(unameInfo.Sysname)
	sysInfo.Nodename = nativeCharsToString(unameInfo.Nodename)
	sysInfo.Machine = nativeCharsToString(unameInfo.Machine)
	sysInfo.Domainname = nativeCharsToString(unameInfo.Domainname)
	//kernel info
	sysInfo.Release = nativeCharsToString(unameInfo.Release)
	sysInfo.Version = nativeCharsToString(unameInfo.Version)
	//distro info
	sysInfo.Distro = distroInfo()

	return sysInfo
}

var defaultSysInfo = newSystemInfo()

func GetSystemInfo() SystemInfo {
	return defaultSysInfo
}

func distroInfo() DistroInfo {
	distro := DistroInfo{
		Name:        "unknown",
		DisplayName: "unknown",
	}

	bdata, err := os.ReadFile(OSReleaseFile)
	if err != nil {
		return distro
	}

	if osr, err := NewOsRelease(bdata); err == nil {
		var nameMain, nameVersion string

		distro.Name = osr.Name
		distro.Version = osr.VersionID
		if distro.Version == "" {
			distro.Version = osr.Version
		}

		distro.DisplayName = osr.PrettyName
		if distro.DisplayName == "" {
			nameMain = osr.Name
			if len(osr.Version) > 0 {
				nameVersion = osr.Version
			} else {
				nameVersion = osr.VersionID
			}

			distro.DisplayName = fmt.Sprintf("%v %v", nameMain, nameVersion)
		}

		return distro
	}

	distro.Name = "other"
	distro.DisplayName = "other"

	return distro
}

/*
func getOperatingSystem() string {
	bdata, err := os.ReadFile("/etc/os-release")
	if err != nil {
		print("error reading /etc/os-release")
		return ""
	}

	var nameMain, nameVersion string

	if i := bytes.Index(bdata, []byte("NAME")); i >= 0 {
		offset := i+ len("NAME") + 2
		nameData = bdata[offset:]
		nameMain = string(nameData[:bytes.IndexByte(nameData, '"')])
	}

	if i := bytes.Index(bdata, []byte("VERSION")); i >= 0 {
		offset := i+ len("VERSION") + 2
		nameData = bdata[offset:]
		nameMain = string(nameData[:bytes.IndexByte(nameData, '"')])
	} else {
		if i := bytes.Index(bdata, []byte("VERSION_ID")); i >= 0 {
			//version id could be with or without quotes
			offset := i+ len("VERSION_ID") + 2
			nameData = bdata[offset:]
			nameMain = string(nameData[:bytes.IndexByte(nameData, '"')])
		}
	}

	return fmt.Sprintf("%v %v",nameMain,nameVersion)
}
*/
