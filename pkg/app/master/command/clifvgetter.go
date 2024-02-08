package command

//Flag value getters

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/config"
	"github.com/slimtoolkit/slim/pkg/app/master/signals"
	"github.com/slimtoolkit/slim/pkg/docker/dockerclient"
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
		hostConfigBytes, err := os.ReadFile(hostConfigFileName)
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

func GetHTTPProbeOptions(xc *app.ExecutionContext, ctx *cli.Context, doProbe bool) config.HTTPProbeOptions {
	opts := config.HTTPProbeOptions{
		Full: ctx.Bool(FlagHTTPProbeFull),

		StartWait:  ctx.Int(FlagHTTPProbeStartWait),
		RetryCount: ctx.Int(FlagHTTPProbeRetryCount),
		RetryWait:  ctx.Int(FlagHTTPProbeRetryWait),

		CrawlMaxDepth:       ctx.Int(FlagHTTPCrawlMaxDepth),
		CrawlMaxPageCount:   ctx.Int(FlagHTTPCrawlMaxPageCount),
		CrawlConcurrency:    ctx.Int(FlagHTTPCrawlConcurrency),
		CrawlConcurrencyMax: ctx.Int(FlagHTTPMaxConcurrentCrawlers),
	}

	if doProbe {
		opts.Do = true
	} else {
		opts.Do = ctx.Bool(FlagHTTPProbe) && !ctx.Bool(FlagHTTPProbeOff)
		opts.ExitOnFailure = ctx.Bool(FlagHTTPProbeExitOnFailure)
	}

	cmds, err := GetHTTPProbes(ctx)
	if err != nil {
		xc.Out.Error("param.http.probe", err.Error())
		xc.Out.State("exited",
			ovars{
				"exit.code": -1,
			})
		xc.Exit(-1)
	}
	opts.Cmds = cmds

	if opts.Do && len(opts.Cmds) == 0 {
		//add default probe cmd if the "http-probe" flag is set
		//but only if there are no custom http probe commands
		xc.Out.Info("param.http.probe",
			ovars{
				"message": "using default probe",
			})

		opts.Cmds = append(opts.Cmds, GetDefaultHTTPProbe())

		if ctx.Bool(FlagHTTPProbeCrawl) {
			opts.Cmds[0].Crawl = true
		}
	}

	if len(opts.Cmds) > 0 {
		opts.Do = true
	}

	ports, err := ParseHTTPProbesPorts(ctx.String(FlagHTTPProbePorts))
	if err != nil {
		xc.Out.Error("param.http.probe.ports", err.Error())
		xc.Out.State("exited",
			ovars{
				"exit.code": -1,
			})
		xc.Exit(-1)
	}
	opts.Ports = ports

	opts.APISpecs = ctx.StringSlice(FlagHTTPProbeAPISpec)
	apiSpecFiles, fileErrors := ValidateFiles(ctx.StringSlice(FlagHTTPProbeAPISpecFile))
	if len(fileErrors) > 0 {
		for k, v := range fileErrors {
			err = v
			xc.Out.Info("error",
				ovars{
					"file":  k,
					"error": err,
				})

			xc.Out.Error("param.error.http.api.spec.file", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}
	}
	opts.APISpecFiles = apiSpecFiles

	if len(opts.APISpecs)+len(opts.APISpecFiles) > 0 {
		opts.Do = true
	}

	return opts
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
	case config.CAMHostExec:
		info.Mode = config.CAMHostExec
	case config.CAMAppExit:
		info.Mode = config.CAMAppExit
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

func RemoveContinueAfterMode(continueAfter, mode string) string {
	if continueAfter == mode {
		return ""
	}

	var result []string
	modes := strings.Split(continueAfter, "&")
	for _, current := range modes {
		if current != mode {
			result = append(result, mode)
		}
	}

	return strings.Join(modes, "&")
}

func GetContinueAfterModeNames(continueAfter string) []string {
	return strings.Split(continueAfter, "&")
}

func GetContainerOverrides(xc *app.ExecutionContext, ctx *cli.Context) (*config.ContainerOverrides, error) {
	const op = "commands.GetContainerOverrides"

	doUseEntrypoint := ctx.String(FlagEntrypoint)
	doUseCmd := ctx.String(FlagCmd)
	exposePortList := ctx.StringSlice(FlagExpose)

	volumesList := ctx.StringSlice(FlagVolume)
	labelsList := ctx.StringSlice(FlagLabel)
	envList, envErr := ParseEnvFile(ctx.String(FlagEnvFile))
	if envErr != nil {
		return nil, envErr
	}
	envList = validateAndCleanEnvVariables(xc, envList, "param.env-file.value")
	env := validateAndCleanEnvVariables(xc, ctx.StringSlice(FlagEnv), "param.env")
	envList = append(envList, env...)
	overrides := &config.ContainerOverrides{
		User:     ctx.String(FlagUser),
		Workdir:  ctx.String(FlagWorkdir),
		Env:      envList,
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

	if appOpts.Global.Quiet != nil {
		values.QuietCLIMode = *appOpts.Global.Quiet
	}

	if appOpts.Global.OutputFormat != nil {
		values.OutputFormat = *appOpts.Global.OutputFormat
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

	if appOpts.Global.APIVersion != nil {
		values.ClientConfig.APIVersion = *appOpts.Global.APIVersion
	}

	return values
}

func GlobalFlagValues(ctx *cli.Context) *GenericParams {
	values := GenericParams{
		CheckVersion:   ctx.Bool(FlagCheckVersion),
		Debug:          ctx.Bool(FlagDebug),
		Verbose:        ctx.Bool(FlagVerbose),
		QuietCLIMode:   ctx.Bool(FlagQuietCLIMode),
		LogLevel:       ctx.String(FlagLogLevel),
		LogFormat:      ctx.String(FlagLogFormat),
		OutputFormat:   ctx.String(FlagOutputFormat),
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

	return &values
}

func GetDockerClientConfig(ctx *cli.Context) *config.DockerClient {
	config := &config.DockerClient{
		APIVersion:  ctx.String(FlagAPIVersion),
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

	for _, ev := range dockerclient.EnvVarNames {
		getEnv(ev)
	}

	return config
}

func validateAndCleanEnvVariables(xc *app.ExecutionContext, envList []string, errType string) []string {
	var envStaging []string

	if len(envList) == 0 {
		return envStaging
	}

	for i, kv := range envList {
		kv = strings.TrimSpace(kv)

		if len(kv) == 0 {
			continue
		}

		if !strings.ContainsAny(kv, "=") {
			xc.Out.Error(errType, fmt.Sprintf("skipping malformed env var - (index=%d data='%s')", i, kv))
			continue
		}

		envKeyValue := strings.SplitN(kv, "=", 2)
		if len(envKeyValue) != 2 {
			xc.Out.Error(errType, fmt.Sprintf("skipping malformed env var - (index=%d data='%s')", i, kv))
			continue
		}

		keyIsEmpty := len(strings.TrimSpace(envKeyValue[0])) == 0
		//no need to trim value (it may have spaces intentionally)
		valIsEmpty := len(envKeyValue[1]) == 0

		if !keyIsEmpty && !valIsEmpty {
			envStaging = append(envStaging, kv)
		} else {
			xc.Out.Error(errType, fmt.Sprintf("skipping malformed env var - (index=%d data='%s')", i, kv))
		}
	}

	return envStaging
}
