package binfile

import (
	"debug/elf"
	"strings"

	log "github.com/sirupsen/logrus"
)

type BinProps struct {
	IsBin bool
	IsSO  bool
	IsExe bool
}

func Detected(filePath string) (*BinProps, error) {
	binFile, err := elf.Open(filePath)
	if err == nil {
		binProps := &BinProps{
			IsBin: true,
		}

		switch binFile.Type {
		case elf.ET_EXEC:
			binProps.IsExe = true
		case elf.ET_DYN:
			binProps.IsSO = true
		}

		binFile.Close()
		return binProps, nil
	}

	log.Debugf("binfile.Detected(%v) - elf.Open error: %v", filePath, err)

	if elfErr, ok := err.(*elf.FormatError); ok {
		if strings.Contains(elfErr.Error(), "bad magic number") {
			return nil, nil
		}

		log.Debugf("binfile.Detected(%v) - malformed binary file", filePath)
		return nil, err
	}

	return nil, err
}
