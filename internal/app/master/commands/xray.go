package commands

import (
	"fmt"
	"os"
	"path/filepath"

	//"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/internal/app/master/inspectors/image"
	"github.com/docker-slim/docker-slim/internal/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerimage"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/dockerutil"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"

	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"
)

// Xray command exit codes
const (
	ecxOther = iota + 1
)

// OnXray implements the 'xray' docker-slim command
func OnXray(
	gparams *GenericParams,
	targetRef string,
	ec *ExecutionContext) {
	const cmdName = "xray"
	logger := log.WithFields(log.Fields{"app": appName, "command": cmdName})
	prefix := fmt.Sprintf("%s[%s]:", appName, cmdName)

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewXrayCommand(gparams.ReportLocation)
	cmdReport.State = report.CmdStateStarted
	cmdReport.OriginalImage = targetRef

	fmt.Printf("%s[%s]: state=started\n", appName, cmdName)
	fmt.Printf("%s[%s]: info=params target=%v\n", appName, cmdName, targetRef)

	client, err := dockerclient.New(gparams.ClientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if gparams.InContainer && gparams.IsDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
		}
		fmt.Printf("%s[%s]: info=docker.connect.error message='%s'\n", appName, cmdName, exitMsg)
		fmt.Printf("%s[%s]: state=exited version=%s location='%s'\n", appName, cmdName, v.Current(), fsutil.ExeDir())
		os.Exit(ectCommon | ecNoDockerConnectInfo)
	}
	errutil.FailOn(err)

	if gparams.Debug {
		version.Print(prefix, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	imageInspector, err := image.NewInspector(client, targetRef)
	errutil.FailOn(err)

	if imageInspector.NoImage() {
		fmt.Printf("%s[%s]: info=target.image.error status=not.found image='%v' message='make sure the target image already exists locally'\n", appName, cmdName, targetRef)
		fmt.Printf("%s[%s]: state=exited\n", appName, cmdName)
		return
	}

	logger.Info("inspecting 'fat' image metadata...")
	err = imageInspector.Inspect()
	errutil.FailOn(err)

	localVolumePath, artifactLocation, statePath, stateKey := fsutil.PrepareImageStateDirs(gparams.StatePath, imageInspector.ImageInfo.ID)
	imageInspector.ArtifactLocation = artifactLocation
	logger.Debugf("localVolumePath=%v, artifactLocation=%v, statePath=%v, stateKey=%v", localVolumePath, artifactLocation, statePath, stateKey)

	fmt.Printf("%s[%s]: info=image id=%v size.bytes=%v size.human=%v\n",
		appName, cmdName,
		imageInspector.ImageInfo.ID,
		imageInspector.ImageInfo.VirtualSize,
		humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)))

	logger.Info("processing 'fat' image info...")
	err = imageInspector.ProcessCollectedData()
	errutil.FailOn(err)

	imageID := dockerutil.CleanImageID(imageInspector.ImageInfo.ID)
	iaName := fmt.Sprintf("%s.tar", imageID)
	iaPath := filepath.Join(localVolumePath, "image", iaName)
	err = dockerutil.SaveImage(client, imageID, iaPath, false, false)
	errutil.FailOn(err)

	imagePkg, err := dockerimage.LoadPackage(iaPath, imageID, false)
	errutil.FailOn(err)

	printImagePackage(imagePkg, appName, cmdName)

	fmt.Printf("%s[%s]: state=completed\n", appName, cmdName)
	cmdReport.State = report.CmdStateCompleted

	fmt.Printf("%s[%s]: state=done\n", appName, cmdName)

	vinfo := <-viChan
	version.PrintCheckVersion(prefix, vinfo)

	cmdReport.State = report.CmdStateDone
	if cmdReport.Save() {
		fmt.Printf("%s[%s]: info=report file='%s'\n", appName, cmdName, cmdReport.ReportLocation())
	}
}

func printImagePackage(pkg *dockerimage.Package, appName, cmdName string) {
	fmt.Printf("%s[%s]: info=image.package.details\n", appName, cmdName)

	fmt.Printf("%s[%s]: info=layers.count: %v\n", appName, cmdName, len(pkg.Layers))
	for _, layer := range pkg.Layers {
		fmt.Printf("%s[%s]: info=layer index=%d id=%s path=%s\n", appName, cmdName, layer.Index, layer.ID, layer.Path)
		fmt.Printf("%s[%s]: info=layer.stats data=%#v\n", appName, cmdName, layer.Stats)
		changeCount := len(layer.Changes.Deleted) + len(layer.Changes.Modified) + len(layer.Changes.Added)

		fmt.Printf("%s[%s]: info=layer.change.summary deleted=%d modified=%d added=%d all=%d\n",
			appName, cmdName, len(layer.Changes.Deleted), len(layer.Changes.Modified),
			len(layer.Changes.Added), changeCount)

		fmt.Printf("%s[%s]: info=layer.objects.count data=%d\n", appName, cmdName, len(layer.Objects))

		topList := layer.Top.List()
		fmt.Printf("%s[%s]: info=layer.objects.top:\n", appName, cmdName)
		for _, object := range topList {
			printObject(object)
		}
		fmt.Printf("\n")

		fmt.Printf("%s[%s]: info=layer.objects.deleted:\n", appName, cmdName)
		for _, objectIdx := range layer.Changes.Deleted {
			printObject(layer.Objects[objectIdx])
		}
		fmt.Printf("\n")
		fmt.Printf("%s[%s]: info=layer.objects.modified:\n", appName, cmdName)
		for _, objectIdx := range layer.Changes.Modified {
			printObject(layer.Objects[objectIdx])
		}
		fmt.Printf("\n")
		fmt.Printf("%s[%s]: info=layer.objects.added:\n", appName, cmdName)
		for _, objectIdx := range layer.Changes.Added {
			printObject(layer.Objects[objectIdx])
		}
	}
}

func printObject(object *dockerimage.ObjectMetadata) {
	fmt.Printf("O: change=%d mode=%s size=%v(%d) uid=%d gid=%d mtime=%s %s",
		object.Change, object.Mode, humanize.Bytes(uint64(object.Size)), object.Size, object.UID, object.GID,
		object.ModTime, object.Name)

	if object.LinkTarget != "" {
		fmt.Printf(" -> %s\n", object.LinkTarget)
	} else {
		fmt.Printf("\n")
	}
}
