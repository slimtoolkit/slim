package sensor

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"

	log "github.com/sirupsen/logrus"
)

type ovars = app.OutVars

const (
	LocalBinFile       = "slim-sensor"
	DefaultConnectWait = 60
)

func EnsureLocalBinary(xc *app.ExecutionContext, logger *log.Entry, statePath string, printState bool) string {
	sensorPath := filepath.Join(fsutil.ExeDir(), LocalBinFile)

	if runtime.GOOS == "darwin" {
		stateSensorPath := filepath.Join(statePath, LocalBinFile)
		if fsutil.Exists(stateSensorPath) {
			sensorPath = stateSensorPath
		}
	}

	if !fsutil.Exists(sensorPath) {
		if printState {
			xc.Out.Info("sensor.error",
				ovars{
					"message":  "sensor binary not found",
					"location": sensorPath,
				})

			xc.Out.State("exited",
				ovars{
					"exit.code": -125,
					"component": "container.inspector",
					"version":   v.Current(),
				})
		}

		xc.Exit(-125)
	}

	if finfo, err := os.Lstat(sensorPath); err == nil {
		logger.Debugf("sensor.EnsureLocalBinary: sensor (%s) perms => %#o", sensorPath, finfo.Mode().Perm())
		if finfo.Mode().Perm()&fsutil.FilePermUserExe == 0 {
			logger.Debugf("sensor.EnsureLocalBinary: sensor (%s) missing execute permission", sensorPath)
			updatedMode := finfo.Mode() | fsutil.FilePermUserExe | fsutil.FilePermGroupExe | fsutil.FilePermOtherExe
			if err = os.Chmod(sensorPath, updatedMode); err != nil {
				logger.Errorf("sensor.EnsureLocalBinary: error updating sensor (%s) perms (%#o -> %#o) => %v",
					sensorPath, finfo.Mode().Perm(), updatedMode.Perm(), err)
			}
		}
	} else {
		logger.Errorf("sensor.EnsureLocalBinary: error getting sensor (%s) info => %#v", sensorPath, err)
	}

	return sensorPath
}
