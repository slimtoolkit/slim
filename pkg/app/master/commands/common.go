package commands

import (
	"path/filepath"

	docker "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/docker-slim/docker-slim/pkg/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
)

const (
	ImagesStateRootPath = "images"
)

/////////////////////////////////////////////////////////

type GenericParams struct {
	CheckVersion   bool
	Debug          bool
	StatePath      string
	ReportLocation string
	InContainer    bool
	IsDSImage      bool
	ArchiveState   string
	ClientConfig   *config.DockerClient
}

// Exit Code Types
const (
	ECTCommon  = 0x01000000
	ECTBuild   = 0x02000000
	ectProfile = 0x03000000
	ectInfo    = 0x04000000
	ectUpdate  = 0x05000000
	ectVersion = 0x06000000
	ECTXray    = 0x07000000
	ECTRun     = 0x08000000
)

// Build command exit codes
const (
	ecOther = iota + 1
	ECNoDockerConnectInfo
	ECBadNetworkName
)

const (
	AppName = "docker-slim"
	appName = "docker-slim"
)

//Common command handler code

func DoArchiveState(logger *log.Entry, client *docker.Client, localStatePath, volumeName, stateKey string) error {
	if volumeName == "" {
		return nil
	}

	err := dockerutil.HasVolume(client, volumeName)
	switch {
	case err == nil:
		logger.Debugf("archiveState: already have volume = %v", volumeName)
	case err == dockerutil.ErrNotFound:
		logger.Debugf("archiveState: no volume yet = %v", volumeName)
		if dockerutil.HasEmptyImage(client) == dockerutil.ErrNotFound {
			err := dockerutil.BuildEmptyImage(client)
			if err != nil {
				logger.Debugf("archiveState: dockerutil.BuildEmptyImage() - error = %v", err)
				return err
			}
		}

		err = dockerutil.CreateVolumeWithData(client, "", volumeName, nil)
		if err != nil {
			logger.Debugf("archiveState: dockerutil.CreateVolumeWithData() - error = %v", err)
			return err
		}
	default:
		logger.Debugf("archiveState: dockerutil.HasVolume() - error = %v", err)
		return err
	}

	return dockerutil.CopyToVolume(client, volumeName, localStatePath, ImagesStateRootPath, stateKey)
}

func CopyMetaArtifacts(logger *log.Entry, names []string, artifactLocation, targetLocation string) bool {
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

func ConfirmNetwork(logger *log.Entry, client *docker.Client, network string) bool {
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

var CLI []*cli.Command
