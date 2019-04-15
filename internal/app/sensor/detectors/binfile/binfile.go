package binfile

import (
	"debug/elf"
	"strings"

	log "github.com/Sirupsen/logrus"
)

func Detected(filePath string) (bool, error) {
	binFile, err := elf.Open(filePath)
	if err == nil {
		binFile.Close()
		return true, nil
	}

	log.Debugf("binfile.Detected(%v) - elf.Open error: %v", filePath, err)

	if elfErr, ok := err.(*elf.FormatError); ok {
		if strings.Contains(elfErr.Error(), "bad magic number") {
			return false, nil
		}

		log.Debug("binfile.Detected(%v) - malformed binary file", filePath)
		return true, err
	}

	return false, err
}
