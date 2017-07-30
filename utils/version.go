package utils

import (
	"fmt"
	"runtime"

	"github.com/docker-slim/docker-slim/consts"
)

var (
	appVersionTag  = "latest"
	appVersionRev  = "latest"
	appVersionTime = "latest"
	currentVersion = "v"
)

func init() {
	currentVersion = fmt.Sprintf("%v|%v|%v|%v|%v (%v)", runtime.GOOS, consts.AppVersionName, appVersionTag, appVersionRev, appVersionTime, runtime.Version())
}

// CurrentVersion returns the current version information
func CurrentVersion() string {
	return currentVersion
}
