package merge

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cespare/xxhash/v2"
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/app/master/inspectors/image"
	"github.com/slimtoolkit/slim/pkg/app/master/version"
	cmd "github.com/slimtoolkit/slim/pkg/command"
	"github.com/slimtoolkit/slim/pkg/docker/dockerclient"
	"github.com/slimtoolkit/slim/pkg/imagebuilder"
	"github.com/slimtoolkit/slim/pkg/imagebuilder/internalbuilder"
	"github.com/slimtoolkit/slim/pkg/imagereader"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
	v "github.com/slimtoolkit/slim/pkg/version"
)

const appName = command.AppName

type ovars = app.OutVars

// OnCommand implements the 'merge' command
func OnCommand(
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	cparams *CommandParams) {
	const cmdName = Name
	logger := log.WithFields(log.Fields{"app": appName, "cmd": cmdName})

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewMergeCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = cmd.StateStarted
	cmdReport.FirstImage = cparams.FirstImage
	cmdReport.LastImage = cparams.LastImage
	cmdReport.UseLastImageMetadata = cparams.UseLastImageMetadata

	xc.Out.State("started")
	xc.Out.Info("params",
		ovars{
			"image.first":             cparams.FirstImage,
			"image.last":              cparams.LastImage,
			"use.last.image.metadata": cparams.UseLastImageMetadata,
			"output.tags":             cparams.OutputTags,
		})

	client, err := dockerclient.New(gparams.ClientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if gparams.InContainer && gparams.IsDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the slim app container"
		}

		xc.Out.Info("docker.connect.error",
			ovars{
				"message": exitMsg,
			})

		exitCode := command.ECTCommon | command.ECCNoDockerConnectInfo
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})
		xc.Exit(exitCode)
	}
	xc.FailOn(err)

	if gparams.Debug {
		version.Print(xc, cmdName, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	//////////////////////////////////////////////////
	ensureImage := func(name string, imageRef string, cr *report.MergeCommand) string {
		imageInspector, err := image.NewInspector(client, imageRef)
		xc.FailOn(err)

		noImage, err := imageInspector.NoImage()
		errutil.FailOn(err)
		if noImage {
			xc.Out.Error(fmt.Sprintf("%s.image.not.found", name), "make sure the target image already exists locally")

			cmdReport.State = cmd.StateError
			exitCode := command.ECTCommon | command.ECCImageNotFound
			xc.Out.State("exited",
				ovars{
					"exit.code": exitCode,
				})
			xc.Exit(exitCode)
		}

		return imageInspector.ImageRef
	}

	//and refresh the image refs
	cparams.FirstImage = ensureImage("first", cmdReport.FirstImage, cmdReport)
	cmdReport.FirstImage = cparams.FirstImage

	//and refresh the image refs
	cparams.LastImage = ensureImage("last", cmdReport.LastImage, cmdReport)
	cmdReport.LastImage = cparams.LastImage

	outputTags := cparams.OutputTags
	if len(outputTags) == 0 {
		var outputName string
		if strings.Contains(cparams.LastImage, ":") {
			parts := strings.SplitN(cparams.LastImage, ":", 2)
			outputName = fmt.Sprintf("%s.merged:%s", parts[0], parts[1])
		} else {
			outputName = fmt.Sprintf("%s.merged", cparams.LastImage)
		}
		outputTags = append(outputTags, outputName)
	}

	fiReader, err := imagereader.New(cparams.FirstImage)
	xc.FailOn(err)
	liReader, err := imagereader.New(cparams.LastImage)
	xc.FailOn(err)

	xc.Out.State("image.metadata.merge.start")
	fiImageConfig, err := fiReader.ImageConfig()
	xc.FailOn(err)
	liImageConfig, err := liReader.ImageConfig()
	xc.FailOn(err)

	var outImageConfig *imagebuilder.ImageConfig
	if cparams.UseLastImageMetadata {
		outImageConfig = liImageConfig
	} else {
		imageConfig := *liImageConfig

		//merge environment variables (todo: do a better job merging envs, need to parse k/v)
		envMap := map[string]struct{}{}
		for _, v := range fiImageConfig.Config.Env {
			envMap[v] = struct{}{}
		}
		for _, v := range liImageConfig.Config.Env {
			envMap[v] = struct{}{}
		}

		imageConfig.Config.Env = []string{}
		for k := range envMap {
			imageConfig.Config.Env = append(imageConfig.Config.Env, k)
		}

		//merge labels
		labelMap := map[string]string{}
		for k, v := range fiImageConfig.Config.Labels {
			labelMap[k] = v
		}
		for k, v := range liImageConfig.Config.Labels {
			labelMap[k] = v
		}

		imageConfig.Config.Labels = labelMap

		//merge exposed ports
		portMap := map[string]struct{}{}
		for k := range fiImageConfig.Config.ExposedPorts {
			portMap[k] = struct{}{}
		}
		for k := range liImageConfig.Config.ExposedPorts {
			portMap[k] = struct{}{}
		}

		imageConfig.Config.ExposedPorts = portMap

		//merge volumes
		volumeMap := map[string]struct{}{}
		for k := range fiImageConfig.Config.Volumes {
			volumeMap[k] = struct{}{}
		}
		for k := range liImageConfig.Config.Volumes {
			volumeMap[k] = struct{}{}
		}

		imageConfig.Config.Volumes = volumeMap

		//Merging OnBuild requires the instruction order to be preserved
		//Auto-merging OnBuild instructions is not always ideal because
		//of the potential side effects if the merged images are not very compatible.
		//Merging minified images of the same source image should have no side effects
		//because the OnBuild instructions will be identical.
		sameLists := func(first, second []string) bool {
			if len(first) != len(second) {
				return false
			}

			for idx := range first {
				if first[idx] != second[idx] {
					return false
				}
			}

			return true
		}

		if !sameLists(fiImageConfig.Config.OnBuild, liImageConfig.Config.OnBuild) {
			var onBuild []string
			onBuild = append(onBuild, fiImageConfig.Config.OnBuild...)
			onBuild = append(onBuild, liImageConfig.Config.OnBuild...)
			imageConfig.Config.OnBuild = onBuild
		}

		outImageConfig = &imageConfig
	}

	xc.Out.State("image.metadata.merge.done")
	xc.Out.State("image.data.merge.start")

	fiDataTarName, err := fiReader.ExportFilesystem()
	xc.FailOn(err)

	liDataTarName, err := liReader.ExportFilesystem()
	xc.FailOn(err)

	f1, err := os.Open(fiDataTarName)
	xc.FailOn(err)
	defer f1.Close()

	index, err := tarMapFromFile(f1)
	xc.FailOn(err)

	f2, err := os.Open(liDataTarName)
	xc.FailOn(err)
	defer f2.Close()

	index2, err := tarMapFromFile(f2)
	xc.FailOn(err)

	logger.Debug("updating tar map with first tar data...")
	for p, info := range index2 {
		other, found := index[p]
		if !found {
			index[p] = info
			continue
		}

		if info.Header.Typeflag == other.Header.Typeflag &&
			info.Header.Size == other.Header.Size &&
			info.Hash == other.Hash {
			//can/should also check info.Header.Mode and info.Header.ModTime
			//if info.Header.ModTime.After(other.Header.ModTime) {
			//	info.Replaced = append(other.Replaced, other)
			//	index[p] = info
			//	continue
			//}

			other.Dups++
			continue
		}

		info.Replaced = append(other.Replaced, other)
		index[p] = info
	}

	outTarFileName, err := tarFromMap(logger, "", index)

	if !fsutil.Exists(outTarFileName) ||
		!fsutil.IsRegularFile(outTarFileName) ||
		!fsutil.IsTarFile(outTarFileName) {
		xc.FailOn(fmt.Errorf("bad output tar - %s", outTarFileName))
	}

	xc.Out.State("image.data.merge.done")
	xc.Out.State("output.image.generate.start")

	ibo, err := imagebuilder.SimpleBuildOptionsFromImageConfig(outImageConfig)
	xc.FailOn(err)

	ibo.Tags = outputTags

	layerInfo := imagebuilder.LayerDataInfo{
		Type:   imagebuilder.TarSource,
		Source: outTarFileName,
		Params: &imagebuilder.DataParams{
			TargetPath: "/",
		},
	}

	ibo.Layers = append(ibo.Layers, layerInfo)

	engine, err := internalbuilder.New(
		false, //show build logs doShowBuildLogs,
		true,  //push to daemon - TODO: have a param to control this later
		//output image tar (if not 'saving' to daemon)
		false)
	xc.FailOn(err)

	imageResult, err := engine.Build(*ibo)
	xc.FailOn(err)

	xc.Out.Info("results.output",
		ovars{
			"image.name":   imageResult.Name,
			"image.id":     imageResult.ID,
			"image.digest": imageResult.Digest,
		})

	ensureImage("output", outputTags[0], cmdReport)
	xc.Out.State("output.image.generate.done")
	//////////////////////////////////////////////////

	xc.Out.State(cmd.StateCompleted)
	cmdReport.State = cmd.StateCompleted
	xc.Out.State(cmd.StateDone)

	vinfo := <-viChan
	version.PrintCheckVersion(xc, "", vinfo)

	cmdReport.State = cmd.StateDone
	if cmdReport.Save() {
		xc.Out.Info("report",
			ovars{
				"file": cmdReport.ReportLocation(),
			})
	}
}

type tfInfo struct {
	FileIndex  uint32
	Header     *tar.Header
	Hash       uint64
	File       *os.File
	DataOffset int64
	Dups       uint32 //to count duplicates (can have extra field to track tar file metadata later)
	Replaced   []*tfInfo
}

func tarMapFromFile(f *os.File) (map[string]*tfInfo, error) {
	tr := tar.NewReader(f)
	tarMap := map[string]*tfInfo{}

	var fileIndex uint32
	for {
		th, err := tr.Next()

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			fmt.Println(err)
			return tarMap, err
		}

		if th == nil {
			fmt.Println("skipping empty tar header...")
			continue
		}

		offset, err := f.Seek(0, os.SEEK_CUR)
		if err != nil {
			fmt.Println(err)
			return tarMap, err
		}

		sr := io.NewSectionReader(f, offset, th.Size)

		hash := xxhash.New()
		//if _, err := io.Copy(hash, tr); err != nil {
		if _, err := io.Copy(hash, sr); err != nil {
			//_, err = io.CopyN(hash, sr, th.Size)
			log.Fatalf("Failed to compute hash: %v", err)
		}
		hashValue := hash.Sum64()

		//NOTE:
		//Not exposing the archived file data right now
		//because it'll require to read/load the data into memory
		//and for big images it'll be a lot of data.
		//For now just re-read the data when needed.

		tarMap[th.Name] = &tfInfo{
			FileIndex:  fileIndex,
			Header:     th,
			Hash:       hashValue,
			File:       f,      //tar file ref (not the file inside tar)
			DataOffset: offset, //offset in tar file
		}

		fileIndex++
	}

	return tarMap, nil
}

func tarFromMap(logger *log.Entry, outputPath string, tarMap map[string]*tfInfo) (string, error) {
	var out *os.File

	if outputPath == "" {
		tarFile, err := os.CreateTemp("", "image-output-*.tar")
		if err != nil {
			return "", err
		}

		out = tarFile
	} else {
		tarFile, err := os.Create(outputPath)
		if err != nil {
			return "", err
		}

		out = tarFile
	}

	defer out.Close()

	// Create a new tar archive
	tw := tar.NewWriter(out)
	defer tw.Close()

	// Iterate over the input files
	for filePath, info := range tarMap {
		logger.Tracef("%s -> %+v\n", filePath, info)

		if err := tw.WriteHeader(info.Header); err != nil {
			panic(err)
		}

		if info.Header.Size == 0 {
			continue
		}

		if info.DataOffset < 0 {
			continue
		}

		sr := io.NewSectionReader(info.File, info.DataOffset, info.Header.Size)
		if _, err := io.Copy(tw, sr); err != nil {
			return "", err
		}
	}

	return out.Name(), nil
}
