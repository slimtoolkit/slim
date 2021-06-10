package system

import (
	"bufio"
	"bytes"
	"fmt"
	"reflect"
	"strings"

	"log"
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

//NOTE:
//copied from https://github.com/docker/machine/blob/master/libmachine/provision/os_release.go

// The /etc/os-release file contains operating system identification data
// See http://www.freedesktop.org/software/systemd/man/os-release.html for more details

// OsRelease reflects values in /etc/os-release
// Values in this struct must always be string
// or the reflection will not work properly.
type OsRelease struct {
	AnsiColor    string `osr:"ANSI_COLOR"`
	Name         string `osr:"NAME"`
	Version      string `osr:"VERSION"`
	Variant      string `osr:"VARIANT"`
	VariantID    string `osr:"VARIANT_ID"`
	ID           string `osr:"ID"`
	IDLike       string `osr:"ID_LIKE"`
	PrettyName   string `osr:"PRETTY_NAME"`
	VersionID    string `osr:"VERSION_ID"`
	HomeURL      string `osr:"HOME_URL"`
	SupportURL   string `osr:"SUPPORT_URL"`
	BugReportURL string `osr:"BUG_REPORT_URL"`
}

func stripQuotes(val string) string {
	if len(val) > 0 && val[0] == '"' {
		return val[1 : len(val)-1]
	}
	return val
}

func (osr *OsRelease) setIfPossible(key, val string) error {
	v := reflect.ValueOf(osr).Elem()
	for i := 0; i < v.NumField(); i++ {
		fieldValue := v.Field(i)
		fieldType := v.Type().Field(i)
		originalName := fieldType.Tag.Get("osr")
		if key == originalName && fieldValue.Kind() == reflect.String {
			fieldValue.SetString(val)
			return nil
		}
	}
	return fmt.Errorf("Couldn't set key %s, no corresponding struct field found", key)
}

func parseLine(osrLine string) (string, string, error) {
	if osrLine == "" {
		return "", "", nil
	}

	vals := strings.Split(osrLine, "=")
	if len(vals) != 2 {
		return "", "", fmt.Errorf("Expected %s to split by '=' char into two strings, instead got %d strings", osrLine, len(vals))
	}
	key := vals[0]
	val := stripQuotes(vals[1])
	return key, val, nil
}

func (osr *OsRelease) ParseOsRelease(osReleaseContents []byte) error {
	r := bytes.NewReader(osReleaseContents)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		key, val, err := parseLine(scanner.Text())
		if err != nil {
			log.Printf("Warning: got an invalid line error parsing /etc/os-release: %s", err)
			continue
		}
		if err := osr.setIfPossible(key, val); err != nil {
			//log.Printf("Info: %s\n",err)
			//note: ignore, printing these messages causes more confusion...
		}
	}
	return nil
}

func NewOsRelease(contents []byte) (*OsRelease, error) {
	osr := &OsRelease{}
	if err := osr.ParseOsRelease(contents); err != nil {
		return nil, err
	}
	return osr, nil
}

//TODO: Add LSB Release and Issue file parsers

/*
cat /etc/os-release

NAME="Ubuntu"
VERSION="14.04.5 LTS, Trusty Tahr"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 14.04.5 LTS"
VERSION_ID="14.04"
HOME_URL="http://www.ubuntu.com/"
SUPPORT_URL="http://help.ubuntu.com/"
BUG_REPORT_URL="http://bugs.launchpad.net/ubuntu/"

cat /etc/os-release

NAME="Alpine Linux"
ID=alpine
VERSION_ID=3.5.2
PRETTY_NAME="Alpine Linux v3.5"
HOME_URL="http://alpinelinux.org"
BUG_REPORT_URL="http://bugs.alpinelinux.org"


cat /etc/os-release

NAME="CentOS Linux"
VERSION="7 (Core)"
ID="centos"
ID_LIKE="rhel fedora"
VERSION_ID="7"
PRETTY_NAME="CentOS Linux 7 (Core)"
ANSI_COLOR="0;31"
CPE_NAME="cpe:/o:centos:centos:7"
HOME_URL="https://www.centos.org/"
BUG_REPORT_URL="https://bugs.centos.org/"
CENTOS_MANTISBT_PROJECT="CentOS-7"
CENTOS_MANTISBT_PROJECT_VERSION="7"
REDHAT_SUPPORT_PRODUCT="centos"
REDHAT_SUPPORT_PRODUCT_VERSION="7"

cat /etc/os-release

PRETTY_NAME="Debian GNU/Linux 9 (stretch)"
NAME="Debian GNU/Linux"
VERSION_ID="9"
VERSION="9 (stretch)"
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
*/
