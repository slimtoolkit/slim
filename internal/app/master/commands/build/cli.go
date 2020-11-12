package build

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"
	"github.com/docker-slim/docker-slim/internal/app/master/config"

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
		cli.StringFlag{
			Name:   FlagBuildFromDockerfile,
			Value:  "",
			Usage:  FlagBuildFromDockerfileUsage,
			EnvVar: "DSLIM_BUILD_DOCKERFILE",
		},
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
		commands.Cflag(commands.FlagPublishPort),
		commands.Cflag(commands.FlagPublishExposedPorts),
		commands.Cflag(commands.FlagKeepPerms),
		commands.Cflag(commands.FlagRunTargetAsUser),
		commands.Cflag(commands.FlagShowContainerLogs),
		cflag(FlagShowBuildLogs),
		commands.Cflag(commands.FlagCopyMetaArtifacts),
		commands.Cflag(commands.FlagRemoveFileArtifacts),
		cli.StringFlag{
			Name:   FlagTag,
			Value:  "",
			Usage:  FlagTagUsage,
			EnvVar: "DSLIM_TARGET_TAG",
		},
		cli.StringFlag{
			Name:   FlagTagFat,
			Value:  "",
			Usage:  FlagTagFatUsage,
			EnvVar: "DSLIM_TARGET_TAG_FAT",
		},
		cli.StringFlag{
			Name:   FlagImageOverrides,
			Value:  "",
			Usage:  FlagImageOverridesUsage,
			EnvVar: "DSLIM_TARGET_OVERRIDES",
		},
		commands.Cflag(commands.FlagExec),
		commands.Cflag(commands.FlagExecFile),
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
		commands.Cflag(commands.FlagPathPerms),
		commands.Cflag(commands.FlagPathPermsFile),
		commands.Cflag(commands.FlagIncludePath),
		commands.Cflag(commands.FlagIncludePathFile),
		commands.Cflag(commands.FlagIncludeBin),
		cli.StringFlag{
			Name:   FlagIncludeBinFile,
			Value:  "",
			Usage:  FlagIncludeBinFileUsage,
			EnvVar: "DSLIM_INCLUDE_BIN_FILE",
		},
		commands.Cflag(commands.FlagIncludeExe),
		cli.StringFlag{
			Name:   FlagIncludeExeFile,
			Value:  "",
			Usage:  FlagIncludeExeFileUsage,
			EnvVar: "DSLIM_INCLUDE_EXE_FILE",
		},
		commands.Cflag(commands.FlagIncludeShell),
		commands.Cflag(commands.FlagMount),
		commands.Cflag(commands.FlagContinueAfter),
		commands.Cflag(commands.FlagUseLocalMounts),
		commands.Cflag(commands.FlagUseSensorVolume),
		commands.Cflag(commands.FlagKeepTmpArtifacts),
	},
	Action: func(ctx *cli.Context) error {
		commands.ShowCommunityInfo()
		targetRef := ctx.String(commands.FlagTarget)

		if targetRef == "" {
			if len(ctx.Args()) < 1 {
				fmt.Printf("docker-slim[%s]: missing image ID/name...\n\n", Name)
				cli.ShowCommandHelp(ctx, Name)
				return nil
			} else {
				targetRef = ctx.Args().First()
			}
		}

		gcvalues, err := commands.GlobalCommandFlagValues(ctx)
		if err != nil {
			return err
		}

		doRmFileArtifacts := ctx.Bool(commands.FlagRemoveFileArtifacts)
		doCopyMetaArtifacts := ctx.String(commands.FlagCopyMetaArtifacts)

		buildFromDockerfile := ctx.String(FlagBuildFromDockerfile)

		portBindings, err := commands.ParsePortBindings(ctx.StringSlice(commands.FlagPublishPort))
		if err != nil {
			return err
		}

		doPublishExposedPorts := ctx.Bool(commands.FlagPublishExposedPorts)

		httpCrawlMaxDepth := ctx.Int(commands.FlagHTTPCrawlMaxDepth)
		httpCrawlMaxPageCount := ctx.Int(commands.FlagHTTPCrawlMaxPageCount)
		httpCrawlConcurrency := ctx.Int(commands.FlagHTTPCrawlConcurrency)
		httpMaxConcurrentCrawlers := ctx.Int(commands.FlagHTTPMaxConcurrentCrawlers)
		doHTTPProbeCrawl := ctx.Bool(commands.FlagHTTPProbeCrawl)

		doHTTPProbe := ctx.Bool(commands.FlagHTTPProbe)

		httpProbeCmds, err := commands.GetHTTPProbes(ctx)
		if err != nil {
			fmt.Printf("docker-slim[%s]: invalid HTTP probes: %v\n", Name, err)
			return err
		}

		if doHTTPProbe {
			//add default probe cmd if the "http-probe" flag is set
			fmt.Printf("docker-slim[%s]: info=http.probe message='using default probe'\n", Name)
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
			fmt.Printf("docker-slim[%s]: invalid HTTP Probe target ports: %v\n", Name, err)
			return err
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
				fmt.Printf("docker-slim[%s]: invalid spec file name='%s' error='%v': %v\n", Name, k, v)
			}

			return err
		}

		if len(httpProbeAPISpecFiles) > 0 {
			doHTTPProbe = true
		}

		doKeepPerms := ctx.Bool(commands.FlagKeepPerms)

		doRunTargetAsUser := ctx.Bool(commands.FlagRunTargetAsUser)

		doShowContainerLogs := ctx.Bool(commands.FlagShowContainerLogs)
		doShowBuildLogs := ctx.Bool(FlagShowBuildLogs)
		doTag := ctx.String(FlagTag)
		doTagFat := ctx.String(FlagTagFat)

		doImageOverrides := ctx.String(FlagImageOverrides)
		overrides, err := commands.GetContainerOverrides(ctx)
		if err != nil {
			fmt.Printf("docker-slim[%s]: invalid container overrides: %v\n", Name, err)
			return err
		}

		instructions, err := GetImageInstructions(ctx)
		if err != nil {
			fmt.Printf("docker-slim[%s]: invalid image instructions: %v\n", Name, err)
			return err
		}

		volumeMounts, err := commands.ParseVolumeMounts(ctx.StringSlice(commands.FlagMount))
		if err != nil {
			fmt.Printf("docker-slim[%s]: invalid volume mounts: %v\n", Name, err)
			return err
		}

		excludePatterns := commands.ParsePaths(ctx.StringSlice(commands.FlagExcludePattern))

		includePaths := commands.ParsePaths(ctx.StringSlice(commands.FlagIncludePath))
		moreIncludePaths, err := commands.ParsePathsFile(ctx.String(commands.FlagIncludePathFile))
		if err != nil {
			fmt.Printf("docker-slim[%s]: could not read include path file (ignoring): %v\n", Name, err)
		} else {
			for k, v := range moreIncludePaths {
				includePaths[k] = v
			}
		}

		pathPerms := commands.ParsePaths(ctx.StringSlice(commands.FlagPathPerms))
		morePathPerms, err := commands.ParsePathsFile(ctx.String(commands.FlagPathPermsFile))
		if err != nil {
			fmt.Printf("docker-slim[%s]: could not read path perms file (ignoring): %v\n", Name, err)
		} else {
			for k, v := range morePathPerms {
				pathPerms[k] = v
			}
		}

		includeBins := commands.ParsePaths(ctx.StringSlice(commands.FlagIncludeBin))
		moreIncludeBins, err := commands.ParsePathsFile(ctx.String(FlagIncludeBinFile))
		if err != nil {
			fmt.Printf("docker-slim[%s]: could not read include bin file (ignoring): %v\n", Name, err)
		} else {
			for k, v := range moreIncludeBins {
				includeBins[k] = v
			}
		}

		includeExes := commands.ParsePaths(ctx.StringSlice(commands.FlagIncludeExe))
		moreIncludeExes, err := commands.ParsePathsFile(ctx.String(FlagIncludeExeFile))
		if err != nil {
			fmt.Printf("docker-slim[%s]: could not read include exe file (ignoring): %v\n", Name, err)
		} else {
			for k, v := range moreIncludeExes {
				includeExes[k] = v
			}
		}

		doIncludeShell := ctx.Bool(commands.FlagIncludeShell)

		doUseLocalMounts := ctx.Bool(commands.FlagUseLocalMounts)
		doUseSensorVolume := ctx.String(commands.FlagUseSensorVolume)

		doKeepTmpArtifacts := ctx.Bool(commands.FlagKeepTmpArtifacts)

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
			fmt.Printf("docker-slim[%s]: invalid continue-after mode: %v\n", Name, err)
			return err
		}

		execCmd := ctx.String(commands.FlagExec)
		execFile := ctx.String(commands.FlagExecFile)
		if len(execCmd) != 0 && len(execFile) != 0 {
			fmt.Printf("docker-slim[%s]: info=exec message='fatal: cannot use both --exec and --exec-file'\n", Name)
			os.Exit(1)
		}
		var execFileCmd []byte
		if len(execFile) > 0 {
			execFileCmd, err = ioutil.ReadFile(execFile)
			if err != nil {
				panic(err)
			}
			continueAfter.Mode = "exec"
			fmt.Printf("docker-slim[%s]: info=exec message='changing continue-after to exec'\n", Name)
		} else if len(execCmd) > 0 {
			continueAfter.Mode = "exec"
			fmt.Printf("docker-slim[%s]: info=exec message='changing continue-after to exec'\n", Name)
		} else if !doHTTPProbe && continueAfter.Mode == "probe" {
			fmt.Printf("docker-slim[%s]: info=probe message='changing continue-after from probe to enter because http-probe is disabled'\n", Name)
			continueAfter.Mode = "enter"
		}

		commandReport := ctx.GlobalString(commands.FlagCommandReport)
		if commandReport == "off" {
			commandReport = ""
		}

		ec := &commands.ExecutionContext{}

		OnCommand(
			gcvalues,
			targetRef,
			buildFromDockerfile,
			doTag,
			doTagFat,
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
			includePaths,
			includeBins,
			includeExes,
			doIncludeShell,
			doUseLocalMounts,
			doUseSensorVolume,
			doKeepTmpArtifacts,
			continueAfter,
			ec,
			execCmd,
			string(execFileCmd))
		commands.ShowCommunityInfo()
		return nil
	},
}
