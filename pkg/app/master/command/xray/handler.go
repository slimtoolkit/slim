package xray

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	//"github.com/bmatcuk/doublestar/v3"
	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/app/master/inspectors/image"
	"github.com/slimtoolkit/slim/pkg/app/master/version"
	cmd "github.com/slimtoolkit/slim/pkg/command"
	"github.com/slimtoolkit/slim/pkg/docker/buildpackinfo"
	"github.com/slimtoolkit/slim/pkg/docker/dockerclient"
	"github.com/slimtoolkit/slim/pkg/docker/dockerimage"
	"github.com/slimtoolkit/slim/pkg/docker/dockerutil"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
	v "github.com/slimtoolkit/slim/pkg/version"
)

const appName = command.AppName

type ovars = app.OutVars

// Xray command exit codes
const (
	ecxOther = iota + 1
)

const (
	fatDockerfileName = "Dockerfile.fat"
)

const (
	// image.ref.name and image.version are supposed to represent
	// the corresponding target image identity properties
	// but sometimes it's base image properties
	// (when the builder doesn't set its own values inheriting the base image tags)

	ociLabelImageName = "org.opencontainers.image.ref.name"

	ociLabelImageVersion = "org.opencontainers.image.version"
	lsLabelImageVersion  = "org.label-schema.version"

	// image build source info
	ociLabelImageSource = "org.opencontainers.image.source"
	lsLabelImageSource  = "org.label-schema.vcs-url"

	//SCM repo revision info (could be commit hash, tag, branch)
	ociLabelImageRevision = "org.opencontainers.image.revision"
	lsLabelImageRevision  = "org.label-schema.vcs-ref"

	ociLabelImageURL = "org.opencontainers.image.url"
	lsLabelImageURL  = "org.label-schema.url"

	ociLabelImageTitle = "org.opencontainers.image.title"
	lsLabelImageTitle  = "org.label-schema.name"

	ociLabelImageDesc = "org.opencontainers.image.description"
	lsLabelImageDesc  = "org.label-schema.description"

	ociLabelImageDocs = "org.opencontainers.image.documentation"
	lsLabelImageDocs  = "org.label-schema.usage"

	ociLabelImageVendor = "org.opencontainers.image.vendor" //high level 'author' info
	lsLabelImageVendor  = "org.label-schema.vendor"

	ociLabelImageAuthors      = "org.opencontainers.image.authors"
	ociLabelBaseImageDigest   = "org.opencontainers.image.base.digest"
	ociLabelBaseImageName     = "org.opencontainers.image.base.name"
	azureLabelBaseImageName   = "image.base.ref.name"
	azureLabelBaseImageDigest = "image.base.digest"

	lsLabelDockerCmd      = "org.label-schema.docker.cmd"
	lsLabelDockerCmdDevel = "org.label-schema.docker.cmd.devel"
	lsLabelDockerCmdTest  = "org.label-schema.docker.cmd.test"
	lsLabelDockerCmdDebug = "org.label-schema.docker.debug"
	lsLabelDockerCmdHelp  = "org.label-schema.docker.cmd.help"
	lsLabelDockerParams   = "org.label-schema.docker.params"
)

// OCI label info:
// https://specs.opencontainers.org/image-spec/annotations/

// OnCommand implements the 'xray' command
func OnCommand(
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	cparams *CommandParams,
	targetRef string,
	doPull bool,
	dockerConfigPath string,
	registryAccount string,
	registrySecret string,
	doShowPullLogs bool,
	changes map[string]struct{},
	changesOutputs map[string]struct{},
	layers map[string]struct{},
	layerChangesMax int,
	allChangesMax int,
	addChangesMax int,
	modifyChangesMax int,
	deleteChangesMax int,
	topChangesMax int,
	changePathMatchers []*dockerimage.ChangePathMatcher,
	changeDataMatcherList []*dockerimage.ChangeDataMatcher,
	changeDataHashMatcherList []*dockerimage.ChangeDataHashMatcher,
	doHashData bool,
	doDetectDuplicates bool,
	doShowDuplicates bool,
	doShowSpecialPerms bool,
	changeMatchLayersOnly bool,
	doAddImageManifest bool,
	doAddImageConfig bool,
	doReuseSavedImage bool,
	doRmFileArtifacts bool,
	utf8Detector *dockerimage.UTF8Detector,
	xdArtifactsPath string,
) {
	const cmdName = Name
	logger := log.WithFields(log.Fields{"app": appName, "cmd": cmdName})

	changeDataMatchers := map[string]*dockerimage.ChangeDataMatcher{}
	for _, cdm := range changeDataMatcherList {
		matcher, err := regexp.Compile(cdm.DataPattern)
		errutil.FailOn(err)

		cdm.Matcher = matcher
		changeDataMatchers[cdm.DataPattern] = cdm
	}

	changeDataHashMatchers := map[string]*dockerimage.ChangeDataHashMatcher{}
	for _, cdhm := range changeDataHashMatcherList {
		changeDataHashMatchers[cdhm.Hash] = cdhm
	}

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewXrayCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = cmd.StateStarted
	cmdReport.TargetReference = targetRef

	xc.Out.State(cmd.StateStarted)
	xc.Out.Info("params",
		ovars{
			"target":             targetRef,
			"add-image-manifest": doAddImageManifest,
			"add-image-config":   doAddImageConfig,
			"rm-file-artifacts":  doRmFileArtifacts,
		})

	client, err := dockerclient.New(gparams.ClientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if gparams.InContainer && gparams.IsDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
		}

		xc.Out.Error("docker.connect.error", exitMsg)

		exitCode := command.ECTCommon | command.ECCNoDockerConnectInfo
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})
		xc.Exit(exitCode)
	}
	errutil.FailOn(err)

	if gparams.Debug {
		version.Print(xc, cmdName, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	imageInspector, err := image.NewInspector(client, targetRef)
	errutil.FailOn(err)

	noImage, err := imageInspector.NoImage()
	errutil.FailOn(err)
	if noImage {
		if doPull {
			xc.Out.Info("target.image",
				ovars{
					"status":  "not.found",
					"image":   targetRef,
					"message": "trying to pull target image",
				})

			err := imageInspector.Pull(doShowPullLogs, dockerConfigPath, registryAccount, registrySecret)
			errutil.FailOn(err)
		} else {
			xc.Out.Error("image.not.found", "make sure the target image already exists locally (use --pull flag to auto-download it from registry)")

			exitCode := command.ECTCommon | command.ECCImageNotFound
			xc.Out.State("exited",
				ovars{
					"exit.code": exitCode,
				})
			xc.Exit(exitCode)
		}
	}

	//refresh the target refs
	targetRef = imageInspector.ImageRef
	cmdReport.TargetReference = imageInspector.ImageRef

	xc.Out.State("image.api.inspection.start")

	logger.Info("inspecting 'fat' image metadata...")
	err = imageInspector.Inspect()
	errutil.FailOn(err)

	localVolumePath, artifactLocation, statePath, stateKey := fsutil.PrepareImageStateDirs(gparams.StatePath, imageInspector.ImageInfo.ID)
	imageInspector.ArtifactLocation = artifactLocation
	logger.Debugf("localVolumePath=%v, artifactLocation=%v, statePath=%v, stateKey=%v", localVolumePath, artifactLocation, statePath, stateKey)

	xc.Out.Info("image",
		ovars{
			"id":           imageInspector.ImageInfo.ID,
			"size.bytes":   imageInspector.ImageInfo.VirtualSize,
			"size.human":   humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)),
			"architecture": imageInspector.ImageInfo.Architecture,
		})

	logger.Info("processing 'fat' image info...")
	err = imageInspector.ProcessCollectedData()
	errutil.FailOn(err)

	if imageInspector.DockerfileInfo != nil {
		if imageInspector.DockerfileInfo.ExeUser != "" {
			xc.Out.Info("image.users",
				ovars{
					"exe": imageInspector.DockerfileInfo.ExeUser,
					"all": strings.Join(imageInspector.DockerfileInfo.AllUsers, ","),
				})
		}

		if len(imageInspector.DockerfileInfo.ImageStack) > 0 {
			cmdReport.ImageStack = imageInspector.DockerfileInfo.ImageStack

			for idx, imageInfo := range imageInspector.DockerfileInfo.ImageStack {
				xc.Out.Info("image.stack",
					ovars{
						"index":        idx,
						"name":         imageInfo.FullName,
						"id":           imageInfo.ID,
						"instructions": len(imageInfo.Instructions),
						"message":      "see report file for details",
					})
			}
		}

		if len(imageInspector.DockerfileInfo.ExposedPorts) > 0 {
			xc.Out.Info("image.exposed_ports",
				ovars{
					"list": strings.Join(imageInspector.DockerfileInfo.ExposedPorts, ","),
				})
		}
	}

	imgIdentity := dockerutil.ImageToIdentity(imageInspector.ImageInfo)
	cmdReport.SourceImage = report.ImageMetadata{
		Identity: report.ImageIdentity{
			ID:          imgIdentity.ID,
			Tags:        imgIdentity.ShortTags,
			Names:       imgIdentity.RepoTags,
			Digests:     imgIdentity.ShortDigests,
			FullDigests: imgIdentity.RepoDigests,
		},
		Size:          imageInspector.ImageInfo.VirtualSize,
		SizeHuman:     humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)),
		CreateTime:    imageInspector.ImageInfo.Created.UTC().Format(time.RFC3339),
		Author:        imageInspector.ImageInfo.Author,
		DockerVersion: imageInspector.ImageInfo.DockerVersion,
		Architecture:  imageInspector.ImageInfo.Architecture,
		User:          imageInspector.ImageInfo.Config.User,
		OS:            imageInspector.ImageInfo.OS,
		WorkDir:       imageInspector.ImageInfo.Config.WorkingDir,
		ContainerEntry: report.ContainerEntryInfo{
			Entrypoint: imageInspector.ImageInfo.Config.Entrypoint,
			Cmd:        imageInspector.ImageInfo.Config.Cmd,
		},
		InheritedInstructions: imageInspector.ImageInfo.Config.OnBuild,
	}

	cmdReport.SourceImage.EnvVars = imageInspector.ImageInfo.Config.Env

	for k := range imageInspector.ImageInfo.Config.ExposedPorts {
		cmdReport.SourceImage.ExposedPorts = append(cmdReport.SourceImage.ExposedPorts, string(k))
	}

	for k := range imageInspector.ImageInfo.Config.Volumes {
		cmdReport.SourceImage.Volumes = append(cmdReport.SourceImage.Volumes, k)
	}

	cmdReport.SourceImage.Labels = imageInspector.ImageInfo.Config.Labels

	maintainers := map[string]struct{}{}

	if buildpackinfo.HasBuildbackLabels(imageInspector.ImageInfo.Config.Labels) {
		for k, v := range imageInspector.ImageInfo.Config.Labels {
			if k == "io.buildpacks.stack.maintainer" {
				buildpackMaintainer := v + " (buildpack)"
				maintainers[buildpackMaintainer] = struct{}{}
			}
		}

		bpStack := imageInspector.ImageInfo.Config.Labels[buildpackinfo.LabelKeyStackID]
		cmdReport.SourceImage.Buildpack = &report.BuildpackInfo{
			Stack: bpStack,
		}
	}

	for _, m := range imageInspector.DockerfileInfo.Maintainers {
		maintainers[m] = struct{}{}
	}

	for k, v := range cmdReport.SourceImage.Labels {
		if strings.ToLower(k) == "maintainer" || strings.ToLower(k) == "author" || strings.ToLower(k) == "authors" || k == ociLabelImageAuthors {
			maintainers[v] = struct{}{}
		} else {
			switch k {
			case ociLabelBaseImageDigest, azureLabelBaseImageDigest:
				cmdReport.SourceImage.BaseImageDigest = v
			case ociLabelBaseImageName, azureLabelBaseImageName:
				cmdReport.SourceImage.BaseImageName = v
			}
		}
	}

	for m := range maintainers {
		cmdReport.SourceImage.Maintainers = append(cmdReport.SourceImage.Maintainers, m)
	}

	cmdReport.ArtifactLocation = imageInspector.ArtifactLocation

	xc.Out.State("image.api.inspection.done")
	xc.Out.State("image.data.inspection.start")

	imageID := dockerutil.CleanImageID(imageInspector.ImageInfo.ID)
	iaName := fmt.Sprintf("%s.tar", imageID)
	iaPath := filepath.Join(localVolumePath, "image", iaName)
	iaPathReady := fmt.Sprintf("%s.ready", iaPath)

	var doSave bool
	if fsutil.IsRegularFile(iaPath) {
		if !doReuseSavedImage {
			doSave = true
		}

		if !fsutil.Exists(iaPathReady) {
			doSave = true
		}
	} else {
		doSave = true
	}

	if doSave {
		if fsutil.Exists(iaPathReady) {
			fsutil.Remove(iaPathReady)
		}

		xc.Out.Info("image.data.inspection.save.image.start")
		err = dockerutil.SaveImage(client, imageID, iaPath, false, false)
		errutil.FailOn(err)

		err = fsutil.Touch(iaPathReady)
		errutil.WarnOn(err)

		xc.Out.Info("image.data.inspection.save.image.end")
	} else {
		logger.Debugf("exported image already exists - %s", iaPath)
	}

	pp := &dockerimage.ProcessorParams{
		DetectIdentities: &dockerimage.DetectOpParam{
			Enabled:      cparams.DetectIdentities.Enabled,
			DumpRaw:      cparams.DetectIdentities.DumpRaw,
			IsConsoleOut: cparams.DetectIdentities.IsConsoleOut,
			IsDirOut:     cparams.DetectIdentities.IsDirOut,
			OutputPath:   cparams.DetectIdentities.OutputPath,
			InputParams:  cparams.DetectIdentities.InputParams,
		},
		DetectAllCertFiles:   cparams.DetectAllCertFiles,
		DetectAllCertPKFiles: cparams.DetectAllCertPKFiles,
	}

	xc.Out.Info("image.data.inspection.process.image.start")
	imagePkg, err := dockerimage.LoadPackage(
		iaPath,
		imageID,
		false,
		topChangesMax,
		doHashData,
		doDetectDuplicates,
		changeDataHashMatchers,
		changePathMatchers,
		changeDataMatchers,
		utf8Detector,
		pp)

	errutil.FailOn(err)
	xc.Out.Info("image.data.inspection.process.image.end")

	if utf8Detector != nil {
		errutil.FailOn(utf8Detector.Close())
	}

	xc.Out.State("image.data.inspection.done")

	if len(imageInspector.DockerfileInfo.AllInstructions) == len(imagePkg.Config.History) {
		for instIdx, instInfo := range imageInspector.DockerfileInfo.AllInstructions {
			instInfo.Author = imagePkg.Config.History[instIdx].Author
			instInfo.EmptyLayer = imagePkg.Config.History[instIdx].EmptyLayer
			instInfo.LayerID = imagePkg.Config.History[instIdx].LayerID
			instInfo.LayerIndex = imagePkg.Config.History[instIdx].LayerIndex
			instInfo.LayerFSDiffID = imagePkg.Config.History[instIdx].LayerFSDiffID
		}
	} else {
		logger.Debugf("history instruction set size mismatch - %v/%v ",
			len(imageInspector.DockerfileInfo.AllInstructions),
			len(imagePkg.Config.History))
	}

	allEntryParams := append(cmdReport.SourceImage.ContainerEntry.Entrypoint,
		cmdReport.SourceImage.ContainerEntry.Cmd...)
	if len(allEntryParams) > 0 {
		cmdReport.SourceImage.ContainerEntry.ExePath = allEntryParams[0]
		cmdReport.SourceImage.ContainerEntry.ExeArgs = allEntryParams[1:]

		//fix up exe path if relative
		if !strings.HasPrefix(cmdReport.SourceImage.ContainerEntry.ExePath, "/") {
			//check relative path
			if strings.HasPrefix(cmdReport.SourceImage.ContainerEntry.ExePath, "./") ||
				strings.HasPrefix(cmdReport.SourceImage.ContainerEntry.ExePath, "../") {

				fullExePath := filepath.Join(cmdReport.SourceImage.WorkDir, cmdReport.SourceImage.ContainerEntry.ExePath)
				object := findChange(imagePkg, fullExePath)
				if object != nil {
					cmdReport.SourceImage.ContainerEntry.FullExePath =
						&report.ContainerFileInfo{
							Name:  fullExePath,
							Layer: object.LayerIndex,
						}
				}
			} else {
				//check env paths
				var envPaths []string
				for _, envInfo := range cmdReport.SourceImage.EnvVars {
					if strings.HasPrefix(envInfo, "PATH=") {
						envInfo = strings.TrimPrefix(envInfo, "PATH=")
						envPaths = strings.Split(envInfo, ":")
						break
					}
				}

				for _, envPath := range envPaths {
					fullExePath := fmt.Sprintf("%s/%s", envPath, cmdReport.SourceImage.ContainerEntry.ExePath)
					object := findChange(imagePkg, fullExePath)
					if object != nil {
						cmdReport.SourceImage.ContainerEntry.FullExePath =
							&report.ContainerFileInfo{
								Name:  fullExePath,
								Layer: object.LayerIndex,
							}
						break
					}
				}
			}
		} else {
			object := findChange(imagePkg, cmdReport.SourceImage.ContainerEntry.ExePath)
			if object != nil {
				cmdReport.SourceImage.ContainerEntry.FullExePath =
					&report.ContainerFileInfo{
						Name:  cmdReport.SourceImage.ContainerEntry.ExePath,
						Layer: object.LayerIndex,
					}
			}
		}

		//find files in exe args
		for _, exeArg := range cmdReport.SourceImage.ContainerEntry.ExeArgs {
			//if starts with / assume a full path and lookup/find in the layer references
			//otherwise try to use workdir and lookup/find in the layer references
			if strings.HasPrefix(exeArg, "-") {
				//skip flag names (might have false positives)
				continue
			}

			var filePath string
			if strings.HasPrefix(exeArg, "/") {
				filePath = exeArg
			} else {
				//not a perfect way to find potential files
				//but better than nothing
				filePath = fmt.Sprintf("%s/%s", cmdReport.SourceImage.WorkDir, exeArg)
			}

			object := findChange(imagePkg, filePath)
			if object != nil {
				cmdReport.SourceImage.ContainerEntry.ArgFiles =
					append(cmdReport.SourceImage.ContainerEntry.ArgFiles,
						&report.ContainerFileInfo{
							Name:  filePath,
							Layer: object.LayerIndex,
						})
				break
			}
		}
	}

	printImagePackage(
		xc,
		imagePkg,
		appName,
		cmdName,
		changes,
		changesOutputs,
		layers,
		layerChangesMax,
		allChangesMax,
		addChangesMax,
		modifyChangesMax,
		deleteChangesMax,
		doHashData,
		doDetectDuplicates,
		doShowDuplicates,
		doShowSpecialPerms,
		changeMatchLayersOnly,
		changeDataHashMatchers,
		changePathMatchers,
		changeDataMatchers,
		cparams,
		cmdReport)

	if doAddImageManifest {
		cmdReport.RawImageManifest = imagePkg.Manifest
	}

	if doAddImageConfig {
		cmdReport.RawImageConfig = imagePkg.Config
	}

	cmdReport.ImageReport.BuildInfo = imagePkg.Config.BuildInfoDecoded

	if (cmdReport.SourceImage.BaseImageDigest == "" || cmdReport.SourceImage.BaseImageName == "") &&
		cmdReport.ImageReport.BuildInfo != nil &&
		len(cmdReport.ImageReport.BuildInfo.Sources) > 0 {
		var lastImageSource *dockerimage.BuildSource
		for _, s := range cmdReport.ImageReport.BuildInfo.Sources {
			if s.Type == dockerimage.SourceTypeDockerImage {
				lastImageSource = s
			}
		}

		if lastImageSource != nil {
			if lastImageSource.Ref != "" && cmdReport.SourceImage.BaseImageName == "" {
				cmdReport.SourceImage.BaseImageName = lastImageSource.Ref
			}

			if lastImageSource.Pin != "" && cmdReport.SourceImage.BaseImageDigest == "" {
				cmdReport.SourceImage.BaseImageDigest = lastImageSource.Pin
			}
		}
	}

	xc.Out.State(cmd.StateCompleted)
	cmdReport.State = cmd.StateCompleted

	if doRmFileArtifacts {
		logger.Info("removing temporary artifacts...")
		err = fsutil.Remove(iaPath)
		errutil.WarnOn(err)
	} else {
		cmdReport.ImageArchiveLocation = iaPath
	}

	xc.Out.State(cmd.StateDone)

	xc.Out.Info("results",
		ovars{
			"artifacts.location": cmdReport.ArtifactLocation,
		})

	xc.Out.Info("results",
		ovars{
			"artifacts.dockerfile.original": "Dockerfile.fat",
		})

	vinfo := <-viChan
	version.PrintCheckVersion(xc, "", vinfo)

	cmdReport.State = cmd.StateDone
	if cmdReport.Save() {
		xc.Out.Info("report",
			ovars{
				"file": cmdReport.ReportLocation(),
			})
	}

	if xdArtifactsPath != "" {
		var filesToExport []string
		filesToExport = append(filesToExport, cmdReport.ReportLocation())
		filesToExport = append(filesToExport, filepath.Join(cmdReport.ArtifactLocation, fatDockerfileName))
		if utf8Detector.DumpArchive != "" {
			filesToExport = append(filesToExport, utf8Detector.DumpArchive)
		}

		if xdArtifactsPath == "." {
			xdArtifactsPath = "data-artifacts.tar"
		}

		if err := fsutil.ArchiveFiles(xdArtifactsPath, filesToExport, true, ""); err == nil {
			xc.Out.Info("exported-data-artifacts",
				ovars{
					"file": xdArtifactsPath,
				})
		} else {
			logger.Errorf("error exporting data artifacts (%s) - %v", xdArtifactsPath, err)
		}
	}
}

func findChange(pkg *dockerimage.Package, filepath string) *dockerimage.ObjectMetadata {
	for _, layer := range pkg.Layers {
		if object, found := layer.References[filepath]; found {
			return object
		}
	}

	return nil
}

func printImagePackage(
	xc *app.ExecutionContext,
	pkg *dockerimage.Package,
	appName string,
	cmdName cmd.Type,
	changes map[string]struct{},
	changesOutputs map[string]struct{},
	layers map[string]struct{},
	layerChangesMax int,
	allChangesMax int,
	addChangesMax int,
	modifyChangesMax int,
	deleteChangesMax int,
	doHashData bool,
	doDetectDuplicates bool,
	doShowDuplicates bool,
	doShowSpecialPerms bool,
	changeMatchLayersOnly bool,
	changeDataHashMatchers map[string]*dockerimage.ChangeDataHashMatcher,
	changePathMatchers []*dockerimage.ChangePathMatcher,
	changeDataMatchers map[string]*dockerimage.ChangeDataMatcher,
	cparams *CommandParams,
	cmdReport *report.XrayCommand) {
	var allChangesCount int
	var addChangesCount int
	var modifyChangesCount int
	var deleteChangesCount int

	xc.Out.Info("image.package.details")
	xc.Out.Info("layers.count",
		ovars{
			"value": len(pkg.Layers),
		})

	//NOTE: all this cmdReport.ImageReport logic should be moved outside of this function
	cmdReport.ImageReport = &dockerimage.ImageReport{
		Stats: pkg.Stats,
	}

	if cparams.DetectIdentities.Enabled {
		cmdReport.ImageReport.Identities = pkg.ProcessIdentityData()

		if cmdReport.ImageReport.Identities != nil {
			xc.Out.Info("image.identities.stats",
				ovars{
					"user_count":  len(cmdReport.ImageReport.Identities.Users),
					"group_count": len(cmdReport.ImageReport.Identities.Groups),
				})

			for username, userInfo := range cmdReport.ImageReport.Identities.Users {
				xc.Out.Info("image.identities.user",
					ovars{
						"username":          username,
						"uid":               userInfo.UID,
						"home":              userInfo.Home,
						"shell":             userInfo.Shell,
						"no_login_shell":    userInfo.NoLoginShell,
						"no_password_login": userInfo.ShadowPassword.NoPasswordLogin,
					})
			}
		}
	}

	for k := range pkg.Certs.Bundles {
		cmdReport.ImageReport.Certs.Bundles =
			append(cmdReport.ImageReport.Certs.Bundles, k)
	}

	for k := range pkg.Certs.Files {
		cmdReport.ImageReport.Certs.Files =
			append(cmdReport.ImageReport.Certs.Files, k)
	}

	cmdReport.ImageReport.Certs.Links = pkg.Certs.Links
	cmdReport.ImageReport.Certs.Hashes = pkg.Certs.Hashes

	for k := range pkg.Certs.PrivateKeys {
		cmdReport.ImageReport.Certs.PrivateKeys =
			append(cmdReport.ImageReport.Certs.PrivateKeys, k)
	}

	cmdReport.ImageReport.Certs.PrivateKeyLinks = pkg.Certs.PrivateKeyLinks

	for k := range pkg.CACerts.Bundles {
		cmdReport.ImageReport.CACerts.Bundles =
			append(cmdReport.ImageReport.CACerts.Bundles, k)
	}

	for k := range pkg.CACerts.Files {
		cmdReport.ImageReport.CACerts.Files =
			append(cmdReport.ImageReport.CACerts.Files, k)
	}

	cmdReport.ImageReport.CACerts.Links = pkg.CACerts.Links
	cmdReport.ImageReport.CACerts.Hashes = pkg.CACerts.Hashes

	for k := range pkg.CACerts.PrivateKeys {
		cmdReport.ImageReport.CACerts.PrivateKeys =
			append(cmdReport.ImageReport.CACerts.PrivateKeys, k)
	}

	cmdReport.ImageReport.CACerts.PrivateKeyLinks = pkg.CACerts.PrivateKeyLinks

	if doDetectDuplicates && pkg.Stats.DuplicateFileCount > 0 {
		xc.Out.Info("image.stats.duplicates",
			ovars{
				"file_count":            pkg.Stats.DuplicateFileCount,
				"file_total_count":      pkg.Stats.DuplicateFileTotalCount,
				"file_size.bytes":       pkg.Stats.DuplicateFileSize,
				"file_size.human":       humanize.Bytes(pkg.Stats.DuplicateFileSize),
				"file_total_size.bytes": pkg.Stats.DuplicateFileTotalSize,
				"file_total_size.human": humanize.Bytes(pkg.Stats.DuplicateFileTotalSize),
				"wasted.bytes":          pkg.Stats.DuplicateFileWastedSize,
				"wasted.human":          humanize.Bytes(pkg.Stats.DuplicateFileWastedSize),
			})
	}

	doShow := func(changeMatchLayersOnly bool, layer *dockerimage.Layer) bool {
		if !changeMatchLayersOnly || (changeMatchLayersOnly && layer.HasMatches()) {
			return true
		}

		return false
	}

	for _, layer := range pkg.Layers {
		layerInfo := ovars{
			"index": layer.Index,
			"id":    layer.ID,
			"path":  layer.Path,
		}

		if layer.MetadataChangesOnly {
			layerInfo["metadata_change_only"] = true
		}

		if layer.LayerDataSource != "" {
			layerInfo["layer_data_source"] = layer.LayerDataSource
		}

		if doShow(changeMatchLayersOnly, layer) {
			xc.Out.Info("layer.start")
			xc.Out.Info("layer", layerInfo)
		}

		var layerChangesCount int

		if layer.Distro != nil {
			distro := &report.DistroInfo{
				Name:        layer.Distro.Name,
				Version:     layer.Distro.Version,
				DisplayName: layer.Distro.DisplayName,
			}

			xc.Out.Info("distro",
				ovars{
					"name":    distro.Name,
					"version": distro.Version,
					"display": distro.DisplayName,
				})

			cmdReport.SourceImage.Distro = distro
		}

		topList := layer.Top.List()

		layerReport := dockerimage.LayerReport{
			ID:                  layer.ID,
			Index:               layer.Index,
			Path:                layer.Path,
			LayerDataSource:     layer.LayerDataSource,
			MetadataChangesOnly: layer.MetadataChangesOnly,
			FSDiffID:            layer.FSDiffID,
			Stats:               layer.Stats,
		}

		layerReport.Changes.Deleted = uint64(len(layer.Changes.Deleted))
		layerReport.Changes.Modified = uint64(len(layer.Changes.Modified))
		layerReport.Changes.Added = uint64(len(layer.Changes.Added))

		layerReport.Top = topList

		for imgIdx, imgInfo := range cmdReport.ImageStack {
			for instIdx, instInfo := range imgInfo.Instructions {
				if layerReport.ID == instInfo.LayerID {
					if !instInfo.EmptyLayer {
						if layerReport.ChangeInstruction != nil {
							log.Debugf("overwriting existing layerReport.ChangeInstruction = %#v", layerReport.ChangeInstruction)
						}

						layerReport.ChangeInstruction = &dockerimage.InstructionSummary{
							Index:      instIdx,
							ImageIndex: imgIdx,
							Type:       instInfo.Type,
							All:        instInfo.CommandAll,
							Snippet:    instInfo.CommandSnippet,
						}
					} else {
						extraInst := &dockerimage.InstructionSummary{
							Index:      instIdx,
							ImageIndex: imgIdx,
							Type:       instInfo.Type,
							All:        instInfo.CommandAll,
							Snippet:    instInfo.CommandSnippet,
						}

						layerReport.OtherInstructions = append(layerReport.OtherInstructions, extraInst)
					}
				}
			}
		}

		cmdReport.ImageLayers = append(cmdReport.ImageLayers, &layerReport)

		if layerReport.ChangeInstruction != nil {
			if doShow(changeMatchLayersOnly, layer) {
				xc.Out.Info("change.instruction",
					ovars{
						"index":   fmt.Sprintf("%d:%d", layerReport.ChangeInstruction.ImageIndex, layerReport.ChangeInstruction.Index),
						"type":    layerReport.ChangeInstruction.Type,
						"snippet": layerReport.ChangeInstruction.Snippet,
						"all":     layerReport.ChangeInstruction.All,
					})
			}

		}

		if doShow(changeMatchLayersOnly, layer) {
			if layerReport.OtherInstructions != nil {
				xc.Out.Info("other.instructions",
					ovars{
						"count": len(layerReport.OtherInstructions),
					})

				for idx, info := range layerReport.OtherInstructions {
					xc.Out.Info("other.instruction",
						ovars{
							"pos":     idx,
							"index":   fmt.Sprintf("%d:%d", info.ImageIndex, info.Index),
							"type":    info.Type,
							"snippet": info.Snippet,
							"all":     info.All,
						})
				}
			}

			if layer.Stats.AllSize != 0 {
				xc.Out.Info("layer.stats",
					ovars{
						"all_size.human": humanize.Bytes(uint64(layer.Stats.AllSize)),
						"all_size.bytes": layer.Stats.AllSize,
					})
			}

			if layer.Stats.ObjectCount != 0 {
				xc.Out.Info("layer.stats",
					ovars{
						"object_count": layer.Stats.ObjectCount,
					})
			}

			if layer.Stats.DirCount != 0 {
				xc.Out.Info("layer.stats",
					ovars{
						"dir_count": layer.Stats.DirCount,
					})
			}

			if layer.Stats.FileCount != 0 {
				xc.Out.Info("layer.stats",
					ovars{
						"file_count": layer.Stats.FileCount,
					})
			}

			if layer.Stats.LinkCount != 0 {
				xc.Out.Info("layer.stats",
					ovars{
						"link_count": layer.Stats.LinkCount,
					})
			}

			if layer.Stats.MaxFileSize != 0 {
				xc.Out.Info("layer.stats",
					ovars{
						"max_file_size.human": humanize.Bytes(uint64(layer.Stats.MaxFileSize)),
						"max_file_size.bytes": layer.Stats.MaxFileSize,
					})
			}

			if layer.Stats.DeletedCount != 0 {
				xc.Out.Info("layer.stats",
					ovars{
						"deleted_count": layer.Stats.DeletedCount,
					})
			}

			if layer.Stats.DeletedDirCount != 0 {
				xc.Out.Info("layer.stats",
					ovars{
						"deleted_dir_count": layer.Stats.DeletedDirCount,
					})
			}

			if layer.Stats.DeletedFileCount != 0 {
				xc.Out.Info("layer.stats",
					ovars{
						"deleted_file_count": layer.Stats.DeletedFileCount,
					})
			}

			if layer.Stats.DeletedLinkCount != 0 {
				xc.Out.Info("layer.stats",
					ovars{
						"deleted_link_count": layer.Stats.DeletedLinkCount,
					})
			}

			if layer.Stats.DeletedSize != 0 {
				xc.Out.Info("layer.stats",
					ovars{
						"deleted_size": layer.Stats.DeletedSize,
					})
			}

			if layer.Stats.AddedSize != 0 {
				xc.Out.Info("layer.stats",
					ovars{
						"added_size.human": humanize.Bytes(uint64(layer.Stats.AddedSize)),
						"added_size.bytes": layer.Stats.AddedSize,
					})
			}

			if layer.Stats.ModifiedSize != 0 {
				xc.Out.Info("layer.stats",
					ovars{
						"modified_size.human": humanize.Bytes(uint64(layer.Stats.ModifiedSize)),
						"modified_size.bytes": layer.Stats.ModifiedSize,
					})
			}
		}

		changeCount := len(layer.Changes.Deleted) + len(layer.Changes.Modified) + len(layer.Changes.Added)

		if doShow(changeMatchLayersOnly, layer) {
			xc.Out.Info("layer.change.summary",
				ovars{
					"deleted":  len(layer.Changes.Deleted),
					"modified": len(layer.Changes.Modified),
					"added":    len(layer.Changes.Added),
					"all":      changeCount,
				})

			xc.Out.Info("layer.objects.count",
				ovars{
					"value": len(layer.Objects),
				})

			if len(topList) > 0 {
				xc.Out.Info("layer.objects.top.start")
				for _, topObject := range topList {
					match := topObject.PathMatch

					if !match && len(changePathMatchers) > 0 {
						log.Tracef("Change path patterns, no match. skipping 'top' change ['%s']", topObject.Name)
						continue
					} else {
						if len(changeDataMatchers) > 0 {
							matchedPatterns, found := layer.DataMatches[topObject.Name]
							if !found {
								log.Tracef("Change data patterns, no match. skipping 'top' change ['%s']", topObject.Name)
								continue
							}

							log.Tracef("'%s' ('top' change) matched data patterns - %d", topObject.Name, len(matchedPatterns))
							for _, cdm := range matchedPatterns {
								log.Tracef("matched => PP='%s' DP='%s'", cdm.PathPattern, cdm.DataPattern)
							}
						} else {
							if len(changeDataHashMatchers) > 0 {
								matched, found := layer.DataHashMatches[topObject.Name]
								if !found {
									log.Trace("Change data hash patterns, no match. skipping 'top' change...")
									continue
								}

								log.Tracef("'%s' ('top' change) matched data hash pattern - %s", topObject.Name, matched.Hash)
							}
						}
					}

					printObject(xc, topObject)
				}
				xc.Out.Info("layer.objects.top.end")
			}
		}

		showLayer := true
		if len(layers) > 0 {
			showLayer = false
			_, hasID := layers[layer.ID]
			layerIdx := fmt.Sprintf("%v", layer.Index)
			_, hasIndex := layers[layerIdx]
			if hasID || hasIndex {
				showLayer = true
			}
		}

		if doShow(changeMatchLayersOnly, layer) && showLayer {
			if _, ok := changes["delete"]; ok && len(layer.Changes.Deleted) > 0 {
				xc.Out.Info("layer.objects.deleted.start")
				for _, objectIdx := range layer.Changes.Deleted {
					allChangesCount++
					deleteChangesCount++
					layerChangesCount++

					objectInfo := layer.Objects[objectIdx]

					//TODO: add a flag to select change type to apply path patterns
					match := objectInfo.PathMatch

					if !match && len(changePathMatchers) > 0 {
						log.Tracef("Change path patterns, no match. skipping 'delete' change ['%s']", objectInfo.Name)
						continue
					}

					//NOTE: not checking change data pattern matches for deletes

					if allChangesMax > -1 && allChangesCount > allChangesMax {
						break
					}

					if deleteChangesMax > -1 && deleteChangesCount > deleteChangesMax {
						break
					}

					if layerChangesMax > -1 && layerChangesCount > layerChangesMax {
						break
					}

					if _, ok := changesOutputs["report"]; ok {
						layerReport.Deleted = append(layerReport.Deleted, objectInfo)
					}

					if _, ok := changesOutputs["console"]; ok {
						printObject(xc, objectInfo)
					}
				}
				xc.Out.Info("layer.objects.deleted.end")
			}

			if _, ok := changes["modify"]; ok && len(layer.Changes.Modified) > 0 {
				xc.Out.Info("layer.objects.modified.start")
				for _, objectIdx := range layer.Changes.Modified {
					allChangesCount++
					modifyChangesCount++
					layerChangesCount++

					objectInfo := layer.Objects[objectIdx]

					//TODO: add a flag to select change type to apply path patterns
					match := objectInfo.PathMatch

					if !match && len(changePathMatchers) > 0 {
						log.Tracef("Change path patterns, no match. skipping 'modify' change ['%s']", objectInfo.Name)
						continue
					} else {
						if len(changeDataMatchers) > 0 {
							matchedPatterns, found := layer.DataMatches[objectInfo.Name]
							if !found {
								log.Tracef("Change data patterns, no match. skipping change ['%s']", objectInfo.Name)
								continue
							}

							log.Tracef("'%s' ('modify' change) matched data patterns - %d", objectInfo.Name, len(matchedPatterns))
							for _, cdm := range matchedPatterns {
								log.Tracef("matched => PP='%s' DP='%s'", cdm.PathPattern, cdm.DataPattern)
							}
						} else {
							if len(changeDataHashMatchers) > 0 {
								matched, found := layer.DataHashMatches[objectInfo.Name]
								if !found {
									log.Trace("Change data hash patterns, no match. skipping 'modify' change...")
									continue
								}

								log.Tracef("'%s' ('modify' change) matched data hash pattern - %s", objectInfo.Name, matched.Hash)
							}
						}
					}

					if allChangesMax > -1 && allChangesCount > allChangesMax {
						break
					}

					if modifyChangesMax > -1 && modifyChangesCount > modifyChangesMax {
						break
					}

					if layerChangesMax > -1 && layerChangesCount > layerChangesMax {
						break
					}

					if _, ok := changesOutputs["report"]; ok {
						layerReport.Modified = append(layerReport.Modified, objectInfo)
					}

					if _, ok := changesOutputs["console"]; ok {
						printObject(xc, objectInfo)
					}
				}
				xc.Out.Info("layer.objects.modified.end")
			}

			if _, ok := changes["add"]; ok && len(layer.Changes.Added) > 0 {
				xc.Out.Info("layer.objects.added.start")
				for _, objectIdx := range layer.Changes.Added {
					allChangesCount++
					addChangesCount++
					layerChangesCount++

					objectInfo := layer.Objects[objectIdx]

					//TODO: add a flag to select change type to apply path patterns
					match := objectInfo.PathMatch

					if !match && len(changePathMatchers) > 0 {
						log.Tracef("Change path patterns, no match. skipping 'add' change ['%s']", objectInfo.Name)
						continue
					} else {
						if len(changeDataMatchers) > 0 {
							matchedPatterns, found := layer.DataMatches[objectInfo.Name]
							if !found {
								log.Tracef("change data patterns, no match. skipping change ['%s']", objectInfo.Name)
								continue
							}

							log.Tracef("'%s' ('add' change) matched data patterns - %d", objectInfo.Name, len(matchedPatterns))
							for _, cdm := range matchedPatterns {
								log.Tracef("matched => PP='%s' DP='%s'", cdm.PathPattern, cdm.DataPattern)
							}
						} else {
							if len(changeDataHashMatchers) > 0 {
								matched, found := layer.DataHashMatches[objectInfo.Name]
								if !found {
									log.Trace("Change data hash patterns, no match. skipping 'add' change...")
									continue
								}

								log.Tracef("'%s' ('add' change) matched data hash pattern - %s", objectInfo.Name, matched.Hash)
							}
						}
					}

					if allChangesMax > -1 && allChangesCount > allChangesMax {
						break
					}

					if addChangesMax > -1 && addChangesCount > addChangesMax {
						break
					}

					if layerChangesMax > -1 && layerChangesCount > layerChangesMax {
						break
					}

					if _, ok := changesOutputs["report"]; ok {
						layerReport.Added = append(layerReport.Added, layer.Objects[objectIdx])
					}

					if _, ok := changesOutputs["console"]; ok {
						printObject(xc, layer.Objects[objectIdx])
					}
				}
				xc.Out.Info("layer.objects.added.end")
			}
		}

		if doShow(changeMatchLayersOnly, layer) {
			xc.Out.Info("layer.end")
		}
	}

	for _, info := range pkg.OSShells {
		xc.Out.Info("image.shells",
			ovars{
				"full_name":  info.FullName,
				"short_name": info.ShortName,
				"exe_path":   info.ExePath,
				"link_path":  info.LinkPath,
				"reference":  info.Reference,
				"verified":   info.Verified,
			})

		cmdReport.ImageReport.OSShells = append(cmdReport.ImageReport.OSShells, info)
	}

	xc.Out.Info("image.entry",
		ovars{
			"exe_path": cmdReport.SourceImage.ContainerEntry.ExePath,
			"exe_args": strings.Join(cmdReport.SourceImage.ContainerEntry.ExeArgs, ","),
		})

	if cmdReport.SourceImage.ContainerEntry.FullExePath != nil {
		xc.Out.Info("image.entry.full_exe_path",
			ovars{
				"name":  cmdReport.SourceImage.ContainerEntry.FullExePath.Name,
				"layer": cmdReport.SourceImage.ContainerEntry.FullExePath.Layer,
			})
	}

	if len(cmdReport.SourceImage.ContainerEntry.ArgFiles) > 0 {
		for _, argFile := range cmdReport.SourceImage.ContainerEntry.ArgFiles {
			xc.Out.Info("image.entry.arg_file",
				ovars{
					"name":  argFile.Name,
					"layer": argFile.Layer,
				})
		}
	}

	if doShowSpecialPerms &&
		(len(pkg.SpecialPermRefs.Setuid) > 0 ||
			len(pkg.SpecialPermRefs.Setgid) > 0 ||
			len(pkg.SpecialPermRefs.Sticky) > 0) {
		cmdReport.ImageReport.SpecialPerms = &dockerimage.SpecialPermsInfo{}

		for name := range pkg.SpecialPermRefs.Setuid {
			xc.Out.Info("image.special_perms.setuid",
				ovars{
					"name": name,
				})

			cmdReport.ImageReport.SpecialPerms.Setuid =
				append(cmdReport.ImageReport.SpecialPerms.Setuid, name)
		}

		for name := range pkg.SpecialPermRefs.Setgid {
			xc.Out.Info("image.special_perms.setgid",
				ovars{
					"name": name,
				})

			cmdReport.ImageReport.SpecialPerms.Setgid =
				append(cmdReport.ImageReport.SpecialPerms.Setgid, name)
		}

		for name := range pkg.SpecialPermRefs.Sticky {
			xc.Out.Info("image.special_perms.sticky",
				ovars{
					"name": name,
				})

			cmdReport.ImageReport.SpecialPerms.Sticky =
				append(cmdReport.ImageReport.SpecialPerms.Sticky, name)
		}
	}

	if doDetectDuplicates && len(pkg.HashReferences) > 0 {
		cmdReport.ImageReport.Duplicates = map[string]*dockerimage.DuplicateFilesReport{}

		//TODO: show duplicates by duplicate total size (biggest waste first)
		for hash, hobjects := range pkg.HashReferences {
			var dfr *dockerimage.DuplicateFilesReport
			showStart := true
			for fpath, info := range hobjects {
				if showStart {
					dfr = &dockerimage.DuplicateFilesReport{
						Files:       map[string]int{},
						FileCount:   uint64(len(hobjects)),
						FileSize:    uint64(info.Size),
						AllFileSize: uint64(info.Size * int64(len(hobjects))),
					}

					dfr.WastedSize = dfr.AllFileSize - dfr.FileSize
					cmdReport.ImageReport.Duplicates[hash] = dfr

					if doShowDuplicates {
						xc.Out.Info("image.duplicates.set.start",
							ovars{
								"hash":             hash,
								"count":            dfr.FileCount,
								"size.bytes":       dfr.FileSize,
								"size.human":       humanize.Bytes(uint64(dfr.FileSize)),
								"all_size.bytes":   dfr.AllFileSize,
								"size_total.human": humanize.Bytes(dfr.AllFileSize),
								"wasted.bytes":     dfr.WastedSize,
								"wasted.human":     humanize.Bytes(dfr.WastedSize),
							})
					}

					showStart = false
				}

				dfr.Files[fpath] = info.LayerIndex

				if doShowDuplicates {
					xc.Out.Info("image.duplicates.object",
						ovars{
							"name":  fpath,
							"layer": info.LayerIndex,
						})
				}
			}

			if doShowDuplicates {
				xc.Out.Info("image.duplicates.set.end")
			}
		}
	}
}

func objectHistoryString(history *dockerimage.ObjectHistory) string {
	if history == nil {
		return "H=[]"
	}

	var builder strings.Builder
	builder.WriteString("H=[")
	if history.Add != nil {
		builder.WriteString(fmt.Sprintf("A:%d", history.Add.Layer))
	}

	if history.Add != nil {
		var idxList []string
		for _, mod := range history.Modifies {
			idxList = append(idxList, fmt.Sprintf("%d", mod.Layer))
		}

		if len(idxList) > 0 {
			builder.WriteString(fmt.Sprintf("/M:%s", strings.Join(idxList, ",")))
		}
	}

	if history.Delete != nil {
		builder.WriteString(fmt.Sprintf("/D:%d", history.Delete.Layer))
	}

	builder.WriteString("]")
	return builder.String()
}

func printObject(xc *app.ExecutionContext, object *dockerimage.ObjectMetadata) {
	var hashInfo string

	if object.Hash != "" {
		hashInfo = fmt.Sprintf(" hash=%s", object.Hash)
	}
	ov := ovars{
		"mode":        object.Mode,
		"size.human":  humanize.Bytes(uint64(object.Size)),
		"size.bytes":  object.Size,
		"uid":         object.UID,
		"gid":         object.GID,
		"mtime":       object.ModTime.UTC().Format(time.RFC3339),
		"H":           objectHistoryString(object.History),
		"hash":        hashInfo,
		"object.name": object.Name,
	}

	if object.LinkTarget != "" {
		ov["link.target"] = object.LinkTarget
	}

	xc.Out.Info("object", ov)

}
