package build

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/artifact"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
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
		commands.Cflag(commands.FlagTarget),
		commands.Cflag(commands.FlagPull),
		commands.Cflag(commands.FlagDockerConfigPath),
		commands.Cflag(commands.FlagRegistryAccount),
		commands.Cflag(commands.FlagRegistrySecret),
		commands.Cflag(commands.FlagShowPullLogs),

		commands.Cflag(commands.FlagComposeFile),
		commands.Cflag(commands.FlagTargetComposeSvc),
		commands.Cflag(commands.FlagTargetComposeSvcImage),
		commands.Cflag(commands.FlagComposeSvcStartWait),
		commands.Cflag(commands.FlagComposeSvcNoPorts),
		commands.Cflag(commands.FlagDepExcludeComposeSvcAll),
		commands.Cflag(commands.FlagDepIncludeComposeSvc),
		commands.Cflag(commands.FlagDepExcludeComposeSvc),
		commands.Cflag(commands.FlagDepIncludeComposeSvcDeps),
		commands.Cflag(commands.FlagDepIncludeTargetComposeSvcDeps),
		commands.Cflag(commands.FlagComposeNet),
		commands.Cflag(commands.FlagComposeEnvNoHost),
		commands.Cflag(commands.FlagComposeEnvFile),
		commands.Cflag(commands.FlagComposeProjectName),
		commands.Cflag(commands.FlagComposeWorkdir),
		commands.Cflag(commands.FlagContainerProbeComposeSvc),
		commands.Cflag(commands.FlagHostExec),
		commands.Cflag(commands.FlagHostExecFile),

		commands.Cflag(commands.FlagTargetKubeWorkload),
		commands.Cflag(commands.FlagTargetKubeWorkloadNamespace),
		commands.Cflag(commands.FlagTargetKubeWorkloadContainer),
		commands.Cflag(commands.FlagTargetKubeWorkloadImage),
		commands.Cflag(commands.FlagKubeManifestFile),
		commands.Cflag(commands.FlagKubeKubeconfigFile),

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
		cflag(FlagImageOverrides),
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
		commands.Cflag(commands.FlagMount),
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
		commands.Cflag(commands.FlagExcludeMounts),
		commands.Cflag(commands.FlagExcludePattern),
		cflag(FlagPreservePath),
		cflag(FlagPreservePathFile),
		cflag(FlagIncludePath),
		cflag(FlagIncludePathFile),
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
		cflag(FlagObfuscateMetadata),
		commands.Cflag(commands.FlagContinueAfter),
		commands.Cflag(commands.FlagUseLocalMounts),
		commands.Cflag(commands.FlagUseSensorVolume),
		commands.Cflag(commands.FlagRTAOnbuildBaseImage),
		commands.Cflag(commands.FlagRTASourcePT),
		//Sensor flags:
		commands.Cflag(commands.FlagSensorIPCEndpoint),
		commands.Cflag(commands.FlagSensorIPCMode),
	}, commands.HTTPProbeFlags()...),
	Action: func(ctx *cli.Context) error {
		xc := app.NewExecutionContext(Name, ctx.String(commands.FlagConsoleFormat))

		cbOpts, err := GetContainerBuildOptions(ctx)
		if err != nil {
			xc.Out.Error("param.error.container.build.options", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		deleteFatImage := ctx.Bool(commands.FlagDeleteFatImage)
		if cbOpts.Dockerfile == "" {
			deleteFatImage = false
		}

		composeFiles := ctx.StringSlice(commands.FlagComposeFile)

		//todo: load/parse compose file and then use it to validate the related compose params
		targetComposeSvc := ctx.String(commands.FlagTargetComposeSvc)
		targetComposeSvcImage := ctx.String(commands.FlagTargetComposeSvcImage)
		composeSvcNoPorts := ctx.Bool(commands.FlagComposeSvcNoPorts)
		depExcludeComposeSvcAll := ctx.Bool(commands.FlagDepExcludeComposeSvcAll)
		depIncludeComposeSvcDeps := ctx.String(commands.FlagDepIncludeComposeSvcDeps)
		depIncludeTargetComposeSvcDeps := ctx.Bool(commands.FlagDepIncludeTargetComposeSvcDeps)
		depIncludeComposeSvcs := ctx.StringSlice(commands.FlagDepIncludeComposeSvc)
		depExcludeComposeSvcs := ctx.StringSlice(commands.FlagDepExcludeComposeSvc)
		composeNets := ctx.StringSlice(commands.FlagComposeNet)

		composeSvcStartWait := ctx.Int(commands.FlagComposeSvcStartWait)

		composeEnvNoHost := ctx.Bool(commands.FlagComposeEnvNoHost)
		composeEnvVars, err := commands.ParseLinesWithCommentsFile(ctx.String(commands.FlagComposeEnvFile))
		if err != nil {
			xc.Out.Error("param.error.compose.env.file", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		composeProjectName := ctx.String(commands.FlagComposeProjectName)
		composeWorkdir := ctx.String(commands.FlagComposeWorkdir)
		containerProbeComposeSvc := ctx.String(commands.FlagContainerProbeComposeSvc)

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
			targetRef = ctx.String(commands.FlagTarget)

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

		gparams, ok := commands.CLIContextGet(ctx.Context, commands.GlobalParams).(*commands.GenericParams)
		if !ok || gparams == nil {
			xc.Out.Error("param.global", "missing params")
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		appOpts, ok := commands.CLIContextGet(ctx.Context, commands.AppParams).(*config.AppOptions)
		if !kubeOpts.HasTargetSet() && (!ok || appOpts == nil) {
			log.Debug("param.error.app.options - no app params")
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

		if continueAfter.Mode == config.CAMProbe && !httpProbeOpts.Do {
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
			execFileCmd, err = os.ReadFile(execFile)
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

		doKeepPerms := ctx.Bool(FlagKeepPerms)

		doRunTargetAsUser := ctx.Bool(commands.FlagRunTargetAsUser)

		doShowContainerLogs := ctx.Bool(commands.FlagShowContainerLogs)
		doShowBuildLogs := ctx.Bool(FlagShowBuildLogs)
		outputTags := ctx.StringSlice(FlagTag)

		doImageOverrides := ctx.String(FlagImageOverrides)
		overrides, err := commands.GetContainerOverrides(xc, ctx)
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

		creportIncludePaths, err := commands.ParsePathsCreportFile(ctx.String(FlagIncludePathsCreportFile))
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

		doIncludeWorkdir := ctx.Bool(FlagIncludeWorkdir)
		includeLastImageLayers := uint(0)
		doIncludeAppImageAll := ctx.Bool(FlagIncludeAppImageAll)
		appImageStartInstGroup := ctx.Int(FlagAppImageStartInstGroup)
		appImageStartInst := ctx.String(FlagAppImageStartInst)

		appImageDockerfilePath := ctx.String(FlagAppImageDockerfile)
		appImageDockerfileInsts, err := commands.ParseLinesWithCommentsFile(appImageDockerfilePath)
		if err != nil {
			xc.Out.Error("param.error.app.image.dockerfile", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		doIncludeOSLibsNet := ctx.Bool(FlagIncludeOSLibsNet)

		doIncludeCertAll := ctx.Bool(FlagIncludeCertAll)
		doIncludeCertBundles := ctx.Bool(FlagIncludeCertBundles)
		doIncludeCertDirs := ctx.Bool(FlagIncludeCertDirs)
		doIncludeCertPKAll := ctx.Bool(FlagIncludeCertPKAll)
		doIncludeCertPKDirs := ctx.Bool(FlagIncludeCertPKDirs)

		doIncludeNew := ctx.Bool(FlagIncludeNew)

		doUseLocalMounts := ctx.Bool(commands.FlagUseLocalMounts)
		doUseSensorVolume := ctx.String(commands.FlagUseSensorVolume)

		doKeepTmpArtifacts := ctx.Bool(FlagKeepTmpArtifacts)

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

		rtaOnbuildBaseImage := ctx.Bool(commands.FlagRTAOnbuildBaseImage)
		rtaSourcePT := ctx.Bool(commands.FlagRTASourcePT)

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
			doIncludeWorkdir,
			includeLastImageLayers,
			doIncludeAppImageAll,
			appImageStartInstGroup,
			appImageStartInst,
			appImageDockerfileInsts,
			doIncludeOSLibsNet,
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
			ctx.String(commands.FlagSensorIPCEndpoint),
			ctx.String(commands.FlagSensorIPCMode),
			kubeOpts,
			GetAppNodejsInspectOptions(ctx),
			imageBuildEngine,
			imageBuildArch)

		return nil
	},
}
