package profile

import (
	"fmt"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"
	"github.com/docker-slim/docker-slim/internal/app/master/config"

	"github.com/urfave/cli"
)

const (
	Name  = "profile"
	Usage = "Collects fat image information and generates a fat container report"
	Alias = "p"
)

var CLI = cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		commands.Cflag(commands.FlagTarget),
		commands.Cflag(commands.FlagPull),
		commands.Cflag(commands.FlagShowPullLogs),
		commands.Cflag(commands.FlagShowContainerLogs),
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
		commands.Cflag(commands.FlagCopyMetaArtifacts),
		commands.Cflag(commands.FlagRemoveFileArtifacts),
		//Container Run Options
		commands.Cflag(commands.FlagCRORuntime),
		//
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
		commands.Cflag(commands.FlagExcludeMounts),
		commands.Cflag(commands.FlagExcludePattern),
		commands.Cflag(commands.FlagPathPerms),
		commands.Cflag(commands.FlagPathPermsFile),
		commands.Cflag(commands.FlagIncludePath),
		commands.Cflag(commands.FlagIncludePathFile),
		commands.Cflag(commands.FlagIncludeBin),
		commands.Cflag(commands.FlagIncludeExe),
		commands.Cflag(commands.FlagIncludeShell),
		commands.Cflag(commands.FlagMount),
		commands.Cflag(commands.FlagContinueAfter),
		commands.Cflag(commands.FlagUseLocalMounts),
		commands.Cflag(commands.FlagUseSensorVolume),
		commands.Cflag(commands.FlagKeepTmpArtifacts),
	},
	Action: func(ctx *cli.Context) error {
		commands.ShowCommunityInfo()
		xc := commands.NewExecutionContext(Name)

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

		crOpts, err := commands.GetContainerRunOptions(ctx)
		if err != nil {
			xc.Out.Error("param.error.container.run.options", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			commands.Exit(-1)
		}

		doPull := ctx.Bool(commands.FlagPull)
		doShowPullLogs := ctx.Bool(commands.FlagShowPullLogs)

		doRmFileArtifacts := ctx.Bool(commands.FlagRemoveFileArtifacts)
		doCopyMetaArtifacts := ctx.String(commands.FlagCopyMetaArtifacts)

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

		httpProbeRetryCount := ctx.Int(commands.FlagHTTPProbeRetryCount)
		httpProbeRetryWait := ctx.Int(commands.FlagHTTPProbeRetryWait)
		httpProbePorts, err := commands.ParseHTTPProbesPorts(ctx.String(commands.FlagHTTPProbePorts))
		if err != nil {
			fmt.Printf("docker-slim[profile]: invalid HTTP Probe target ports: %v\n", err)
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
				fmt.Printf("docker-slim[profile]: invalid spec file name='%s' error='%v'\n", k, v)
			}

			return err
		}

		if len(httpProbeAPISpecFiles) > 0 {
			doHTTPProbe = true
		}

		doKeepPerms := ctx.Bool(commands.FlagKeepPerms)

		doRunTargetAsUser := ctx.Bool(commands.FlagRunTargetAsUser)

		doShowContainerLogs := ctx.Bool(commands.FlagShowContainerLogs)
		overrides, err := commands.GetContainerOverrides(ctx)
		if err != nil {
			fmt.Printf("docker-slim[profile]: invalid container overrides: %v", err)
			return err
		}

		volumeMounts, err := commands.ParseVolumeMounts(ctx.StringSlice(commands.FlagMount))
		if err != nil {
			fmt.Printf("docker-slim[profile]: invalid volume mounts: %v\n", err)
			return err
		}

		excludePatterns := commands.ParsePaths(ctx.StringSlice(commands.FlagExcludePattern))

		includePaths := commands.ParsePaths(ctx.StringSlice(commands.FlagIncludePath))
		moreIncludePaths, err := commands.ParsePathsFile(ctx.String(commands.FlagIncludePathFile))
		if err != nil {
			fmt.Printf("docker-slim[profile]: could not read include path file (ignoring): %v\n", err)
		} else {
			for k, v := range moreIncludePaths {
				includePaths[k] = v
			}
		}

		pathPerms := commands.ParsePaths(ctx.StringSlice(commands.FlagPathPerms))
		morePathPerms, err := commands.ParsePathsFile(ctx.String(commands.FlagPathPermsFile))
		if err != nil {
			fmt.Printf("docker-slim[profile]: could not read path perms file (ignoring): %v\n", err)
		} else {
			for k, v := range morePathPerms {
				pathPerms[k] = v
			}
		}

		includeBins := commands.ParsePaths(ctx.StringSlice(commands.FlagIncludeBin))
		includeExes := commands.ParsePaths(ctx.StringSlice(commands.FlagIncludeExe))
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
			fmt.Printf("docker-slim[profile]: invalid continue-after mode: %v\n", err)
			return err
		}

		if !doHTTPProbe && continueAfter.Mode == "probe" {
			fmt.Printf("docker-slim[profile]: info=probe message='changing continue-after from probe to enter because http-probe is disabled'\n")
			continueAfter.Mode = "enter"
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
			doShowPullLogs,
			crOpts,
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
			continueAfter)
		commands.ShowCommunityInfo()
		return nil
	},
}
