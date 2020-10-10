package commands

//Flag value getters

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/internal/app/master/signals"

	"github.com/urfave/cli"
)

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
		Mode: "enter",
	}

	doContinueAfter := ctx.String(FlagContinueAfter)
	switch doContinueAfter {
	case "enter":
		info.Mode = "enter"
	case "signal":
		info.Mode = "signal"
		info.ContinueChan = signals.AppContinueChan
	case "probe":
		info.Mode = "probe"
	case "timeout":
		info.Mode = "timeout"
		info.Timeout = 60
	default:
		if waitTime, err := strconv.Atoi(doContinueAfter); err == nil && waitTime > 0 {
			info.Mode = "timeout"
			info.Timeout = time.Duration(waitTime)
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

func GlobalCommandFlagValues(ctx *cli.Context) (*GenericParams, error) {
	values := GenericParams{
		CheckVersion:   ctx.GlobalBool(FlagCheckVersion),
		Debug:          ctx.GlobalBool(FlagDebug),
		StatePath:      ctx.GlobalString(FlagStatePath),
		ReportLocation: ctx.GlobalString(FlagCommandReport),
	}

	if values.ReportLocation == "off" {
		values.ReportLocation = ""
	}

	values.InContainer, values.IsDSImage = IsInContainer(ctx.GlobalBool(FlagInContainer))
	values.ArchiveState = ArchiveState(ctx.GlobalString(FlagArchiveState), values.InContainer)

	values.ClientConfig = GetDockerClientConfig(ctx)

	return &values, nil
}

func GetDockerClientConfig(ctx *cli.Context) *config.DockerClient {
	config := &config.DockerClient{
		UseTLS:      ctx.GlobalBool(FlagUseTLS),
		VerifyTLS:   ctx.GlobalBool(FlagVerifyTLS),
		TLSCertPath: ctx.GlobalString(FlagTLSCertPath),
		Host:        ctx.GlobalString(FlagHost),
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
