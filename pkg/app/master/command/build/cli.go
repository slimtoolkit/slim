package build

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/app/master/config"
	"github.com/slimtoolkit/slim/pkg/artifact"
)

const (
	Name  = "build"
	Usage = "Analyzes, profiles and optimizes your container image auto-generating Seccomp and AppArmor security profiles"
	Alias = "b"
)

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: append([]cli.Flag{
		command.Cflag(command.FlagCommandParamsFile),
		command.Cflag(command.FlagTarget),
		command.Cflag(command.FlagPull),
		command.Cflag(command.FlagDockerConfigPath),
		command.Cflag(command.FlagRegistryAccount),
		command.Cflag(command.FlagRegistrySecret),
		command.Cflag(command.FlagShowPullLogs),

		command.Cflag(command.FlagComposeFile),
		command.Cflag(command.FlagTargetComposeSvc),
		command.Cflag(command.FlagTargetComposeSvcImage),
		command.Cflag(command.FlagComposeSvcStartWait),
		command.Cflag(command.FlagComposeSvcNoPorts),
		command.Cflag(command.FlagDepExcludeComposeSvcAll),
		command.Cflag(command.FlagDepIncludeComposeSvc),
		command.Cflag(command.FlagDepExcludeComposeSvc),
		command.Cflag(command.FlagDepIncludeComposeSvcDeps),
		command.Cflag(command.FlagDepIncludeTargetComposeSvcDeps),
		command.Cflag(command.FlagComposeNet),
		command.Cflag(command.FlagComposeEnvNoHost),
		command.Cflag(command.FlagComposeEnvFile),
		command.Cflag(command.FlagComposeProjectName),
		command.Cflag(command.FlagComposeWorkdir),
		command.Cflag(command.FlagContainerProbeComposeSvc),
		command.Cflag(command.FlagHostExec),
		command.Cflag(command.FlagHostExecFile),

		command.Cflag(command.FlagTargetKubeWorkload),
		command.Cflag(command.FlagTargetKubeWorkloadNamespace),
		command.Cflag(command.FlagTargetKubeWorkloadContainer),
		command.Cflag(command.FlagTargetKubeWorkloadImage),
		command.Cflag(command.FlagKubeManifestFile),
		command.Cflag(command.FlagKubeKubeconfigFile),

		command.Cflag(command.FlagPublishPort),
		command.Cflag(command.FlagPublishExposedPorts),
		command.Cflag(command.FlagRunTargetAsUser),
		command.Cflag(command.FlagShowContainerLogs),
		command.Cflag(command.FlagEnableMondelLogs),
		cflag(FlagShowBuildLogs),
		command.Cflag(command.FlagCopyMetaArtifacts),
		command.Cflag(command.FlagRemoveFileArtifacts),
		command.Cflag(command.FlagExec),
		command.Cflag(command.FlagExecFile),
		//
		cflag(FlagTag),
		cflag(FlagImageOverrides),
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
		command.Cflag(command.FlagMount),
		//Container Build Options
		cflag(FlagImageBuildEngine),
		cflag(FlagImageBuildArch),
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
		cflag(FlagRemoveExpose),
		cflag(FlagRemoveEnv),
		cflag(FlagRemoveLabel),
		cflag(FlagRemoveVolume),
		cflag(FlagPreservePath),
		cflag(FlagPreservePathFile),
		cflag(FlagIncludePath),
		cflag(FlagIncludePathFile),
		cflag(FlagIncludeDirBins),
		cflag(FlagIncludeBin),
		cflag(FlagIncludeBinFile),
		cflag(FlagIncludeExeFile),
		cflag(FlagIncludeExe),
		cflag(FlagIncludeShell),
		cflag(FlagIncludeWorkdir),
		cflag(FlagIncludeAppImageAll),
		cflag(FlagAppImageStartInstGroup),
		cflag(FlagAppImageStartInst),
		cflag(FlagAppImageDockerfile),
		cflag(FlagIncludePathsCreportFile),
		cflag(FlagIncludeOSLibsNet),
		cflag(FlagIncludeSSHClient),
		cflag(FlagIncludeZoneInfo),
		cflag(FlagIncludeCertAll),
		cflag(FlagIncludeCertBundles),
		cflag(FlagIncludeCertDirs),
		cflag(FlagIncludeCertPKAll),
		cflag(FlagIncludeCertPKDirs),
		cflag(FlagIncludeNew),
		cflag(FlagKeepTmpArtifacts),
		cflag(FlagIncludeAppNuxtDir),
		cflag(FlagIncludeAppNuxtBuildDir),
		cflag(FlagIncludeAppNuxtDistDir),
		cflag(FlagIncludeAppNuxtStaticDir),
		cflag(FlagIncludeAppNuxtNodeModulesDir),
		cflag(FlagIncludeAppNextDir),
		cflag(FlagIncludeAppNextBuildDir),
		cflag(FlagIncludeAppNextDistDir),
		cflag(FlagIncludeAppNextStaticDir),
		cflag(FlagIncludeAppNextNodeModulesDir),
		cflag(FlagIncludeNodePackage),
		cflag(FlagKeepPerms),
		cflag(FlagPathPerms),
		cflag(FlagPathPermsFile),
		//"EXCLUDE" FLAGS - START
		cflag(FlagExcludePattern),
		cflag(FlagExcludeVarLockFiles),
		cflag(FlagExcludeMounts),
		//"EXCLUDE" FLAGS - END
		cflag(FlagObfuscateMetadata),
		command.Cflag(command.FlagContinueAfter),
		command.Cflag(command.FlagUseLocalMounts),
		command.Cflag(command.FlagUseSensorVolume),
		command.Cflag(command.FlagRTAOnbuildBaseImage),
		command.Cflag(command.FlagRTASourcePT),
		//Sensor flags:
		command.Cflag(command.FlagSensorIPCEndpoint),
		command.Cflag(command.FlagSensorIPCMode),
	}, command.HTTPProbeFlags()...),
	Action: func(ctx *cli.Context) error {
		gparams, ok := command.CLIContextGet(ctx.Context, command.GlobalParams).(*command.GenericParams)
		if !ok || gparams == nil {
			return command.ErrNoGlobalParams
		}

		xc := app.NewExecutionContext(
			Name,
			gparams.QuietCLIMode,
			gparams.OutputFormat)

		//NOTE: this is a placeholder to load all command params from a file
		_ = ctx.String(command.FlagCommandParamsFile)

		cbOpts, err := GetContainerBuildOptions(ctx)
		if err != nil {
			xc.Out.Error("param.error.container.build.options", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		deleteFatImage := ctx.Bool(command.FlagDeleteFatImage)
		if cbOpts.Dockerfile == "" {
			deleteFatImage = false
		}

		composeFiles := ctx.StringSlice(command.FlagComposeFile)

		//todo: load/parse compose file and then use it to validate the related compose params
		targetComposeSvc := ctx.String(command.FlagTargetComposeSvc)
		targetComposeSvcImage := ctx.String(command.FlagTargetComposeSvcImage)
		composeSvcNoPorts := ctx.Bool(command.FlagComposeSvcNoPorts)
		depExcludeComposeSvcAll := ctx.Bool(command.FlagDepExcludeComposeSvcAll)
		depIncludeComposeSvcDeps := ctx.String(command.FlagDepIncludeComposeSvcDeps)
		depIncludeTargetComposeSvcDeps := ctx.Bool(command.FlagDepIncludeTargetComposeSvcDeps)
		depIncludeComposeSvcs := ctx.StringSlice(command.FlagDepIncludeComposeSvc)
		depExcludeComposeSvcs := ctx.StringSlice(command.FlagDepExcludeComposeSvc)
		composeNets := ctx.StringSlice(command.FlagComposeNet)

		composeSvcStartWait := ctx.Int(command.FlagComposeSvcStartWait)

		composeEnvNoHost := ctx.Bool(command.FlagComposeEnvNoHost)
		composeEnvVars, err := command.ParseLinesWithCommentsFile(ctx.String(command.FlagComposeEnvFile))
		if err != nil {
			xc.Out.Error("param.error.compose.env.file", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		composeProjectName := ctx.String(command.FlagComposeProjectName)
		composeWorkdir := ctx.String(command.FlagComposeWorkdir)
		containerProbeComposeSvc := ctx.String(command.FlagContainerProbeComposeSvc)

		kubeOpts, err := GetKubernetesOptions(ctx)
		if err != nil {
			xc.Out.Error("param.error.kubernetes.options", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		var targetRef string

		if kubeOpts.HasTargetSet() {
			targetRef = kubeOpts.Target.Workload
		} else if len(composeFiles) > 0 && targetComposeSvc != "" {
			targetRef = targetComposeSvc
		} else if cbOpts.Dockerfile == "" {
			targetRef = ctx.String(command.FlagTarget)

			if targetRef == "" {
				if ctx.Args().Len() < 1 {
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
				if ctx.Args().Len() < 1 {
					xc.Out.Error("param.target", "missing Dockerfile build context directory")
					cli.ShowCommandHelp(ctx, Name)
					return nil
				} else {
					targetRef = ctx.Args().First()
				}
			}
		}

		if targetRef == "" {
			xc.Out.Error("param.target", "missing target - make sure to set one of the target params")
			cli.ShowCommandHelp(ctx, Name)
			return nil
		}

		appOpts, ok := command.CLIContextGet(ctx.Context, command.AppParams).(*config.AppOptions)
		if !kubeOpts.HasTargetSet() && (!ok || appOpts == nil) {
			log.Debug("param.error.app.options - no app params")
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

		if continueAfter.Mode == config.CAMProbe && !httpProbeOpts.Do {
			continueAfter.Mode = ""
			xc.Out.Info("exec",
				ovars{
					"message": "changing continue-after from probe to nothing because http-probe is disabled",
				})
		}

		execCmd := ctx.String(command.FlagExec)
		execFile := ctx.String(command.FlagExecFile)
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
			execFileCmd, err = os.ReadFile(execFile)
			xc.FailOn(err)

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

		if containerProbeComposeSvc != "" {
			if !strings.Contains(continueAfter.Mode, config.CAMContainerProbe) {
				if continueAfter.Mode == "" {
					continueAfter.Mode = config.CAMContainerProbe
				} else {
					continueAfter.Mode = fmt.Sprintf("%s&%s", continueAfter.Mode, config.CAMContainerProbe)
				}

				xc.Out.Info("continue.after",
					ovars{
						"message": fmt.Sprintf("updating mode to %s", continueAfter.Mode),
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

		doKeepPerms := ctx.Bool(FlagKeepPerms)

		doRunTargetAsUser := ctx.Bool(command.FlagRunTargetAsUser)

		doShowContainerLogs := ctx.Bool(command.FlagShowContainerLogs)
		doEnableMondel := ctx.Bool(command.FlagEnableMondelLogs)
		doShowBuildLogs := ctx.Bool(FlagShowBuildLogs)
		outputTags := ctx.StringSlice(FlagTag)

		doImageOverrides := ctx.String(FlagImageOverrides)
		overrides, err := command.GetContainerOverrides(xc, ctx)
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

		volumeMounts, err := command.ParseVolumeMounts(ctx.StringSlice(command.FlagMount))
		if err != nil {
			xc.Out.Error("param.error.mount", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		excludePatterns := command.ParsePaths(ctx.StringSlice(FlagExcludePattern))

		preservePaths := command.ParsePaths(ctx.StringSlice(FlagPreservePath))
		morePreservePaths, err := command.ParsePathsFile(ctx.String(FlagPreservePathFile))
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

		if len(preservePaths) > 0 {
			for filtered := range artifact.FilteredPaths {
				if _, found := preservePaths[filtered]; found {
					delete(preservePaths, filtered)
					xc.Out.Info("params",
						ovars{
							"preserve.path": filtered,
							"message":       "ignoring",
						})
				}
			}

			var toDelete []string
			for ip := range preservePaths {
				if artifact.IsFilteredPath(ip) {
					toDelete = append(toDelete, ip)
				}
			}

			for _, dp := range toDelete {
				delete(preservePaths, dp)
				xc.Out.Info("params",
					ovars{
						"preserve.path": dp,
						"message":       "ignoring",
					})
			}
		}

		includePaths := command.ParsePaths(ctx.StringSlice(FlagIncludePath))
		moreIncludePaths, err := command.ParsePathsFile(ctx.String(FlagIncludePathFile))
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

		if len(includePaths) > 0 {
			for filtered := range artifact.FilteredPaths {
				if _, found := includePaths[filtered]; found {
					delete(includePaths, filtered)
					xc.Out.Info("params",
						ovars{
							"include.path": filtered,
							"message":      "ignoring",
						})
				}
			}

			var toDelete []string
			for ip := range includePaths {
				if artifact.IsFilteredPath(ip) {
					toDelete = append(toDelete, ip)
				}
			}

			for _, dp := range toDelete {
				delete(includePaths, dp)
				xc.Out.Info("params",
					ovars{
						"include.path": dp,
						"message":      "ignoring",
					})
			}
		}

		creportIncludePaths, err := command.ParsePathsCreportFile(ctx.String(FlagIncludePathsCreportFile))
		if err != nil {
			xc.Out.Error("param.error.include.paths.creport.file", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		} else {
			for k, v := range creportIncludePaths {
				includePaths[k] = v
			}
		}

		pathPerms := command.ParsePaths(ctx.StringSlice(FlagPathPerms))
		morePathPerms, err := command.ParsePathsFile(ctx.String(FlagPathPermsFile))
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

		includeBins := command.ParsePaths(ctx.StringSlice(FlagIncludeBin))
		moreIncludeBins, err := command.ParsePathsFile(ctx.String(FlagIncludeBinFile))
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

		if len(includeBins) > 0 {
			//shouldn't happen, but filtering either way
			for filtered := range artifact.FilteredPaths {
				if _, found := includeBins[filtered]; found {
					delete(includeBins, filtered)
					xc.Out.Info("params",
						ovars{
							"include.bin": filtered,
							"message":     "ignoring",
						})
				}
			}

			var toDelete []string
			for ip := range includeBins {
				if artifact.IsFilteredPath(ip) {
					toDelete = append(toDelete, ip)
				}
			}

			for _, dp := range toDelete {
				delete(includeBins, dp)
				xc.Out.Info("params",
					ovars{
						"include.bin": dp,
						"message":     "ignoring",
					})
			}
		}

		//note: if path perms, ID change are provided they are applied to all matching binaries
		includeDirBinsList := command.ParsePaths(ctx.StringSlice(FlagIncludeDirBins))

		includeExes := command.ParsePaths(ctx.StringSlice(FlagIncludeExe))
		moreIncludeExes, err := command.ParsePathsFile(ctx.String(FlagIncludeExeFile))
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

		doIncludeWorkdir := ctx.Bool(FlagIncludeWorkdir)
		includeLastImageLayers := uint(0)
		doIncludeAppImageAll := ctx.Bool(FlagIncludeAppImageAll)
		appImageStartInstGroup := ctx.Int(FlagAppImageStartInstGroup)
		appImageStartInst := ctx.String(FlagAppImageStartInst)

		appImageDockerfilePath := ctx.String(FlagAppImageDockerfile)
		appImageDockerfileInsts, err := command.ParseLinesWithCommentsFile(appImageDockerfilePath)
		if err != nil {
			xc.Out.Error("param.error.app.image.dockerfile", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		doIncludeSSHClient := ctx.Bool(FlagIncludeSSHClient)
		doIncludeOSLibsNet := ctx.Bool(FlagIncludeOSLibsNet)

		doIncludeZoneInfo := ctx.Bool(FlagIncludeZoneInfo)

		doIncludeCertAll := ctx.Bool(FlagIncludeCertAll)
		doIncludeCertBundles := ctx.Bool(FlagIncludeCertBundles)
		doIncludeCertDirs := ctx.Bool(FlagIncludeCertDirs)
		doIncludeCertPKAll := ctx.Bool(FlagIncludeCertPKAll)
		doIncludeCertPKDirs := ctx.Bool(FlagIncludeCertPKDirs)

		doIncludeNew := ctx.Bool(FlagIncludeNew)

		doUseLocalMounts := ctx.Bool(command.FlagUseLocalMounts)
		doUseSensorVolume := ctx.String(command.FlagUseSensorVolume)

		doKeepTmpArtifacts := ctx.Bool(FlagKeepTmpArtifacts)

		doExcludeVarLockFiles := ctx.Bool(FlagExcludeVarLockFiles)
		doExcludeMounts := ctx.Bool(FlagExcludeMounts)
		if doExcludeMounts {
			for mpath := range volumeMounts {
				excludePatterns[mpath] = nil
				mpattern := fmt.Sprintf("%s/**", mpath)
				excludePatterns[mpattern] = nil
			}
		}

		commandReport := ctx.String(command.FlagCommandReport)
		if commandReport == "off" {
			commandReport = ""
		}

		rtaOnbuildBaseImage := ctx.Bool(command.FlagRTAOnbuildBaseImage)
		rtaSourcePT := ctx.Bool(command.FlagRTASourcePT)

		doObfuscateMetadata := ctx.Bool(FlagObfuscateMetadata)

		imageBuildEngine, err := getImageBuildEngine(ctx)
		if err != nil {
			xc.Out.Error("param.error.image-build-engine", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		imageBuildArch, err := getImageBuildArch(ctx)
		if err != nil {
			xc.Out.Error("param.error.image-build-arch", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		OnCommand(
			xc,
			gparams,
			targetRef,
			doPull,
			dockerConfigPath,
			registryAccount,
			registrySecret,
			doShowPullLogs,
			composeFiles,
			targetComposeSvc,
			targetComposeSvcImage,
			composeSvcStartWait,
			composeSvcNoPorts,
			depExcludeComposeSvcAll,
			depIncludeComposeSvcDeps,
			depIncludeTargetComposeSvcDeps,
			depIncludeComposeSvcs,
			depExcludeComposeSvcs,
			composeNets,
			composeEnvVars,
			composeEnvNoHost,
			composeWorkdir,
			composeProjectName,
			containerProbeComposeSvc,
			cbOpts,
			crOpts,
			outputTags,
			httpProbeOpts,
			portBindings,
			doPublishExposedPorts,
			hostExecProbes,
			doRmFileArtifacts,
			doCopyMetaArtifacts,
			doRunTargetAsUser,
			doShowContainerLogs,
			doEnableMondel,
			doShowBuildLogs,
			command.ParseImageOverrides(doImageOverrides),
			overrides,
			instructions,
			ctx.StringSlice(command.FlagLink),
			ctx.StringSlice(command.FlagEtcHostsMap),
			ctx.StringSlice(command.FlagContainerDNS),
			ctx.StringSlice(command.FlagContainerDNSSearch),
			volumeMounts,
			doKeepPerms,
			pathPerms,
			excludePatterns,
			doExcludeVarLockFiles,
			preservePaths,
			includePaths,
			includeBins,
			includeDirBinsList,
			includeExes,
			doIncludeShell,
			doIncludeWorkdir,
			includeLastImageLayers,
			doIncludeAppImageAll,
			appImageStartInstGroup,
			appImageStartInst,
			appImageDockerfileInsts,
			doIncludeSSHClient,
			doIncludeOSLibsNet,
			doIncludeZoneInfo,
			doIncludeCertAll,
			doIncludeCertBundles,
			doIncludeCertDirs,
			doIncludeCertPKAll,
			doIncludeCertPKDirs,
			doIncludeNew,
			doUseLocalMounts,
			doUseSensorVolume,
			doKeepTmpArtifacts,
			continueAfter,
			execCmd,
			string(execFileCmd),
			deleteFatImage,
			rtaOnbuildBaseImage,
			rtaSourcePT,
			doObfuscateMetadata,
			ctx.String(command.FlagSensorIPCEndpoint),
			ctx.String(command.FlagSensorIPCMode),
			kubeOpts,
			GetAppNodejsInspectOptions(ctx),
			imageBuildEngine,
			imageBuildArch)

		return nil
	},
}
