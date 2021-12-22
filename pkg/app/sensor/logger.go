// +build linux

package app

import log "github.com/sirupsen/logrus"

func setLogLevel(enableDebug bool, logLevelName string) {
	if enableDebug {
		log.SetLevel(log.DebugLevel)
		return
	}

	var logLevel log.Level

	switch logLevelName {
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
		log.Fatalf("unknown log-level %q", logLevelName)
	}

	log.SetLevel(logLevel)
}
