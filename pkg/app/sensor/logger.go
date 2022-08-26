//go:build linux
// +build linux

package app

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

func configureLogger(enableDebug bool, levelName, format string) error {
	if err := setLogLevel(enableDebug, levelName); err != nil {
		return fmt.Errorf("failed to set log-level: %v", err)
	}

	if err := setLogFormat(format); err != nil {
		return fmt.Errorf("failed to set log format: %v", err)
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
