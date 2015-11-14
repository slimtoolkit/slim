package main

import (
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
)

func failOnError(err error) {
	if err != nil {
		log.Fatalln("launcher: ERROR =>", err)
	}
}

func failWhen(cond bool, msg string) {
	if cond {
		log.Fatalln("launcher: ERROR =>", msg)
	}
}

func myFileDir() string {
	dirName, err := filepath.Abs(filepath.Dir(os.Args[0]))
	failOnError(err)
	return dirName
}

func fileDir(fileName string) string {
	dirName, err := filepath.Abs(filepath.Dir(fileName))
	failOnError(err)
	return dirName
}
