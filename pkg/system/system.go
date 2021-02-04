package system

import (
	"os/user"
	"strconv"
)

const (
	OSReleaseFile    = "/etc/os-release"
	OSReleaseFileNew = "/usr/lib/os-release"
	LSBReleaseFile   = "/etc/lsb-release"
	IssueFile        = "/etc/issue"
	IssueNetFile     = "/etc/issue.net"
)

func IsOSReleaseFile(name string) bool {
	switch name {
	case OSReleaseFile, OSReleaseFileNew:
		return true
	default:
		return false
	}
}

type SystemInfo struct {
	Sysname    string
	Nodename   string
	Release    string
	Version    string
	Machine    string
	Domainname string
	OsBuild    string
	Distro     DistroInfo
}

type DistroInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	DisplayName string `json:"display_name"`
}

func ResolveUser(identity string) (uint32, uint32, error) {
	var userInfo *user.User
	if _, err := strconv.ParseUint(identity, 10, 32); err == nil {
		userInfo, err = user.LookupId(identity)
		if err != nil {
			return 0, 0, err
		}
	} else {
		userInfo, err = user.Lookup(identity)
		if err != nil {
			return 0, 0, err
		}
	}

	uid, err := strconv.ParseUint(userInfo.Uid, 10, 32)
	if err != nil {
		return 0, 0, err
	}

	gid, err := strconv.ParseUint(userInfo.Gid, 10, 32)
	if err != nil {
		return 0, 0, err
	}

	return uint32(uid), uint32(gid), nil
}

func ResolveGroup(identity string) (uint32, error) {
	var groupInfo *user.Group
	if _, err := strconv.ParseUint(identity, 10, 32); err == nil {
		groupInfo, err = user.LookupGroupId(identity)
		if err != nil {
			return 0, err
		}
	} else {
		groupInfo, err = user.LookupGroup(identity)
		if err != nil {
			return 0, err
		}
	}

	gid, err := strconv.ParseUint(groupInfo.Gid, 10, 32)
	if err != nil {
		return 0, err
	}

	return uint32(gid), nil
}
