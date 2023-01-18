package version

import (
	"fmt"
	"runtime"

	"github.com/docker-slim/docker-slim/pkg/consts"
)

var (
	appVersionTag  = "latest"
	appVersionRev  = "latest"
	appVersionTime = "latest"
	currentVersion = "v"
)

func init() {
	currentVersion = fmt.Sprintf("%v|%v|%v|%v|%v", runtime.GOOS, consts.AppVersionName, appVersionTag, appVersionRev, appVersionTime)
}

// Current returns the current version information
func Current() string {
	return currentVersion
}

func Tag() string {
	return appVersionTag
}
