package commands

//Flag value getters

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/docker-slim/docker-slim/pkg/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/pkg/app/master/signals"
)

func GetContainerRunOptions(ctx *cli.Context) (*config.ContainerRunOptions, error) {
	const op = "commands.GetContainerRunOptions"
	var cro config.ContainerRunOptions
	cro.Runtime = ctx.String(FlagCRORuntime)
	sysctlList := ctx.StringSlice(FlagCROSysctl)
	if len(sysctlList) > 0 {
		params, err := ParseTokenMap(sysctlList)
		if err != nil {
			log.WithFields(log.Fields{
				"op":    op,
				"error": err,
			}).Error("invalid sysctl options")
			return nil, err
		}

		cro.SysctlParams = params
	}
	hostConfigFileName := ctx.String(FlagCROHostConfigFile)
	if len(hostConfigFileName) > 0 {
		hostConfigBytes, err := ioutil.ReadFile(hostConfigFileName)
		if err != nil {
			log.WithFields(log.Fields{
				"op":        op,
				"file.name": hostConfigFileName,
				"error":     err,
			}).Error("could not read host config file")
			return nil, err
		}
		json.Unmarshal(hostConfigBytes, &cro.HostConfig)
	}

	cro.ShmSize = ctx.Int64(FlagCROShmSize)
	return &cro, nil
}

func GetDefaultHTTPProbe() config.HTTPProbeCmd {
	return config.HTTPProbeCmd{
		Protocol: "http",
		Method:   "GET",
		Resource: "/",
	}
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
	const op = "commands.GetContainerOverrides"

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
			log.WithFields(log.Fields{
				"op":    op,
				"error": err,
			}).Error("invalid expose options")
			return nil, err
		}
	}

	if len(volumesList) > 0 {
		volumes, err := ParseTokenSet(volumesList)
		if err != nil {
			log.WithFields(log.Fields{
				"op":    op,
				"error": err,
			}).Error("invalid volume options")
			return nil, err
		}

		overrides.Volumes = volumes
	}

	if len(labelsList) > 0 {
		labels, err := ParseTokenMap(labelsList)
		if err != nil {
			log.WithFields(log.Fields{
				"op":    op,
				"error": err,
			}).Error("invalid label options")
			return nil, err
		}

		overrides.Labels = labels
	}

	overrides.Entrypoint, err = ParseExec(doUseEntrypoint)
	if err != nil {
		log.WithFields(log.Fields{
			"op":    op,
			"error": err,
		}).Error("invalid entrypoint option")
		return nil, err
	}

	//TODO: use a '--no-entrypoint' flag instead of this one space hack
	overrides.ClearEntrypoint = IsOneSpace(doUseEntrypoint)

	overrides.Cmd, err = ParseExec(doUseCmd)
	if err != nil {
		log.WithFields(log.Fields{
			"op":    op,
			"error": err,
		}).Error("invalid cmd option")
		return nil, err
	}

	overrides.ClearCmd = IsOneSpace(doUseCmd)

	return overrides, nil
}

func UpdateGlobalFlagValues(appOpts *config.AppOptions, values *GenericParams) *GenericParams {
	if appOpts == nil || appOpts.Global == nil || values == nil {
		return values
	}

	if appOpts.Global.NoColor != nil {
		values.NoColor = *appOpts.Global.NoColor
	}

	if appOpts.Global.Debug != nil {
		values.Debug = *appOpts.Global.Debug
	}

	if appOpts.Global.Verbose != nil {
		values.Verbose = *appOpts.Global.Verbose
	}

	if appOpts.Global.LogLevel != nil {
		values.LogLevel = *appOpts.Global.LogLevel
	}

	if appOpts.Global.LogFormat != nil {
		values.LogFormat = *appOpts.Global.LogFormat
	}

	if appOpts.Global.Log != nil {
		values.Log = *appOpts.Global.Log
	}

	if appOpts.Global.UseTLS != nil {
		values.ClientConfig.UseTLS = *appOpts.Global.UseTLS
	}

	if appOpts.Global.VerifyTLS != nil {
		values.ClientConfig.VerifyTLS = *appOpts.Global.VerifyTLS
	}

	if appOpts.Global.TLSCertPath != nil {
		values.ClientConfig.TLSCertPath = *appOpts.Global.TLSCertPath
	}

	if appOpts.Global.Host != nil {
		values.ClientConfig.Host = *appOpts.Global.Host
	}

	return values
}

func GlobalFlagValues(ctx *cli.Context) (*GenericParams, error) {
	values := GenericParams{
		CheckVersion:   ctx.Bool(FlagCheckVersion),
		Debug:          ctx.Bool(FlagDebug),
		Verbose:        ctx.Bool(FlagVerbose),
		LogLevel:       ctx.String(FlagLogLevel),
		LogFormat:      ctx.String(FlagLogFormat),
		Log:            ctx.String(FlagLog),
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
