package commands

//Flag value getters

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker-slim/docker-slim/pkg/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/pkg/app/master/signals"

	"github.com/urfave/cli/v2"
)

func GetContainerRunOptions(ctx *cli.Context) (*config.ContainerRunOptions, error) {
	var cro config.ContainerRunOptions
	cro.Runtime = ctx.String(FlagCRORuntime)
	sysctlList := ctx.StringSlice(FlagCROSysctl)
	if len(sysctlList) > 0 {
		params, err := ParseTokenMap(sysctlList)
		if err != nil {
			fmt.Printf("invalid sysctl options %v\n", err)
			return nil, err
		}

		cro.SysctlParams = params
	}
	hostConfigFileName := ctx.String(FlagCROHostConfigFile)
	if len(hostConfigFileName) > 0 {
		hostConfigBytes, err := ioutil.ReadFile(hostConfigFileName)
		if err != nil {
			fmt.Printf("could not read host config file %v: %v\n", hostConfigFileName, err)
		}
		json.Unmarshal(hostConfigBytes, &cro.HostConfig)
	}

	cro.ShmSize = ctx.Int64(FlagCROShmSize)
	return &cro, nil
}

func GetHTTPProbes(ctx *cli.Context) ([]config.HTTPProbeCmd, error) {
	httpProbeCmds, err := ParseHTTPProbes(ctx.StringSlice(FlagHTTPProbeCmd))
	if err != nil {
		return nil, err
	}

	moreHTTPProbeCmds, err := ParseHTTPProbesFile(ctx.String(FlagHTTPProbeCmdFile))
	if err != nil {
		return nil, err
	}

	if moreHTTPProbeCmds != nil {
		httpProbeCmds = append(httpProbeCmds, moreHTTPProbeCmds...)
	}

	return httpProbeCmds, nil
}

func GetContinueAfter(ctx *cli.Context) (*config.ContinueAfter, error) {
	info := &config.ContinueAfter{
		Mode: config.CAMEnter,
	}

	doContinueAfter := ctx.String(FlagContinueAfter)
	switch doContinueAfter {
	case config.CAMEnter:
		info.Mode = config.CAMEnter
	case config.CAMSignal:
		info.Mode = config.CAMSignal
		info.ContinueChan = signals.AppContinueChan
	case config.CAMProbe:
		info.Mode = config.CAMProbe
	case config.CAMExec:
		info.Mode = config.CAMExec
	case config.CAMContainerProbe:
		info.Mode = config.CAMContainerProbe
	case config.CAMTimeout:
		info.Mode = config.CAMTimeout
		info.Timeout = 60
	default:
		modes := strings.Split(doContinueAfter, "&")
		if len(modes) > 1 {
			//not supporting combining signal or custom timeout modes with other modes
			info.Mode = doContinueAfter
		} else {
			if waitTime, err := strconv.Atoi(doContinueAfter); err == nil && waitTime > 0 {
				info.Mode = config.CAMTimeout
				info.Timeout = time.Duration(waitTime)
			}
		}
	}

	return info, nil
}

func GetContainerOverrides(ctx *cli.Context) (*config.ContainerOverrides, error) {
	doUseEntrypoint := ctx.String(FlagEntrypoint)
	doUseCmd := ctx.String(FlagCmd)
	exposePortList := ctx.StringSlice(FlagExpose)

	volumesList := ctx.StringSlice(FlagVolume)
	labelsList := ctx.StringSlice(FlagLabel)

	overrides := &config.ContainerOverrides{
		User:     ctx.String(FlagUser),
		Workdir:  ctx.String(FlagWorkdir),
		Env:      ctx.StringSlice(FlagEnv),
		Network:  ctx.String(FlagNetwork),
		Hostname: ctx.String(FlagHostname),
	}

	var err error
	if len(exposePortList) > 0 {
		overrides.ExposedPorts, err = ParseDockerExposeOpt(exposePortList)
		if err != nil {
			fmt.Printf("invalid expose options..\n\n")
			return nil, err
		}
	}

	if len(volumesList) > 0 {
		volumes, err := ParseTokenSet(volumesList)
		if err != nil {
			fmt.Printf("invalid volume options %v\n", err)
			return nil, err
		}

		overrides.Volumes = volumes
	}

	if len(labelsList) > 0 {
		labels, err := ParseTokenMap(labelsList)
		if err != nil {
			fmt.Printf("invalid label options %v\n", err)
			return nil, err
		}

		overrides.Labels = labels
	}

	overrides.Entrypoint, err = ParseExec(doUseEntrypoint)
	if err != nil {
		fmt.Printf("invalid entrypoint option..\n\n")
		return nil, err
	}

	//TODO: use a '--no-entrypoint' flag instead of this one space hack
	overrides.ClearEntrypoint = IsOneSpace(doUseEntrypoint)

	overrides.Cmd, err = ParseExec(doUseCmd)
	if err != nil {
		fmt.Printf("invalid cmd option..\n\n")
		return nil, err
	}

	overrides.ClearCmd = IsOneSpace(doUseCmd)

	return overrides, nil
}

func GlobalFlagValues(ctx *cli.Context) (*GenericParams, error) {
	values := GenericParams{
		CheckVersion:   ctx.Bool(FlagCheckVersion),
		Debug:          ctx.Bool(FlagDebug),
		StatePath:      ctx.String(FlagStatePath),
		ReportLocation: ctx.String(FlagCommandReport),
	}

	if values.ReportLocation == "off" {
		values.ReportLocation = ""
	}

	values.InContainer, values.IsDSImage = IsInContainer(ctx.Bool(FlagInContainer))
	values.ArchiveState = ArchiveState(ctx.String(FlagArchiveState), values.InContainer)

	values.ClientConfig = GetDockerClientConfig(ctx)

	return &values, nil
}

func GetDockerClientConfig(ctx *cli.Context) *config.DockerClient {
	config := &config.DockerClient{
		UseTLS:      ctx.Bool(FlagUseTLS),
		VerifyTLS:   ctx.Bool(FlagVerifyTLS),
		TLSCertPath: ctx.String(FlagTLSCertPath),
		Host:        ctx.String(FlagHost),
		Env:         map[string]string{},
	}

	getEnv := func(name string) {
		if value, exists := os.LookupEnv(name); exists {
			config.Env[name] = value
		}
	}

	getEnv(dockerclient.EnvDockerHost)
	getEnv(dockerclient.EnvDockerTLSVerify)
	getEnv(dockerclient.EnvDockerCertPath)

	return config
}
