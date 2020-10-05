package app

import (
	"fmt"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"
	"github.com/docker-slim/docker-slim/internal/app/master/config"

	"github.com/urfave/cli"
)

var cmdProfile = cli.Command{
	Name:    cmdSpecs[CmdProfile].name,
	Aliases: []string{cmdSpecs[CmdProfile].alias},
	Usage:   cmdSpecs[CmdProfile].usage,
	Flags: []cli.Flag{
		cflag(FlagTarget),
		cflag(FlagShowContainerLogs),
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
		cflag(FlagCopyMetaArtifacts),
		cflag(FlagRemoveFileArtifacts),
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
		cflag(FlagExcludeMounts),
		cflag(FlagExcludePattern),
		cflag(FlagPathPerms),
		cflag(FlagPathPermsFile),
		cflag(FlagIncludePath),
		cflag(FlagIncludePathFile),
		cflag(FlagIncludeBin),
		cflag(FlagIncludeExe),
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
				fmt.Printf("docker-slim[profile]: missing image ID/name...\n\n")
				cli.ShowCommandHelp(ctx, CmdProfile)
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
			fmt.Printf("docker-slim[profile]: invalid HTTP probes: %v\n", err)
			return err
		}

		if doHTTPProbe {
			//add default probe cmd if the "http-probe" flag is set
			fmt.Println("docker-slim[profile]: info=http.probe message='using default probe'")
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
			fmt.Printf("docker-slim[profile]: invalid HTTP Probe target ports: %v\n", err)
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
				fmt.Printf("docker-slim[profile]: invalid spec file name='%s' error='%v': %v\n", k, v)
			}

			return err
		}

		if len(httpProbeAPISpecFiles) > 0 {
			doHTTPProbe = true
		}

		doKeepPerms := ctx.Bool(FlagKeepPerms)

		doRunTargetAsUser := ctx.Bool(FlagRunTargetAsUser)

		doShowContainerLogs := ctx.Bool(FlagShowContainerLogs)
		overrides, err := getContainerOverrides(ctx)
		if err != nil {
			fmt.Printf("docker-slim[profile]: invalid container overrides: %v", err)
			return err
		}

		volumeMounts, err := parseVolumeMounts(ctx.StringSlice(FlagMount))
		if err != nil {
			fmt.Printf("docker-slim[profile]: invalid volume mounts: %v\n", err)
			return err
		}

		excludePatterns := parsePaths(ctx.StringSlice(FlagExcludePattern))

		includePaths := parsePaths(ctx.StringSlice(FlagIncludePath))
		moreIncludePaths, err := parsePathsFile(ctx.String(FlagIncludePathFile))
		if err != nil {
			fmt.Printf("docker-slim[profile]: could not read include path file (ignoring): %v\n", err)
		} else {
			for k, v := range moreIncludePaths {
				includePaths[k] = v
			}
		}

		pathPerms := parsePaths(ctx.StringSlice(FlagPathPerms))
		morePathPerms, err := parsePathsFile(ctx.String(FlagPathPermsFile))
		if err != nil {
			fmt.Printf("docker-slim[profile]: could not read path perms file (ignoring): %v\n", err)
		} else {
			for k, v := range morePathPerms {
				pathPerms[k] = v
			}
		}

		includeBins := parsePaths(ctx.StringSlice(FlagIncludeBin))
		includeExes := parsePaths(ctx.StringSlice(FlagIncludeExe))
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
			fmt.Printf("docker-slim[profile]: invalid continue-after mode: %v\n", err)
			return err
		}

		if !doHTTPProbe && continueAfter.Mode == "probe" {
			fmt.Printf("docker-slim[profile]: info=probe message='changing continue-after from probe to enter because http-probe is disabled'\n")
			continueAfter.Mode = "enter"
		}

		commandReport := ctx.GlobalString(FlagCommandReport)
		if commandReport == "off" {
			commandReport = ""
		}

		ec := &commands.ExecutionContext{}

		commands.OnProfile(
			gcvalues,
			targetRef,
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
			overrides,
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
