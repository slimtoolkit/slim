//go:build linux
// +build linux

package sensor

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
)

func configureLogger(
	enableDebug bool,
	levelName string,
	format string,
	logFile string,
) error {
	if err := setLogLevel(enableDebug, levelName); err != nil {
		return fmt.Errorf("failed to set log-level: %v", err)
	}

	if err := setLogFormat(format); err != nil {
		return fmt.Errorf("failed to set log format: %v", err)
	}

	if len(logFile) > 0 {
		// This touch is not ideal - need to understand how to merge this logic with artifacts.PrepareEnv().
		if err := fsutil.Touch(logFile); err != nil {
			return fmt.Errorf("failed to set log output destination to %q, touch failed with: %v", logFile, err)
		}

		f, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to set log output destination to %q: %w", logFile, err)
		}

		log.SetOutput(f)
	}

	return nil
}

func setLogFormat(format string) error {
	switch format {
	case "text":
		log.SetFormatter(&log.TextFormatter{DisableColors: true})
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
	default:
		return fmt.Errorf("unknown log-format %q", format)
	}

	return nil
}

func setLogLevel(enableDebug bool, levelName string) error {
	if enableDebug {
		log.SetLevel(log.DebugLevel)
		return nil
	}

	var logLevel log.Level

	switch levelName {
	case "trace":
		logLevel = log.TraceLevel
	case "debug":
		logLevel = log.DebugLevel
	case "info":
		logLevel = log.InfoLevel
	case "warn":
		logLevel = log.WarnLevel
	case "error":
		logLevel = log.ErrorLevel
	case "fatal":
		logLevel = log.FatalLevel
	case "panic":
		logLevel = log.PanicLevel
	default:
		return fmt.Errorf("unknown log-level %q", levelName)
	}

	log.SetLevel(logLevel)
	return nil
}
