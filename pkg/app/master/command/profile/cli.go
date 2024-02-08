package profile

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/app/master/config"
)

const (
	Name  = "profile"
	Usage = "Collects fat image information and generates a fat container report"
	Alias = "p"
)

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: append([]cli.Flag{
		command.Cflag(command.FlagTarget),
		command.Cflag(command.FlagPull),
		command.Cflag(command.FlagDockerConfigPath),
		command.Cflag(command.FlagRegistryAccount),
		command.Cflag(command.FlagRegistrySecret),
		command.Cflag(command.FlagShowPullLogs),
		//Compose support
		command.Cflag(command.FlagComposeFile),                    //not used yet
		command.Cflag(command.FlagTargetComposeSvc),               //not used yet
		command.Cflag(command.FlagComposeSvcStartWait),            //not used yet
		command.Cflag(command.FlagTargetComposeSvcImage),          //not used yet
		command.Cflag(command.FlagComposeSvcNoPorts),              //not used yet
		command.Cflag(command.FlagDepExcludeComposeSvcAll),        //not used yet
		command.Cflag(command.FlagDepIncludeComposeSvc),           //not used yet
		command.Cflag(command.FlagDepExcludeComposeSvc),           //not used yet
		command.Cflag(command.FlagDepIncludeComposeSvcDeps),       //not used yet
		command.Cflag(command.FlagDepIncludeTargetComposeSvcDeps), //not used yet
		command.Cflag(command.FlagComposeNet),                     //not used yet
		command.Cflag(command.FlagComposeEnvNoHost),               //not used yet
		command.Cflag(command.FlagComposeEnvFile),                 //not used yet
		command.Cflag(command.FlagComposeProjectName),             //not used yet
		command.Cflag(command.FlagComposeWorkdir),                 //not used yet
		command.Cflag(command.FlagPublishPort),
		command.Cflag(command.FlagPublishExposedPorts),
		command.Cflag(command.FlagHostExec),
		command.Cflag(command.FlagHostExecFile),
		//command.Cflag(command.FlagKeepPerms),
		command.Cflag(command.FlagRunTargetAsUser),
		command.Cflag(command.FlagShowContainerLogs),
		command.Cflag(command.FlagEnableMondelLogs),
		command.Cflag(command.FlagCopyMetaArtifacts),
		command.Cflag(command.FlagRemoveFileArtifacts),
		command.Cflag(command.FlagExec),
		command.Cflag(command.FlagExecFile),
		//Container Run Options
		command.Cflag(command.FlagCRORuntime),
		command.Cflag(command.FlagCROHostConfigFile),
		command.Cflag(command.FlagCROSysctl),
		command.Cflag(command.FlagCROShmSize),
		command.Cflag(command.FlagUser),
		command.Cflag(command.FlagEntrypoint),
		command.Cflag(command.FlagCmd),
		command.Cflag(command.FlagWorkdir),
		command.Cflag(command.FlagEnv),
		command.Cflag(command.FlagEnvFile),
		command.Cflag(command.FlagLabel),
		command.Cflag(command.FlagVolume),
		command.Cflag(command.FlagLink),
		command.Cflag(command.FlagEtcHostsMap),
		command.Cflag(command.FlagContainerDNS),
		command.Cflag(command.FlagContainerDNSSearch),
		command.Cflag(command.FlagNetwork),
		command.Cflag(command.FlagHostname),
		command.Cflag(command.FlagExpose),
		//command.Cflag(command.FlagExcludeMounts),
		//command.Cflag(command.FlagExcludePattern), //should remove too (no need)
		command.Cflag(command.FlagMount),
		command.Cflag(command.FlagContinueAfter),
		command.Cflag(command.FlagUseLocalMounts),
		command.Cflag(command.FlagUseSensorVolume),
		//Sensor flags:
		command.Cflag(command.FlagSensorIPCEndpoint),
		command.Cflag(command.FlagSensorIPCMode),
	}, command.HTTPProbeFlags()...),
	Action: func(ctx *cli.Context) error {
		gcvalues := command.GlobalFlagValues(ctx)
		xc := app.NewExecutionContext(
			Name,
			gcvalues.QuietCLIMode,
			gcvalues.OutputFormat)

		targetRef := ctx.String(command.FlagTarget)
		if targetRef == "" {
			if ctx.Args().Len() < 1 {
				xc.Out.Error("param.target", "missing target image ID/name")
				cli.ShowCommandHelp(ctx, Name)
				return nil
			} else {
				targetRef = ctx.Args().First()
			}
		}

		crOpts, err := command.GetContainerRunOptions(ctx)
		if err != nil {
			xc.Out.Error("param.error.container.run.options", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		doPull := ctx.Bool(command.FlagPull)
		dockerConfigPath := ctx.String(command.FlagDockerConfigPath)
		registryAccount := ctx.String(command.FlagRegistryAccount)
		registrySecret := ctx.String(command.FlagRegistrySecret)
		doShowPullLogs := ctx.Bool(command.FlagShowPullLogs)

		doRmFileArtifacts := ctx.Bool(command.FlagRemoveFileArtifacts)
		doCopyMetaArtifacts := ctx.String(command.FlagCopyMetaArtifacts)

		portBindings, err := command.ParsePortBindings(ctx.StringSlice(command.FlagPublishPort))
		if err != nil {
			xc.Out.Error("param.publish.port", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		doPublishExposedPorts := ctx.Bool(command.FlagPublishExposedPorts)

		httpProbeOpts := command.GetHTTPProbeOptions(xc, ctx, false)

		continueAfter, err := command.GetContinueAfter(ctx)
		if err != nil {
			xc.Out.Error("param.error.continue.after", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		if !httpProbeOpts.Do && continueAfter.Mode == "probe" {
			continueAfter.Mode = "enter"
			xc.Out.Info("enter",
				ovars{
					"message": "changing continue-after from probe to enter because http-probe is disabled",
				})
		}

		hostExecProbes := ctx.StringSlice(command.FlagHostExec)
		moreHostExecProbes, err := command.ParseHTTPProbeExecFile(ctx.String(command.FlagHostExecFile))
		if err != nil {
			xc.Out.Error("param.host.exec.file", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		if len(moreHostExecProbes) > 0 {
			hostExecProbes = append(hostExecProbes, moreHostExecProbes...)
		}

		if strings.Contains(continueAfter.Mode, config.CAMHostExec) &&
			len(hostExecProbes) == 0 {
			if continueAfter.Mode == config.CAMHostExec {
				continueAfter.Mode = config.CAMEnter
				xc.Out.Info("host-exec",
					ovars{
						"message": "changing continue-after from host-exec to enter because there are no host-exec commands",
					})
			} else {
				continueAfter.Mode = command.RemoveContinueAfterMode(continueAfter.Mode, config.CAMHostExec)
				xc.Out.Info("host-exec",
					ovars{
						"message": "removing host-exec continue-after mode because there are no host-exec commands",
					})
			}
		}

		if len(hostExecProbes) > 0 {
			if !strings.Contains(continueAfter.Mode, config.CAMHostExec) {
				if continueAfter.Mode == "" {
					continueAfter.Mode = config.CAMHostExec
				} else {
					continueAfter.Mode = fmt.Sprintf("%s&%s", continueAfter.Mode, config.CAMHostExec)
				}

				xc.Out.Info("exec",
					ovars{
						"message": fmt.Sprintf("updating continue-after mode to %s", continueAfter.Mode),
					})
			}
		}

		//doKeepPerms := ctx.Bool(command.FlagKeepPerms)

		doRunTargetAsUser := ctx.Bool(command.FlagRunTargetAsUser)

		doShowContainerLogs := ctx.Bool(command.FlagShowContainerLogs)
		doEnableMondel := ctx.Bool(command.FlagEnableMondelLogs)

		overrides, err := command.GetContainerOverrides(xc, ctx)
		if err != nil {
			xc.Out.Error("param.error.container.overrides", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		volumeMounts, err := command.ParseVolumeMounts(ctx.StringSlice(command.FlagMount))
		if err != nil {
			xc.Out.Error("param.error.mount", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		//excludePatterns := command.ParsePaths(ctx.StringSlice(command.FlagExcludePattern))

		//includePaths := command.ParsePaths(ctx.StringSlice(command.FlagIncludePath))
		//moreIncludePaths, err := command.ParsePathsFile(ctx.String(command.FlagIncludePathFile))
		//if err != nil {
		//	xc.Out.Error("param.error.include.path.file", err.Error())
		//	xc.Out.State("exited",
		//		ovars{
		//			"exit.code": -1,
		//		})
		//	xc.Exit(-1)
		//} else {
		//	for k, v := range moreIncludePaths {
		//		includePaths[k] = v
		//	}
		//}

		//pathPerms := command.ParsePaths(ctx.StringSlice(command.FlagPathPerms))
		//morePathPerms, err := command.ParsePathsFile(ctx.String(command.FlagPathPermsFile))
		//if err != nil {
		//	xc.Out.Error("param.error.path.perms.file", err.Error())
		//	xc.Out.State("exited",
		//		ovars{
		//			"exit.code": -1,
		//		})
		//	xc.Exit(-1)
		//} else {
		//	for k, v := range morePathPerms {
		//		pathPerms[k] = v
		//	}
		//}

		//includeBins := command.ParsePaths(ctx.StringSlice(command.FlagIncludeBin))
		//includeExes := command.ParsePaths(ctx.StringSlice(command.FlagIncludeExe))
		//doIncludeShell := ctx.Bool(command.FlagIncludeShell)

		doUseLocalMounts := ctx.Bool(command.FlagUseLocalMounts)
		doUseSensorVolume := ctx.String(command.FlagUseSensorVolume)

		//doKeepTmpArtifacts := ctx.Bool(command.FlagKeepTmpArtifacts)

		//doExcludeMounts := ctx.Bool(command.FlagExcludeMounts)
		//if doExcludeMounts {
		//	for mpath := range volumeMounts {
		//		excludePatterns[mpath] = nil
		//		mpattern := fmt.Sprintf("%s/**", mpath)
		//		excludePatterns[mpattern] = nil
		//	}
		//}

		commandReport := ctx.String(command.FlagCommandReport)
		if commandReport == "off" {
			commandReport = ""
		}

		OnCommand(
			xc,
			gcvalues,
			targetRef,
			doPull,
			dockerConfigPath,
			registryAccount,
			registrySecret,
			doShowPullLogs,
			crOpts,
			httpProbeOpts,
			portBindings,
			doPublishExposedPorts,
			hostExecProbes,
			doRmFileArtifacts,
			doCopyMetaArtifacts,
			doRunTargetAsUser,
			doShowContainerLogs,
			doEnableMondel,
			overrides,
			ctx.StringSlice(command.FlagLink),
			ctx.StringSlice(command.FlagEtcHostsMap),
			ctx.StringSlice(command.FlagContainerDNS),
			ctx.StringSlice(command.FlagContainerDNSSearch),
			volumeMounts,
			//doKeepPerms,
			//pathPerms,
			//excludePatterns,
			//includePaths,
			//includeBins,
			//includeExes,
			//doIncludeShell,
			doUseLocalMounts,
			doUseSensorVolume,
			//doKeepTmpArtifacts,
			continueAfter,
			ctx.String(command.FlagSensorIPCEndpoint),
			ctx.String(command.FlagSensorIPCMode),
			ctx.String(command.FlagLogLevel),
			ctx.String(command.FlagLogFormat))

		return nil
	},
}
