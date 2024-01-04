package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/c-bata/go-prompt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/config"
	"github.com/slimtoolkit/slim/pkg/docker/dockerutil"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
)

const (
	ImagesStateRootPath = "images"
)

var (
	ErrNoGlobalParams = errors.New("No global params")
)

type ovars = app.OutVars

/////////////////////////////////////////////////////////

type CLIContextKey int

const (
	GlobalParams CLIContextKey = 1
	AppParams    CLIContextKey = 2
)

func CLIContextSave(ctx context.Context, key CLIContextKey, data interface{}) context.Context {
	return context.WithValue(ctx, key, data)
}

func CLIContextGet(ctx context.Context, key CLIContextKey) interface{} {
	if ctx == nil {
		return nil
	}

	return ctx.Value(key)
}

/////////////////////////////////////////////////////////

type GenericParams struct {
	NoColor        bool
	CheckVersion   bool
	Debug          bool
	Verbose        bool
	QuietCLIMode   bool
	LogLevel       string
	LogFormat      string
	OutputFormat   string
	Log            string
	StatePath      string
	ReportLocation string
	InContainer    bool
	IsDSImage      bool
	ArchiveState   string
	ClientConfig   *config.DockerClient
}

// TODO: spread these code types across all command definition, so it's not all defined here
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
	ECTMerge   = 0x09000000
)

// Build command exit codes
const (
	ECCOther = iota + 1
	ECCImageNotFound
	ECCNoDockerConnectInfo
	ECCBadNetworkName
)

const (
	AppName = "slim"
	appName = "slim"
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

// /
func UpdateImageRef(logger *log.Entry, ref, override string) string {
	logger.Debugf("UpdateImageRef() - ref='%s' override='%s'", ref, override)
	if override == "" {
		return ref
	}

	refParts := strings.SplitN(ref, ":", 2)
	refImage := refParts[0]
	refTag := ""
	if len(refParts) > 1 {
		refTag = refParts[1]
	}

	overrideParts := strings.SplitN(override, ":", 2)
	switch len(overrideParts) {
	case 2:
		refImage = overrideParts[0]
		refTag = overrideParts[1]
	case 1:
		refTag = overrideParts[0]
	}

	if refTag == "" {
		//shouldn't happen
		refTag = "latest"
	}

	return fmt.Sprintf("%s:%s", refImage, refTag)
}

func RunHostExecProbes(printState bool, xc *app.ExecutionContext, hostExecProbes []string) {
	if len(hostExecProbes) > 0 {
		var callCount uint
		var okCount uint
		var errCount uint

		if printState {
			xc.Out.Info("host.exec.probes",
				ovars{
					"count": len(hostExecProbes),
				})
		}

		for idx, appCall := range hostExecProbes {
			if printState {
				xc.Out.Info("host.exec.probes",
					ovars{
						"idx": idx,
						"app": appCall,
					})
			}

			xc.Out.Info("host.exec.probe.output.start")
			//TODO LATER:
			//add more parameters and outputs for more advanced execution control capabilities
			err := exeAppCall(appCall)
			xc.Out.Info("host.exec.probe.output.end")

			callCount++
			statusCode := "error"
			callErrorStr := "none"
			if err == nil {
				okCount++
				statusCode = "ok"
			} else {
				errCount++
				callErrorStr = err.Error()
			}

			if printState {
				xc.Out.Info("host.exec.probes",
					ovars{
						"idx":    idx,
						"app":    appCall,
						"status": statusCode,
						"error":  callErrorStr,
						"time":   time.Now().UTC().Format(time.RFC3339),
					})
			}
		}
	}
}

func exeAppCall(appCall string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Second)
	defer cancel()

	appCall = strings.TrimSpace(appCall)
	args, err := shlex.Split(appCall)
	if err != nil {
		log.Errorf("exeAppCall(%s): call parse error: %v", appCall, err)
		return err
	}

	if len(args) == 0 {
		return fmt.Errorf("empty appCall")
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	//cmd.Dir = "."
	cmd.Stdin = os.Stdin

	//var outBuf, errBuf bytes.Buffer
	//cmd.Stdout = io.MultiWriter(os.Stdout, &outBuf)
	//cmd.Stderr = io.MultiWriter(os.Stderr, &errBuf)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Errorf("exeAppCall(%s): command start error: %v", appCall, err)
		return err
	}

	err = cmd.Wait()
	fmt.Printf("\n")
	if err != nil {
		log.Fatalf("exeAppCall(%s): command exited with error: %v", appCall, err)
		return err
	}

	//TODO: process outBuf and errBuf here
	return nil
}

///////////////////////////////////////

// var CLI []*cli.Command
var cliCommands []*cli.Command

func AddCLICommand(
	name string,
	cmd *cli.Command,
	cmdSuggestion prompt.Suggest,
	flagSuggestions *FlagSuggestions) {
	cliCommands = append(cliCommands, cmd)
	if flagSuggestions != nil {
		CommandFlagSuggestions[name] = flagSuggestions
	}

	if cmdSuggestion.Text != "" {
		CommandSuggestions = append(CommandSuggestions, cmdSuggestion)
	}
}

func GetCommands() []*cli.Command {
	return cliCommands
}
