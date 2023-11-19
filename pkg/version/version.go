package version

import (
	"fmt"
	"runtime"

	"github.com/slimtoolkit/slim/pkg/consts"
)

var (
	appVersionTag  = "latest"
	appVersionRev  = "latest"
	appVersionTime = "latest"
	currentVersion = "v"
)

func init() {
	currentVersion = fmt.Sprintf("%s/%s|%s|%s|%s|%s",
		runtime.GOOS,
		runtime.GOARCH,
		consts.AppVersionName,
		appVersionTag,
		appVersionRev,
		appVersionTime)
}

// Current returns the current version information
func Current() string {
	return currentVersion
}

func Tag() string {
	return appVersionTag
}
