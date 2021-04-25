package xray

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/pkg/app/master/inspectors/image"
	"github.com/docker-slim/docker-slim/pkg/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/command"
	"github.com/docker-slim/docker-slim/pkg/docker/buildpackinfo"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerimage"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerutil"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"

	"github.com/bmatcuk/doublestar/v3"
	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"
)

const appName = commands.AppName

type ovars = commands.OutVars

// Xray command exit codes
const (
	ecxOther = iota + 1
	ecxImageNotFound
)

// OnCommand implements the 'xray' docker-slim command
func OnCommand(
	xc *commands.ExecutionContext,
	gparams *commands.GenericParams,
	targetRef string,
	doPull bool,
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
	doAddImageManifest bool,
	doAddImageConfig bool,
	doReuseSavedImage bool,
	doRmFileArtifacts bool) {
	const cmdName = Name
	logger := log.WithFields(log.Fields{"app": appName, "command": cmdName})
	prefix := fmt.Sprintf("cmd=%s", cmdName)

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
	cmdReport.State = command.StateStarted
	cmdReport.TargetReference = targetRef

	xc.Out.State("started")
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

		exitCode := commands.ECTCommon | commands.ECNoDockerConnectInfo
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
		version.Print(prefix, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	imageInspector, err := image.NewInspector(client, targetRef)
	errutil.FailOn(err)

	if imageInspector.NoImage() {
		if doPull {
			xc.Out.Info("target.image",
				ovars{
					"status":  "not.found",
					"image":   targetRef,
					"message": "trying to pull target image",
				})

			err := imageInspector.Pull(doShowPullLogs)
			errutil.FailOn(err)
		} else {
			xc.Out.Error("image.not.found", "make sure the target image already exists locally")

			exitCode := commands.ECTBuild | ecxImageNotFound
			xc.Out.State("exited",
				ovars{
					"exit.code": exitCode,
				})
			xc.Exit(exitCode)
		}
	}

	xc.Out.State("image.api.inspection.start")

	logger.Info("inspecting 'fat' image metadata...")
	err = imageInspector.Inspect()
	errutil.FailOn(err)

	localVolumePath, artifactLocation, statePath, stateKey := fsutil.PrepareImageStateDirs(gparams.StatePath, imageInspector.ImageInfo.ID)
	imageInspector.ArtifactLocation = artifactLocation
	logger.Debugf("localVolumePath=%v, artifactLocation=%v, statePath=%v, stateKey=%v", localVolumePath, artifactLocation, statePath, stateKey)

	xc.Out.Info("image",
		ovars{
			"id":         imageInspector.ImageInfo.ID,
			"size.bytes": imageInspector.ImageInfo.VirtualSize,
			"size.human": humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)),
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
	}

	for k := range imageInspector.ImageInfo.Config.ExposedPorts {
		cmdReport.SourceImage.ExposedPorts = append(cmdReport.SourceImage.ExposedPorts, string(k))
	}

	for k := range imageInspector.ImageInfo.Config.Volumes {
		cmdReport.SourceImage.Volumes = append(cmdReport.SourceImage.Volumes, k)
	}

	cmdReport.SourceImage.Labels = imageInspector.ImageInfo.Config.Labels

	if buildpackinfo.HasBuildbackLabels(imageInspector.ImageInfo.Config.Labels) {
		bpStack := imageInspector.ImageInfo.Config.Labels[buildpackinfo.LabelKeyStackID]
		cmdReport.SourceImage.Buildpack = &report.BuildpackInfo{
			Stack: bpStack,
		}
	}

	cmdReport.SourceImage.EnvVars = imageInspector.ImageInfo.Config.Env

	cmdReport.ArtifactLocation = imageInspector.ArtifactLocation

	xc.Out.State("image.api.inspection.done")
	xc.Out.State("image.data.inspection.start")

	imageID := dockerutil.CleanImageID(imageInspector.ImageInfo.ID)
	iaName := fmt.Sprintf("%s.tar", imageID)
	iaPath := filepath.Join(localVolumePath, "image", iaName)

	var doSave bool
	if fsutil.IsRegularFile(iaPath) {
		if !doReuseSavedImage {
			doSave = true
		}
	} else {
		doSave = true
	}

	if doSave {
		xc.Out.Info("image.data.inspection.save.image.start")
		err = dockerutil.SaveImage(client, imageID, iaPath, false, false)
		errutil.FailOn(err)
		xc.Out.Info("image.data.inspection.save.image.end")
	} else {
		logger.Debugf("exported image already exists - %s", iaPath)
	}

	xc.Out.Info("image.data.inspection.process.image.start")
	imagePkg, err := dockerimage.LoadPackage(iaPath, imageID, false, topChangesMax, doHashData, changeDataHashMatchers, changePathMatchers, changeDataMatchers)
	errutil.FailOn(err)
	xc.Out.Info("image.data.inspection.process.image.end")

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
		changeDataHashMatchers,
		changePathMatchers,
		changeDataMatchers,
		cmdReport)

	if doAddImageManifest {
		cmdReport.RawImageManifest = imagePkg.Manifest
	}

	if doAddImageConfig {
		cmdReport.RawImageConfig = imagePkg.Config
	}

	xc.Out.State("completed")
	cmdReport.State = command.StateCompleted

	if doRmFileArtifacts {
		logger.Info("removing temporary artifacts...")
		err = fsutil.Remove(iaPath)
		errutil.WarnOn(err)
	} else {
		cmdReport.ImageArchiveLocation = iaPath
	}

	xc.Out.State("done")

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

	cmdReport.State = command.StateDone
	if cmdReport.Save() {
		xc.Out.Info("report",
			ovars{
				"file": cmdReport.ReportLocation(),
			})
	}
}

func printImagePackage(
	xc *commands.ExecutionContext,
	pkg *dockerimage.Package,
	appName string,
	cmdName command.Type,
	changes map[string]struct{},
	changesOutputs map[string]struct{},
	layers map[string]struct{},
	layerChangesMax int,
	allChangesMax int,
	addChangesMax int,
	modifyChangesMax int,
	deleteChangesMax int,
	doHashData bool,
	changeDataHashMatchers map[string]*dockerimage.ChangeDataHashMatcher,
	changePathMatchers []*dockerimage.ChangePathMatcher,
	changeDataMatchers map[string]*dockerimage.ChangeDataMatcher,
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

		xc.Out.Info("layer.start")
		xc.Out.Info("layer", layerInfo)

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
			xc.Out.Info("change.instruction",
				ovars{
					"index":   fmt.Sprintf("%d:%d", layerReport.ChangeInstruction.ImageIndex, layerReport.ChangeInstruction.Index),
					"type":    layerReport.ChangeInstruction.Type,
					"snippet": layerReport.ChangeInstruction.Snippet,
					"all":     layerReport.ChangeInstruction.All,
				})

		}

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

		changeCount := len(layer.Changes.Deleted) + len(layer.Changes.Modified) + len(layer.Changes.Added)

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
				var match bool
				for _, pm := range changePathMatchers {
					ptrn := strings.TrimSpace(pm.PathPattern)
					if len(ptrn) == 0 {
						continue
					}

					var err error
					match, err = doublestar.Match(ptrn, topObject.Name)
					if err != nil {
						log.Errorf("doublestar.Match name='%s' error=%v", topObject.Name, err)
					}

					if match {
						log.Tracef("Change path patterns match for 'top'. ptrn='%s' object.Name='%s'\n", ptrn, topObject.Name)
						break
						//not collecting all file path matches here
					}
				}

				if !match && len(changePathMatchers) > 0 {
					log.Trace("Change path patterns, no match. skipping 'top' change...")
					continue
				} else {
					if len(changeDataMatchers) > 0 {
						matchedPatterns, found := layer.DataMatches[topObject.Name]
						if !found {
							log.Trace("Change data patterns, no match. skipping 'top' change...")
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

		if showLayer {
			if _, ok := changes["delete"]; ok && len(layer.Changes.Deleted) > 0 {
				xc.Out.Info("layer.objects.deleted.start")
				for _, objectIdx := range layer.Changes.Deleted {
					allChangesCount++
					deleteChangesCount++
					layerChangesCount++

					objectInfo := layer.Objects[objectIdx]

					//TODO: add a flag to select change type to apply path patterns
					var match bool
					for _, pm := range changePathMatchers {
						ptrn := strings.TrimSpace(pm.PathPattern)
						if len(ptrn) == 0 {
							continue
						}

						var err error
						match, err = doublestar.Match(ptrn, objectInfo.Name)
						if err != nil {
							log.Errorf("doublestar.Match name='%s' error=%v", objectInfo.Name, err)
						}

						if match {
							log.Trace("Change path patterns match for 'delete'. ptrn='%s' objectInfo.Name='%s'\n", ptrn, objectInfo.Name)
							break
							//not collecting all file path matches here
						}
					}

					if !match && len(changePathMatchers) > 0 {
						log.Trace("Change path patterns, no match. skipping 'delete' change...")
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
					var match bool
					for _, pm := range changePathMatchers {
						ptrn := strings.TrimSpace(pm.PathPattern)
						if len(ptrn) == 0 {
							continue
						}

						var err error
						match, err = doublestar.Match(ptrn, objectInfo.Name)
						if err != nil {
							log.Errorf("doublestar.Match name='%s' error=%v", objectInfo.Name, err)
						}

						if match {
							log.Trace("Change path patterns match for 'modify'. ptrn='%s' objectInfo.Name='%s'\n", ptrn, objectInfo.Name)
							break
							//not collecting all file path matches here
						}
					}

					if !match && len(changePathMatchers) > 0 {
						log.Trace("Change path patterns, no match. skipping 'modify' change...")
						continue
					} else {
						if len(changeDataMatchers) > 0 {
							matchedPatterns, found := layer.DataMatches[objectInfo.Name]
							if !found {
								log.Trace("Change data patterns, no match. skipping change...")
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
					var match bool
					for _, pm := range changePathMatchers {
						ptrn := strings.TrimSpace(pm.PathPattern)
						if len(ptrn) == 0 {
							continue
						}

						var err error
						match, err = doublestar.Match(ptrn, objectInfo.Name)
						if err != nil {
							log.Errorf("doublestar.Match name='%s' error=%v", objectInfo.Name, err)
						}

						if match {
							log.Trace("Change path patterns match for 'add'. ptrn='%s' objectInfo.Name='%s'\n", ptrn, objectInfo.Name)
							break
							//not collecting all file path matches here
						}
					}

					if !match && len(changePathMatchers) > 0 {
						log.Trace("Change path patterns, no match. skipping 'add' change...")
						continue
					} else {
						if len(changeDataMatchers) > 0 {
							matchedPatterns, found := layer.DataMatches[objectInfo.Name]
							if !found {
								log.Trace("change data patterns, no match. skipping change...")
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

		xc.Out.Info("layer.end")
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

func printObject(xc *commands.ExecutionContext, object *dockerimage.ObjectMetadata) {
	var hashInfo string
	if object.Hash != "" {
		hashInfo = fmt.Sprintf(" hash=%s", object.Hash)
	}

	fmt.Printf("%s: mode=%s size.human='%v' size.bytes=%d uid=%d gid=%d mtime='%s' %s%s '%s'",
		object.Change,
		object.Mode,
		humanize.Bytes(uint64(object.Size)),
		object.Size,
		object.UID,
		object.GID,
		object.ModTime.UTC().Format(time.RFC3339),
		objectHistoryString(object.History),
		hashInfo,
		object.Name)

	if object.LinkTarget != "" {
		fmt.Printf(" -> '%s'\n", object.LinkTarget)
	} else {
		fmt.Printf("\n")
	}
}
