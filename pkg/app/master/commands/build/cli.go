package build

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"

	"github.com/urfave/cli"
)

const (
	Name  = "build"
	Usage = "Analyzes, profiles and optimizes your container image auto-generating Seccomp and AppArmor security profiles"
	Alias = "b"
)

var CLI = cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		commands.Cflag(commands.FlagTarget),
		commands.Cflag(commands.FlagPull),
		commands.Cflag(commands.FlagDockerConfigPath),
		commands.Cflag(commands.FlagRegistryAccount),
		commands.Cflag(commands.FlagRegistrySecret),
		commands.Cflag(commands.FlagShowPullLogs),
		commands.Cflag(commands.FlagComposeFile),
		commands.Cflag(commands.FlagTargetComposeSvc),
		commands.Cflag(commands.FlagComposeSvcNoPorts),
		commands.Cflag(commands.FlagDepExcludeComposeSvcAll),
		commands.Cflag(commands.FlagDepIncludeComposeSvc),
		commands.Cflag(commands.FlagDepExcludeComposeSvc),
		commands.Cflag(commands.FlagDepIncludeComposeSvcDeps),
		commands.Cflag(commands.FlagComposeNet),
		commands.Cflag(commands.FlagHTTPProbeOff),
		commands.Cflag(commands.FlagHTTPProbe),
		commands.Cflag(commands.FlagHTTPProbeCmd),
		commands.Cflag(commands.FlagHTTPProbeCmdFile),
		commands.Cflag(commands.FlagHTTPProbeRetryCount),
		commands.Cflag(commands.FlagHTTPProbeRetryWait),
		commands.Cflag(commands.FlagHTTPProbePorts),
		commands.Cflag(commands.FlagHTTPProbeFull),
		commands.Cflag(commands.FlagHTTPProbeExitOnFailure),
		commands.Cflag(commands.FlagHTTPProbeCrawl),
		commands.Cflag(commands.FlagHTTPCrawlMaxDepth),
		commands.Cflag(commands.FlagHTTPCrawlMaxPageCount),
		commands.Cflag(commands.FlagHTTPCrawlConcurrency),
		commands.Cflag(commands.FlagHTTPMaxConcurrentCrawlers),
		commands.Cflag(commands.FlagHTTPProbeAPISpec),
		commands.Cflag(commands.FlagHTTPProbeAPISpecFile),
		commands.Cflag(commands.FlagHTTPProbeExec),
		commands.Cflag(commands.FlagHTTPProbeExecFile),
		commands.Cflag(commands.FlagPublishPort),
		commands.Cflag(commands.FlagPublishExposedPorts),
		commands.Cflag(commands.FlagRunTargetAsUser),
		commands.Cflag(commands.FlagShowContainerLogs),
		cflag(FlagShowBuildLogs),
		commands.Cflag(commands.FlagCopyMetaArtifacts),
		commands.Cflag(commands.FlagRemoveFileArtifacts),
		commands.Cflag(commands.FlagExec),
		commands.Cflag(commands.FlagExecFile),
		//
		cflag(FlagTag),
		cli.StringFlag{
			Name:   FlagImageOverrides,
			Value:  "",
			Usage:  FlagImageOverridesUsage,
			EnvVar: "DSLIM_TARGET_OVERRIDES",
		},
		//Container Run Options
		commands.Cflag(commands.FlagCRORuntime),
		commands.Cflag(commands.FlagCROHostConfigFile),
		commands.Cflag(commands.FlagCROSysctl),
		commands.Cflag(commands.FlagCROShmSize),
		commands.Cflag(commands.FlagEntrypoint),
		commands.Cflag(commands.FlagCmd),
		commands.Cflag(commands.FlagWorkdir),
		commands.Cflag(commands.FlagEnv),
		commands.Cflag(commands.FlagLabel),
		commands.Cflag(commands.FlagVolume),
		commands.Cflag(commands.FlagLink),
		commands.Cflag(commands.FlagEtcHostsMap),
		commands.Cflag(commands.FlagContainerDNS),
		commands.Cflag(commands.FlagContainerDNSSearch),
		commands.Cflag(commands.FlagNetwork),
		commands.Cflag(commands.FlagHostname),
		commands.Cflag(commands.FlagExpose),
		commands.Cflag(commands.FlagMount),
		//Container Build Options
		cflag(FlagBuildFromDockerfile),
		cflag(FlagDockerfileContext),
		cflag(FlagTagFat),
		cflag(FlagCBOAddHost),
		cflag(FlagCBOBuildArg),
		cflag(FlagCBOCacheFrom),
		cflag(FlagCBOLabel),
		cflag(FlagCBOTarget),
		cflag(FlagCBONetwork),
		cflag(FlagDeleteFatImage),
		//New/Optimized Build Options
		cflag(FlagNewEntrypoint),
		cflag(FlagNewCmd),
		cflag(FlagNewExpose),
		cflag(FlagNewWorkdir),
		cflag(FlagNewEnv),
		cflag(FlagNewVolume),
		cflag(FlagNewLabel),
		cli.StringSliceFlag{
			Name:   FlagRemoveExpose,
			Value:  &cli.StringSlice{},
			Usage:  FlagRemoveExposeUsage,
			EnvVar: "DSLIM_RM_EXPOSE",
		},
		cli.StringSliceFlag{
			Name:   FlagRemoveEnv,
			Value:  &cli.StringSlice{},
			Usage:  FlagRemoveEnvUsage,
			EnvVar: "DSLIM_RM_ENV",
		},
		cli.StringSliceFlag{
			Name:   FlagRemoveLabel,
			Value:  &cli.StringSlice{},
			Usage:  FlagRemoveLabelUsage,
			EnvVar: "DSLIM_RM_LABEL",
		},
		cli.StringSliceFlag{
			Name:   FlagRemoveVolume,
			Value:  &cli.StringSlice{},
			Usage:  FlagRemoveVolumeUsage,
			EnvVar: "DSLIM_RM_VOLUME",
		},
		commands.Cflag(commands.FlagExcludeMounts),
		commands.Cflag(commands.FlagExcludePattern),
		cflag(FlagPreservePath),
		cflag(FlagPreservePathFile),
		cflag(FlagIncludePath),
		cflag(FlagIncludePathFile),
		cflag(FlagIncludeBin),
		cli.StringFlag{
			Name:   FlagIncludeBinFile,
			Value:  "",
			Usage:  FlagIncludeBinFileUsage,
			EnvVar: "DSLIM_INCLUDE_BIN_FILE",
		},
		cflag(FlagIncludeExe),
		cli.StringFlag{
			Name:   FlagIncludeExeFile,
			Value:  "",
			Usage:  FlagIncludeExeFileUsage,
			EnvVar: "DSLIM_INCLUDE_EXE_FILE",
		},
		cflag(FlagIncludeShell),
		cflag(FlagIncludeCertAll),
		cflag(FlagIncludeCertBundles),
		cflag(FlagIncludeCertDirs),
		cflag(FlagIncludeCertPKAll),
		cflag(FlagIncludeCertPKDirs),
		cflag(FlagKeepTmpArtifacts),
		cflag(FlagKeepPerms),
		cflag(FlagPathPerms),
		cflag(FlagPathPermsFile),
		commands.Cflag(commands.FlagContinueAfter),
		commands.Cflag(commands.FlagUseLocalMounts),
		commands.Cflag(commands.FlagUseSensorVolume),
	},
	Action: func(ctx *cli.Context) error {
		xc := app.NewExecutionContext(Name)

		cbOpts, err := GetContainerBuildOptions(ctx)
		if err != nil {
			xc.Out.Error("param.error.container.build.options", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		composeFile := ctx.String(commands.FlagComposeFile)
		//todo: load/parse compose file and then use it to validate the related compose params
		targetComposeSvc := ctx.String(commands.FlagTargetComposeSvc)
		composeSvcNoPorts := ctx.Bool(commands.FlagComposeSvcNoPorts)
		depExcludeComposeSvcAll := ctx.Bool(commands.FlagDepExcludeComposeSvcAll)
		depIncludeComposeSvcDeps := ctx.String(commands.FlagDepIncludeComposeSvcDeps)
		depIncludeComposeSvcs := ctx.StringSlice(commands.FlagDepIncludeComposeSvc)
		depExcludeComposeSvcs := ctx.StringSlice(commands.FlagDepExcludeComposeSvc)
		composeNets := ctx.StringSlice(commands.FlagComposeNet)
		var targetRef string
		deleteFatImage := ctx.Bool(commands.FlagDeleteFatImage)

		if cbOpts.Dockerfile == "" {
			deleteFatImage = false
		}

		if composeFile != "" && targetComposeSvc != "" {
			targetRef = targetComposeSvc
		} else {
			if cbOpts.Dockerfile == "" {
				targetRef = ctx.String(commands.FlagTarget)

				if targetRef == "" {
					if len(ctx.Args()) < 1 {
						xc.Out.Error("param.target", "missing image ID/name")
						cli.ShowCommandHelp(ctx, Name)
						return nil
					} else {
						targetRef = ctx.Args().First()
					}
				}
			} else {
				targetRef = cbOpts.DockerfileContext
				if targetRef == "" {
					if len(ctx.Args()) < 1 {
						xc.Out.Error("param.target", "missing Dockerfile build context directory")
						cli.ShowCommandHelp(ctx, Name)
						return nil
					} else {
						targetRef = ctx.Args().First()
					}
				}
			}
		}

		if targetRef == "" {
			xc.Out.Error("param.target", "missing target - make sure to set one of the target params")
			cli.ShowCommandHelp(ctx, Name)
			return nil
		}

		gcvalues, err := commands.GlobalFlagValues(ctx)
		if err != nil {
			xc.Out.Error("param.global", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
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

		httpCrawlMaxDepth := ctx.Int(commands.FlagHTTPCrawlMaxDepth)
		httpCrawlMaxPageCount := ctx.Int(commands.FlagHTTPCrawlMaxPageCount)
		httpCrawlConcurrency := ctx.Int(commands.FlagHTTPCrawlConcurrency)
		httpMaxConcurrentCrawlers := ctx.Int(commands.FlagHTTPMaxConcurrentCrawlers)
		doHTTPProbeCrawl := ctx.Bool(commands.FlagHTTPProbeCrawl)

		doHTTPProbe := ctx.Bool(commands.FlagHTTPProbe)
		if doHTTPProbe && ctx.Bool(commands.FlagHTTPProbeOff) {
			doHTTPProbe = false
		}

		httpProbeCmds, err := commands.GetHTTPProbes(ctx)
		if err != nil {
			xc.Out.Error("param.http.probe", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		if doHTTPProbe {
			//add default probe cmd if the "http-probe" flag is set
			xc.Out.Info("param.http.probe",
				ovars{
					"message": "using default probe",
				})

			defaultCmd := config.HTTPProbeCmd{
				Protocol: "http",
				Method:   "GET",
				Resource: "/",
			}

			if doHTTPProbeCrawl {
				defaultCmd.Crawl = true
			}
			httpProbeCmds = append(httpProbeCmds, defaultCmd)
		}

		if len(httpProbeCmds) > 0 {
			doHTTPProbe = true
		}

		httpProbeRetryCount := ctx.Int(commands.FlagHTTPProbeRetryCount)
		httpProbeRetryWait := ctx.Int(commands.FlagHTTPProbeRetryWait)
		httpProbePorts, err := commands.ParseHTTPProbesPorts(ctx.String(commands.FlagHTTPProbePorts))
		if err != nil {
			xc.Out.Error("param.http.probe.ports", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		doHTTPProbeFull := ctx.Bool(commands.FlagHTTPProbeFull)
		doHTTPProbeExitOnFailure := ctx.Bool(commands.FlagHTTPProbeExitOnFailure)

		httpProbeAPISpecs := ctx.StringSlice(commands.FlagHTTPProbeAPISpec)
		if len(httpProbeAPISpecs) > 0 {
			doHTTPProbe = true
		}

		httpProbeAPISpecFiles, fileErrors := commands.ValidateFiles(ctx.StringSlice(commands.FlagHTTPProbeAPISpecFile))
		if len(fileErrors) > 0 {
			var err error
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

			return err
		}

		if len(httpProbeAPISpecFiles) > 0 {
			doHTTPProbe = true
		}

		httpProbeApps := ctx.StringSlice(commands.FlagHTTPProbeExec)
		moreProbeApps, err := commands.ParseHTTPProbeExecFile(ctx.String(commands.FlagHTTPProbeExecFile))
		if err != nil {
			xc.Out.Error("param.http.probe.exec.file", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		if len(moreProbeApps) > 0 {
			httpProbeApps = append(httpProbeApps, moreProbeApps...)
		}

		doKeepPerms := ctx.Bool(FlagKeepPerms)

		doRunTargetAsUser := ctx.Bool(commands.FlagRunTargetAsUser)

		doShowContainerLogs := ctx.Bool(commands.FlagShowContainerLogs)
		doShowBuildLogs := ctx.Bool(FlagShowBuildLogs)
		outputTags := ctx.StringSlice(FlagTag)

		doImageOverrides := ctx.String(FlagImageOverrides)
		overrides, err := commands.GetContainerOverrides(ctx)
		if err != nil {
			xc.Out.Error("param.error.image.overrides", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		instructions, err := GetImageInstructions(ctx)
		if err != nil {
			xc.Out.Error("param.error.image.instructions", err.Error())
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

		preservePaths := commands.ParsePaths(ctx.StringSlice(FlagPreservePath))
		morePreservePaths, err := commands.ParsePathsFile(ctx.String(FlagPreservePathFile))
		if err != nil {
			xc.Out.Error("param.error.preserve.path.file", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		} else {
			for k, v := range morePreservePaths {
				preservePaths[k] = v
			}
		}

		includePaths := commands.ParsePaths(ctx.StringSlice(FlagIncludePath))
		moreIncludePaths, err := commands.ParsePathsFile(ctx.String(FlagIncludePathFile))
		if err != nil {
			xc.Out.Error("param.error.include.path.file", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		} else {
			for k, v := range moreIncludePaths {
				includePaths[k] = v
			}
		}

		pathPerms := commands.ParsePaths(ctx.StringSlice(FlagPathPerms))
		morePathPerms, err := commands.ParsePathsFile(ctx.String(FlagPathPermsFile))
		if err != nil {
			xc.Out.Error("param.error.path.perms.file", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		} else {
			for k, v := range morePathPerms {
				pathPerms[k] = v
			}
		}

		includeBins := commands.ParsePaths(ctx.StringSlice(FlagIncludeBin))
		moreIncludeBins, err := commands.ParsePathsFile(ctx.String(FlagIncludeBinFile))
		if err != nil {
			xc.Out.Error("param.error.include.bin.file", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		} else {
			for k, v := range moreIncludeBins {
				includeBins[k] = v
			}
		}

		includeExes := commands.ParsePaths(ctx.StringSlice(FlagIncludeExe))
		moreIncludeExes, err := commands.ParsePathsFile(ctx.String(FlagIncludeExeFile))
		if err != nil {
			xc.Out.Error("param.error.include.exe.file", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		} else {
			for k, v := range moreIncludeExes {
				includeExes[k] = v
			}
		}

		doIncludeShell := ctx.Bool(FlagIncludeShell)

		doIncludeCertAll := ctx.Bool(FlagIncludeCertAll)
		doIncludeCertBundles := ctx.Bool(FlagIncludeCertBundles)
		doIncludeCertDirs := ctx.Bool(FlagIncludeCertDirs)
		doIncludeCertPKAll := ctx.Bool(FlagIncludeCertPKAll)
		doIncludeCertPKDirs := ctx.Bool(FlagIncludeCertPKDirs)

		doUseLocalMounts := ctx.Bool(commands.FlagUseLocalMounts)
		doUseSensorVolume := ctx.String(commands.FlagUseSensorVolume)

		doKeepTmpArtifacts := ctx.Bool(FlagKeepTmpArtifacts)

		doExcludeMounts := ctx.BoolT(commands.FlagExcludeMounts)
		if doExcludeMounts {
			for mpath := range volumeMounts {
				excludePatterns[mpath] = nil
				mpattern := fmt.Sprintf("%s/**", mpath)
				excludePatterns[mpattern] = nil
			}
		}

		continueAfter, err := commands.GetContinueAfter(ctx)
		if err != nil {
			xc.Out.Error("param.error.continue.after", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		if continueAfter.Mode == config.CAMProbe && !doHTTPProbe {
			continueAfter.Mode = ""
			xc.Out.Info("exec",
				ovars{
					"message": "changing continue-after from probe to nothing because http-probe is disabled",
				})
		}

		execCmd := ctx.String(commands.FlagExec)
		execFile := ctx.String(commands.FlagExecFile)
		if strings.Contains(continueAfter.Mode, config.CAMExec) &&
			len(execCmd) == 0 &&
			len(execFile) == 0 {
			continueAfter.Mode = config.CAMEnter
			xc.Out.Info("exec",
				ovars{
					"message": "changing continue-after from exec to enter because there are no exec flags",
				})
		}

		if len(execCmd) != 0 && len(execFile) != 0 {
			xc.Out.Error("param.error.exec", "fatal: cannot use both --exec and --exec-file")
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}
		var execFileCmd []byte
		if len(execFile) > 0 {
			execFileCmd, err = ioutil.ReadFile(execFile)
			errutil.FailOn(err)

			if !strings.Contains(continueAfter.Mode, config.CAMExec) {
				if continueAfter.Mode == "" {
					continueAfter.Mode = config.CAMExec
				} else {
					continueAfter.Mode = fmt.Sprintf("%s&%s", continueAfter.Mode, config.CAMExec)
				}

				xc.Out.Info("exec",
					ovars{
						"message": fmt.Sprintf("updating continue-after mode to %s", continueAfter.Mode),
					})
			}

		} else if len(execCmd) > 0 {
			if !strings.Contains(continueAfter.Mode, config.CAMExec) {
				if continueAfter.Mode == "" {
					continueAfter.Mode = config.CAMExec
				} else {
					continueAfter.Mode = fmt.Sprintf("%s&%s", continueAfter.Mode, config.CAMExec)
				}

				xc.Out.Info("exec",
					ovars{
						"message": fmt.Sprintf("updating continue-after mode to %s", continueAfter.Mode),
					})
			}
		}

		if continueAfter.Mode == "" {
			continueAfter.Mode = config.CAMEnter
			xc.Out.Info("exec",
				ovars{
					"message": "changing continue-after to enter",
				})
		}

		commandReport := ctx.GlobalString(commands.FlagCommandReport)
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
			composeFile,
			targetComposeSvc,
			composeSvcNoPorts,
			depExcludeComposeSvcAll,
			depIncludeComposeSvcDeps,
			depIncludeComposeSvcs,
			depExcludeComposeSvcs,
			composeNets,
			cbOpts,
			crOpts,
			outputTags,
			doHTTPProbe,
			httpProbeCmds,
			httpProbeRetryCount,
			httpProbeRetryWait,
			httpProbePorts,
			httpCrawlMaxDepth,
			httpCrawlMaxPageCount,
			httpCrawlConcurrency,
			httpMaxConcurrentCrawlers,
			doHTTPProbeFull,
			doHTTPProbeExitOnFailure,
			httpProbeAPISpecs,
			httpProbeAPISpecFiles,
			httpProbeApps,
			portBindings,
			doPublishExposedPorts,
			doRmFileArtifacts,
			doCopyMetaArtifacts,
			doRunTargetAsUser,
			doShowContainerLogs,
			doShowBuildLogs,
			commands.ParseImageOverrides(doImageOverrides),
			overrides,
			instructions,
			ctx.StringSlice(commands.FlagLink),
			ctx.StringSlice(commands.FlagEtcHostsMap),
			ctx.StringSlice(commands.FlagContainerDNS),
			ctx.StringSlice(commands.FlagContainerDNSSearch),
			volumeMounts,
			doKeepPerms,
			pathPerms,
			excludePatterns,
			preservePaths,
			includePaths,
			includeBins,
			includeExes,
			doIncludeShell,
			doIncludeCertAll,
			doIncludeCertBundles,
			doIncludeCertDirs,
			doIncludeCertPKAll,
			doIncludeCertPKDirs,
			doUseLocalMounts,
			doUseSensorVolume,
			doKeepTmpArtifacts,
			continueAfter,
			execCmd,
			string(execFileCmd),
			deleteFatImage)

		return nil
	},
}
