package system

import (
	"fmt"
	"io/ioutil"
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
	sysInfo.Release = nativeCharsToString(unameInfo.Release)
	sysInfo.Version = nativeCharsToString(unameInfo.Version)
	sysInfo.Machine = nativeCharsToString(unameInfo.Machine)
	sysInfo.Domainname = nativeCharsToString(unameInfo.Domainname)

	sysInfo.OsName = osName()

	return sysInfo
}

var defaultSysInfo = newSystemInfo()

func GetSystemInfo() SystemInfo {
	return defaultSysInfo
}

func osName() string {
	bdata, err := ioutil.ReadFile("/etc/os-release")
	if err != nil {
		fmt.Printf("error reading /etc/os-release: %v\n", err)
		return "other"
	}

	if osr, err := NewOsRelease(bdata); err == nil {
		var nameMain, nameVersion string

		nameMain = osr.Name
		if len(osr.Version) > 0 {
			nameVersion = osr.Version
		} else {
			nameVersion = osr.VersionID
		}

		return fmt.Sprintf("%v %v", nameMain, nameVersion)
	}

	return "other"
}

/*
func getOperatingSystem() string {
	bdata, err := ioutil.ReadFile("/etc/os-release")
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
