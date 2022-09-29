package sensor

import (
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

func getCurrentPaths(root string) (map[string]struct{}, error) {
	pathMap := map[string]struct{}{}
	err := filepath.Walk(root,
		func(pth string, info os.FileInfo, err error) error {
			if strings.HasPrefix(pth, "/proc/") {
				log.Debugf("sensor: getCurrentPaths() - skipping /proc file system objects...")
				return filepath.SkipDir
			}

			if strings.HasPrefix(pth, "/sys/") {
				log.Debugf("sensor: getCurrentPaths() - skipping /sys file system objects...")
				return filepath.SkipDir
			}

			if strings.HasPrefix(pth, "/dev/") {
				log.Debugf("sensor: getCurrentPaths() - skipping /dev file system objects...")
				return filepath.SkipDir
			}

			if info.Mode().IsRegular() &&
				!strings.HasPrefix(pth, "/proc/") &&
				!strings.HasPrefix(pth, "/sys/") &&
				!strings.HasPrefix(pth, "/dev/") {
				pth, err := filepath.Abs(pth)
				if err == nil {
					pathMap[pth] = struct{}{}
				}
			}
			return nil
		})

	if err != nil {
		return nil, err
	}

	return pathMap, nil
}
