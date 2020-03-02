package commands

import (
	"fmt"
	"os"

	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/internal/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"

	log "github.com/sirupsen/logrus"
)

// OnContainerize implements the 'containerize' docker-slim command
func OnContainerize(
	doCheckVersion bool,
	cmdReportLocation string,
	doDebug bool,
	statePath string,
	archiveState string,
	inContainer bool,
	isDSImage bool,
	clientConfig *config.DockerClient,
	targetRef string,
	ec *ExecutionContext) {
	const cmdName = "containerize"
	logger := log.WithFields(log.Fields{"app": appName, "command": cmdName})
	prefix := fmt.Sprintf("%s[%s]:", appName, cmdName)

	viChan := version.CheckAsync(doCheckVersion, inContainer, isDSImage)

	cmdReport := report.NewContainerizeCommand(cmdReportLocation)
	cmdReport.State = report.CmdStateStarted

	fmt.Printf("%s[%s]: state=started\n", appName, cmdName)
	fmt.Printf("%s[%s]: info=params target=%v\n", appName, cmdName, targetRef)

	client, err := dockerclient.New(clientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if inContainer && isDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
		}
		fmt.Printf("%s[%s]: info=docker.connect.error message='%s'\n", appName, cmdName, exitMsg)
		fmt.Printf("%s[%s]: state=exited version=%s location='%s'\n", appName, cmdName, v.Current(), fsutil.ExeDir())
		os.Exit(ectCommon | ecNoDockerConnectInfo)
	}
	errutil.FailOn(err)

	if doDebug {
		version.Print(prefix, logger, client, false, inContainer, isDSImage)
	}

	fmt.Printf("%s[%s]: state=completed\n", appName, cmdName)
	cmdReport.State = report.CmdStateCompleted

	fmt.Printf("%s[%s]: state=done\n", appName, cmdName)

	vinfo := <-viChan
	version.PrintCheckVersion(prefix, vinfo)

	cmdReport.State = report.CmdStateDone
	if cmdReport.Save() {
		fmt.Printf("%s[%s]: info=report file='%s'\n", appName, cmdName, cmdReport.ReportLocation())
	}
}
