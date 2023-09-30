package profile

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/config"
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
		commands.Cflag(commands.FlagTarget),
		commands.Cflag(commands.FlagPull),
		commands.Cflag(commands.FlagDockerConfigPath),
		commands.Cflag(commands.FlagRegistryAccount),
		commands.Cflag(commands.FlagRegistrySecret),
		commands.Cflag(commands.FlagShowPullLogs),
		//Compose support
		commands.Cflag(commands.FlagComposeFile),                    //not used yet
		commands.Cflag(commands.FlagTargetComposeSvc),               //not used yet
		commands.Cflag(commands.FlagComposeSvcStartWait),            //not used yet
		commands.Cflag(commands.FlagTargetComposeSvcImage),          //not used yet
		commands.Cflag(commands.FlagComposeSvcNoPorts),              //not used yet
		commands.Cflag(commands.FlagDepExcludeComposeSvcAll),        //not used yet
		commands.Cflag(commands.FlagDepIncludeComposeSvc),           //not used yet
		commands.Cflag(commands.FlagDepExcludeComposeSvc),           //not used yet
		commands.Cflag(commands.FlagDepIncludeComposeSvcDeps),       //not used yet
		commands.Cflag(commands.FlagDepIncludeTargetComposeSvcDeps), //not used yet
		commands.Cflag(commands.FlagComposeNet),                     //not used yet
		commands.Cflag(commands.FlagComposeEnvNoHost),               //not used yet
		commands.Cflag(commands.FlagComposeEnvFile),                 //not used yet
		commands.Cflag(commands.FlagComposeProjectName),             //not used yet
		commands.Cflag(commands.FlagComposeWorkdir),                 //not used yet
		commands.Cflag(commands.FlagPublishPort),
		commands.Cflag(commands.FlagPublishExposedPorts),
		commands.Cflag(commands.FlagHostExec),
		commands.Cflag(commands.FlagHostExecFile),
		//commands.Cflag(commands.FlagKeepPerms),
		commands.Cflag(commands.FlagRunTargetAsUser),
		commands.Cflag(commands.FlagShowContainerLogs),
		commands.Cflag(commands.FlagCopyMetaArtifacts),
		commands.Cflag(commands.FlagRemoveFileArtifacts),
		commands.Cflag(commands.FlagExec),
		commands.Cflag(commands.FlagExecFile),
		//Container Run Options
		commands.Cflag(commands.FlagCRORuntime),
		commands.Cflag(commands.FlagCROHostConfigFile),
		commands.Cflag(commands.FlagCROSysctl),
		commands.Cflag(commands.FlagCROShmSize),
		commands.Cflag(commands.FlagUser),
		commands.Cflag(commands.FlagEntrypoint),
		commands.Cflag(commands.FlagCmd),
		commands.Cflag(commands.FlagWorkdir),
		commands.Cflag(commands.FlagEnv),
		commands.Cflag(commands.FlagEnvFile),
		commands.Cflag(commands.FlagLabel),
		commands.Cflag(commands.FlagVolume),
		commands.Cflag(commands.FlagLink),
		commands.Cflag(commands.FlagEtcHostsMap),
		commands.Cflag(commands.FlagContainerDNS),
		commands.Cflag(commands.FlagContainerDNSSearch),
		commands.Cflag(commands.FlagNetwork),
		commands.Cflag(commands.FlagHostname),
		commands.Cflag(commands.FlagExpose),
		commands.Cflag(commands.FlagExcludeMounts),
		commands.Cflag(commands.FlagExcludePattern), //should remove too (no need)
		commands.Cflag(commands.FlagMount),
		commands.Cflag(commands.FlagContinueAfter),
		commands.Cflag(commands.FlagUseLocalMounts),
		commands.Cflag(commands.FlagUseSensorVolume),
		//Sensor flags:
		commands.Cflag(commands.FlagSensorIPCEndpoint),
		commands.Cflag(commands.FlagSensorIPCMode),
	}, commands.HTTPProbeFlags()...),
	Action: func(ctx *cli.Context) error {
		xc := app.NewExecutionContext(Name, ctx.String(commands.FlagConsoleFormat))

		targetRef := ctx.String(commands.FlagTarget)
		if targetRef == "" {
			if ctx.Args().Len() < 1 {
				xc.Out.Error("param.target", "missing target image ID/name")
				cli.ShowCommandHelp(ctx, Name)
				return nil
			} else {
				targetRef = ctx.Args().First()
			}
		}

		gcvalues, err := commands.GlobalFlagValues(ctx)
		if err != nil {
			return err
		}

		crOpts, err := commands.GetContainerRunOptions(ctx)
		if err != nil {
			xc.Out.Error("param.error.container.run.options", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		doPull := ctx.Bool(commands.FlagPull)
		dockerConfigPath := ctx.String(commands.FlagDockerConfigPath)
		registryAccount := ctx.String(commands.FlagRegistryAccount)
		registrySecret := ctx.String(commands.FlagRegistrySecret)
		doShowPullLogs := ctx.Bool(commands.FlagShowPullLogs)

		doRmFileArtifacts := ctx.Bool(commands.FlagRemoveFileArtifacts)
		doCopyMetaArtifacts := ctx.String(commands.FlagCopyMetaArtifacts)

		portBindings, err := commands.ParsePortBindings(ctx.StringSlice(commands.FlagPublishPort))
		if err != nil {
			xc.Out.Error("param.publish.port", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		doPublishExposedPorts := ctx.Bool(commands.FlagPublishExposedPorts)

		httpProbeOpts := commands.GetHTTPProbeOptions(xc, ctx)

		continueAfter, err := commands.GetContinueAfter(ctx)
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

		hostExecProbes := ctx.StringSlice(commands.FlagHostExec)
		moreHostExecProbes, err := commands.ParseHTTPProbeExecFile(ctx.String(commands.FlagHostExecFile))
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
				continueAfter.Mode = commands.RemoveContinueAfterMode(continueAfter.Mode, config.CAMHostExec)
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

		//doKeepPerms := ctx.Bool(commands.FlagKeepPerms)

		doRunTargetAsUser := ctx.Bool(commands.FlagRunTargetAsUser)

		doShowContainerLogs := ctx.Bool(commands.FlagShowContainerLogs)
		overrides, err := commands.GetContainerOverrides(xc, ctx)
		if err != nil {
			xc.Out.Error("param.error.container.overrides", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		volumeMounts, err := commands.ParseVolumeMounts(ctx.StringSlice(commands.FlagMount))
		if err != nil {
			xc.Out.Error("param.error.mount", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		excludePatterns := commands.ParsePaths(ctx.StringSlice(commands.FlagExcludePattern))

		//includePaths := commands.ParsePaths(ctx.StringSlice(commands.FlagIncludePath))
		//moreIncludePaths, err := commands.ParsePathsFile(ctx.String(commands.FlagIncludePathFile))
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

		//pathPerms := commands.ParsePaths(ctx.StringSlice(commands.FlagPathPerms))
		//morePathPerms, err := commands.ParsePathsFile(ctx.String(commands.FlagPathPermsFile))
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

		//includeBins := commands.ParsePaths(ctx.StringSlice(commands.FlagIncludeBin))
		//includeExes := commands.ParsePaths(ctx.StringSlice(commands.FlagIncludeExe))
		//doIncludeShell := ctx.Bool(commands.FlagIncludeShell)

		doUseLocalMounts := ctx.Bool(commands.FlagUseLocalMounts)
		doUseSensorVolume := ctx.String(commands.FlagUseSensorVolume)

		//doKeepTmpArtifacts := ctx.Bool(commands.FlagKeepTmpArtifacts)

		doExcludeMounts := ctx.Bool(commands.FlagExcludeMounts)
		if doExcludeMounts {
			for mpath := range volumeMounts {
				excludePatterns[mpath] = nil
				mpattern := fmt.Sprintf("%s/**", mpath)
				excludePatterns[mpattern] = nil
			}
		}

		commandReport := ctx.String(commands.FlagCommandReport)
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
			overrides,
			ctx.StringSlice(commands.FlagLink),
			ctx.StringSlice(commands.FlagEtcHostsMap),
			ctx.StringSlice(commands.FlagContainerDNS),
			ctx.StringSlice(commands.FlagContainerDNSSearch),
			volumeMounts,
			//doKeepPerms,
			//pathPerms,
			excludePatterns,
			//includePaths,
			//includeBins,
			//includeExes,
			//doIncludeShell,
			doUseLocalMounts,
			doUseSensorVolume,
			//doKeepTmpArtifacts,
			continueAfter,
			ctx.String(commands.FlagSensorIPCEndpoint),
			ctx.String(commands.FlagSensorIPCMode),
			ctx.String(commands.FlagLogLevel),
			ctx.String(commands.FlagLogFormat))

		return nil
	},
}
