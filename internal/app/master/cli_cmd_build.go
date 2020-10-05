package app

import (
	"fmt"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"
	"github.com/docker-slim/docker-slim/internal/app/master/config"

	"github.com/urfave/cli"
)

var cmdBuild = cli.Command{
	Name:    cmdSpecs[CmdBuild].name,
	Aliases: []string{cmdSpecs[CmdBuild].alias},
	Usage:   cmdSpecs[CmdBuild].usage,
	Flags: []cli.Flag{
		cflag(FlagTarget),
		cli.StringFlag{
			Name:   FlagBuildFromDockerfile,
			Value:  "",
			Usage:  FlagBuildFromDockerfileUsage,
			EnvVar: "DSLIM_BUILD_DOCKERFILE",
		},
		cflag(FlagHTTPProbe),
		cflag(FlagHTTPProbeCmd),
		cflag(FlagHTTPProbeCmdFile),
		cflag(FlagHTTPProbeRetryCount),
		cflag(FlagHTTPProbeRetryWait),
		cflag(FlagHTTPProbePorts),
		cflag(FlagHTTPProbeFull),
		cflag(FlagHTTPProbeExitOnFailure),
		cflag(FlagHTTPProbeCrawl),
		cflag(FlagHTTPCrawlMaxDepth),
		cflag(FlagHTTPCrawlMaxPageCount),
		cflag(FlagHTTPCrawlConcurrency),
		cflag(FlagHTTPMaxConcurrentCrawlers),
		cflag(FlagHTTPProbeAPISpec),
		cflag(FlagHTTPProbeAPISpecFile),
		cflag(FlagPublishPort),
		cflag(FlagPublishExposedPorts),
		cflag(FlagKeepPerms),
		cflag(FlagRunTargetAsUser),
		cflag(FlagShowContainerLogs),
		cflag(FlagShowBuildLogs),
		cflag(FlagCopyMetaArtifacts),
		cflag(FlagRemoveFileArtifacts),
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
		cflag(FlagEntrypoint),
		cflag(FlagCmd),
		cflag(FlagWorkdir),
		cflag(FlagEnv),
		cflag(FlagLabel),
		cflag(FlagVolume),
		cflag(FlagLink),
		cflag(FlagEtcHostsMap),
		cflag(FlagContainerDNS),
		cflag(FlagContainerDNSSearch),
		cflag(FlagNetwork),
		cflag(FlagHostname),
		cflag(FlagExpose),
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
		cflag(FlagExcludeMounts),
		cflag(FlagExcludePattern),
		cflag(FlagPathPerms),
		cflag(FlagPathPermsFile),
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
		cflag(FlagMount),
		cflag(FlagContinueAfter),
		cflag(FlagUseLocalMounts),
		cflag(FlagUseSensorVolume),
		cflag(FlagKeepTmpArtifacts),
	},
	Action: func(ctx *cli.Context) error {
		commands.ShowCommunityInfo()
		targetRef := ctx.String(FlagTarget)

		if targetRef == "" {
			if len(ctx.Args()) < 1 {
				fmt.Printf("docker-slim[build]: missing image ID/name...\n\n")
				cli.ShowCommandHelp(ctx, CmdBuild)
				return nil
			} else {
				targetRef = ctx.Args().First()
			}
		}

		gcvalues, err := globalCommandFlagValues(ctx)
		if err != nil {
			return err
		}

		doRmFileArtifacts := ctx.Bool(FlagRemoveFileArtifacts)
		doCopyMetaArtifacts := ctx.String(FlagCopyMetaArtifacts)

		buildFromDockerfile := ctx.String(FlagBuildFromDockerfile)

		portBindings, err := parsePortBindings(ctx.StringSlice(FlagPublishPort))
		if err != nil {
			return err
		}

		doPublishExposedPorts := ctx.Bool(FlagPublishExposedPorts)

		httpCrawlMaxDepth := ctx.Int(FlagHTTPCrawlMaxDepth)
		httpCrawlMaxPageCount := ctx.Int(FlagHTTPCrawlMaxPageCount)
		httpCrawlConcurrency := ctx.Int(FlagHTTPCrawlConcurrency)
		httpMaxConcurrentCrawlers := ctx.Int(FlagHTTPMaxConcurrentCrawlers)
		doHTTPProbeCrawl := ctx.Bool(FlagHTTPProbeCrawl)

		doHTTPProbe := ctx.Bool(FlagHTTPProbe)

		httpProbeCmds, err := getHTTPProbes(ctx)
		if err != nil {
			fmt.Printf("docker-slim[build]: invalid HTTP probes: %v\n", err)
			return err
		}

		if doHTTPProbe {
			//add default probe cmd if the "http-probe" flag is set
			fmt.Println("docker-slim[build]: info=http.probe message='using default probe'")
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

		httpProbeRetryCount := ctx.Int(FlagHTTPProbeRetryCount)
		httpProbeRetryWait := ctx.Int(FlagHTTPProbeRetryWait)
		httpProbePorts, err := parseHTTPProbesPorts(ctx.String(FlagHTTPProbePorts))
		if err != nil {
			fmt.Printf("docker-slim[build]: invalid HTTP Probe target ports: %v\n", err)
			return err
		}

		doHTTPProbeFull := ctx.Bool(FlagHTTPProbeFull)
		doHTTPProbeExitOnFailure := ctx.Bool(FlagHTTPProbeExitOnFailure)

		httpProbeAPISpecs := ctx.StringSlice(FlagHTTPProbeAPISpec)
		if len(httpProbeAPISpecs) > 0 {
			doHTTPProbe = true
		}

		httpProbeAPISpecFiles, fileErrors := validateFiles(ctx.StringSlice(FlagHTTPProbeAPISpecFile))
		if len(fileErrors) > 0 {
			var err error
			for k, v := range fileErrors {
				err = v
				fmt.Printf("docker-slim[build]: invalid spec file name='%s' error='%v': %v\n", k, v)
			}

			return err
		}

		if len(httpProbeAPISpecFiles) > 0 {
			doHTTPProbe = true
		}

		doKeepPerms := ctx.Bool(FlagKeepPerms)

		doRunTargetAsUser := ctx.Bool(FlagRunTargetAsUser)

		doShowContainerLogs := ctx.Bool(FlagShowContainerLogs)
		doShowBuildLogs := ctx.Bool(FlagShowBuildLogs)
		doTag := ctx.String(FlagTag)
		doTagFat := ctx.String(FlagTagFat)

		doImageOverrides := ctx.String(FlagImageOverrides)
		overrides, err := getContainerOverrides(ctx)
		if err != nil {
			fmt.Printf("docker-slim[build]: invalid container overrides: %v\n", err)
			return err
		}

		instructions, err := getImageInstructions(ctx)
		if err != nil {
			fmt.Printf("docker-slim[build]: invalid image instructions: %v\n", err)
			return err
		}

		volumeMounts, err := parseVolumeMounts(ctx.StringSlice(FlagMount))
		if err != nil {
			fmt.Printf("docker-slim[build]: invalid volume mounts: %v\n", err)
			return err
		}

		excludePatterns := parsePaths(ctx.StringSlice(FlagExcludePattern))

		includePaths := parsePaths(ctx.StringSlice(FlagIncludePath))
		moreIncludePaths, err := parsePathsFile(ctx.String(FlagIncludePathFile))
		if err != nil {
			fmt.Printf("docker-slim[build]: could not read include path file (ignoring): %v\n", err)
		} else {
			for k, v := range moreIncludePaths {
				includePaths[k] = v
			}
		}

		pathPerms := parsePaths(ctx.StringSlice(FlagPathPerms))
		morePathPerms, err := parsePathsFile(ctx.String(FlagPathPermsFile))
		if err != nil {
			fmt.Printf("docker-slim[build]: could not read path perms file (ignoring): %v\n", err)
		} else {
			for k, v := range morePathPerms {
				pathPerms[k] = v
			}
		}

		includeBins := parsePaths(ctx.StringSlice(FlagIncludeBin))
		moreIncludeBins, err := parsePathsFile(ctx.String(FlagIncludeBinFile))
		if err != nil {
			fmt.Printf("docker-slim[build]: could not read include bin file (ignoring): %v\n", err)
		} else {
			for k, v := range moreIncludeBins {
				includeBins[k] = v
			}
		}

		includeExes := parsePaths(ctx.StringSlice(FlagIncludeExe))
		moreIncludeExes, err := parsePathsFile(ctx.String(FlagIncludeExeFile))
		if err != nil {
			fmt.Printf("docker-slim[build]: could not read include exe file (ignoring): %v\n", err)
		} else {
			for k, v := range moreIncludeExes {
				includeExes[k] = v
			}
		}

		doIncludeShell := ctx.Bool(FlagIncludeShell)

		doUseLocalMounts := ctx.Bool(FlagUseLocalMounts)
		doUseSensorVolume := ctx.String(FlagUseSensorVolume)

		doKeepTmpArtifacts := ctx.Bool(FlagKeepTmpArtifacts)

		doExcludeMounts := ctx.BoolT(FlagExcludeMounts)
		if doExcludeMounts {
			for mpath := range volumeMounts {
				excludePatterns[mpath] = nil
				mpattern := fmt.Sprintf("%s/**", mpath)
				excludePatterns[mpattern] = nil
			}
		}

		continueAfter, err := getContinueAfter(ctx)
		if err != nil {
			fmt.Printf("docker-slim[build]: invalid continue-after mode: %v\n", err)
			return err
		}

		if !doHTTPProbe && continueAfter.Mode == "probe" {
			fmt.Printf("docker-slim[build]: info=probe message='changing continue-after from probe to enter because http-probe is disabled'\n")
			continueAfter.Mode = "enter"
		}

		commandReport := ctx.GlobalString(FlagCommandReport)
		if commandReport == "off" {
			commandReport = ""
		}

		ec := &commands.ExecutionContext{}

		commands.OnBuild(
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
			parseImageOverrides(doImageOverrides),
			overrides,
			instructions,
			ctx.StringSlice(FlagLink),
			ctx.StringSlice(FlagEtcHostsMap),
			ctx.StringSlice(FlagContainerDNS),
			ctx.StringSlice(FlagContainerDNSSearch),
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
			ec)
		commands.ShowCommunityInfo()
		return nil
	},
}
