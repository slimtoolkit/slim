package commands

import (
	"path/filepath"

	"github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
)

func copyMetaArtifacts(logger *log.Entry, names []string, artifactLocation, targetLocation string) bool {
	if targetLocation != "" {
		if !fsutil.Exists(artifactLocation) {
			logger.Debugf("copyMetaArtifacts() - bad artifact location (%v)\n", artifactLocation)
			return false
		}

		if len(names) == 0 {
			logger.Debug("copyMetaArtifacts() - no artifact names")
			return false
		}

		for _, name := range names {
			srcPath := filepath.Join(artifactLocation, name)
			if fsutil.Exists(srcPath) && fsutil.IsRegularFile(srcPath) {
				dstPath := filepath.Join(targetLocation, name)
				err := fsutil.CopyRegularFile(false, srcPath, dstPath, true)
				if err != nil {
					logger.Debugf("copyMetaArtifacts() - error saving file: %v\n", err)
					return false
				}
			}
		}

		return true
	}

	logger.Debug("copyMetaArtifacts() - no target location")
	return false
}

func confirmNetwork(logger *log.Entry, client *docker.Client, network string) bool {
	if network == "" {
		return true
	}

	if networks, err := client.ListNetworks(); err == nil {
		for _, n := range networks {
			if n.Name == network {
				return true
			}
		}
	} else {
		logger.Debugf("confirmNetwork() - error getting networks = %v", err)
	}

	return false
}
