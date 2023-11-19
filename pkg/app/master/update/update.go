package update

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	vchecker "github.com/slimtoolkit/slim/pkg/app/master/version"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
	vinfo "github.com/slimtoolkit/slim/pkg/version"

	"github.com/c4milo/unpackit"
	log "github.com/sirupsen/logrus"
	"github.com/slimtoolkit/go-update"
	"github.com/slimtoolkit/uiprogress"
)

const (
	hdrUserAgent     = "User-Agent"
	hdrContentLength = "Content-Length"
	downloadEndpoint = "https://downloads.dockerslim.com/releases"
	masterAppName    = "slim"
	sensorAppName    = "slim-sensor"
	distDirName      = "dist"
	artifactsPerms   = 0740
)

var (
	errUnexpectedHTTPStatus = errors.New("unexpected HTTP status code")
)

// Run checks the current version and updates it if it doesn't match the latest available version
func Run(doDebug bool, statePath string, inContainer, isDSImage, doShowProgress bool) {
	logger := log.WithFields(log.Fields{"app": "slim", "command": "update"})

	appPath, err := os.Executable()
	errutil.FailOn(err)
	appDirPath := filepath.Dir(appPath)

	vstatus := vchecker.Check(inContainer, isDSImage)
	logger.Debugf("Version Status => %+v", vstatus)

	if vstatus == nil || vstatus.Status != "success" {
		fmt.Printf("slim[update]: info=status message='version check was not successful'\n")
		fmt.Printf("slim[update]: state=exited version=%s\n", vinfo.Current())
		return
	}

	if !vstatus.Outdated {
		fmt.Printf("slim[update]: info=status message='already using the current version'\n")
		fmt.Printf("slim[update]: state=exited version=%s\n", vinfo.Current())
		return
	}

	fmt.Printf("slim[update]: info=version local=%s current=%s\n", vinfo.Tag(), vstatus.Current)

	blobNameBase, blobNameExt := getReleaseBlobInfo()
	errutil.FailWhen(blobNameBase == "", "could not discover platform-specific release package name")

	releaseDirPath, statePath := fsutil.PrepareReleaseStateDirs(statePath, vstatus.Current)
	errutil.FailOn(err)

	blobName := fmt.Sprintf("%s.%s", blobNameBase, blobNameExt)
	blobPath := filepath.Join(releaseDirPath, blobName)
	logger.Debugf("release package blob: %v", blobPath)

	if fsutil.Exists(blobPath) {
		//feature: not removing/replacing the existing release package blob if it's already there
		fmt.Printf("slim[update]: info=status message='release package already downloaded'\n")
		fmt.Printf("slim[update]: state=exited version=%s\n", vinfo.Current())
		return
	}

	fmt.Println("slim[update]: state=update.download.started")

	releaseDownloadPath := fmt.Sprintf("%s/%s/%s", downloadEndpoint, vstatus.Current, blobName)
	logger.Debugf("release download path: %v", releaseDownloadPath)

	if !isGoodDownloadSource(logger, releaseDownloadPath) {
		fmt.Printf("slim[update]: info=status message='release package download location is not accessible'\n")
		fmt.Printf("slim[update]: state=exited version=%s\n", vinfo.Current())
		return
	}

	var brConstructor BlobReaderConstructor = newPassThroughReader
	if doShowProgress {
		logger.Debug("show download progress")
		brConstructor = newProgressReader
	}

	err = downloadRelease(logger, blobPath, releaseDownloadPath, brConstructor)
	if err != nil {
		logger.Debugf("error downloading release: %v", err)
		fmt.Printf("slim[update]: info=status message='error downloading release package'\n")
		fmt.Printf("slim[update]: state=exited version=%s\n", vinfo.Current())
		return
	}

	fmt.Println("slim[update]: state=update.download.completed")

	if err := unpackRelease(logger, blobPath, releaseDirPath, blobNameBase); err != nil {
		logger.Debugf("error unpacking release package: %v", err)
		fmt.Printf("slim[update]: info=status message='error unpacking release package'\n")
		fmt.Printf("slim[update]: state=exited version=%s\n", vinfo.Current())
		return
	}

	fmt.Println("slim[update]: state=update.unpacked")

	if err := installRelease(logger, appDirPath, statePath, releaseDirPath); err != nil {
		logger.Debugf("error installing release: %v", err)
		fmt.Printf("slim[update]: info=status message='error installing release'\n")
		fmt.Printf("slim[update]: state=exited version=%s\n", vinfo.Current())
		return
	}

	fmt.Println("slim[update]: state=update.installed")
	fmt.Printf("slim[update]: state=exited version=%s\n", vinfo.Current())
}

func getReleaseBlobInfo() (base string, ext string) {
	switch runtime.GOOS {
	case "darwin":
		return "dist_mac", "zip"
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "dist_linux", "tar.gz"
		case "arm":
			return "dist_linux_arm", "tar.gz"
		case "arm64":
			return "dist_linux_arm64", "tar.gz"
		default:
			return "", ""
		}
	default:
		return "", ""
	}
}

func downloadRelease(logger *log.Entry, localBlobPath, downloadSource string, c BlobReaderConstructor) error {
	d, err := os.Create(localBlobPath)
	if err != nil {
		return err
	}

	client := http.Client{
		Timeout: 13 * time.Second,
	}

	req, err := http.NewRequest("GET", downloadSource, nil)
	if err != nil {
		panic(err)
	}

	req.Header.Set(hdrUserAgent, fmt.Sprintf("DockerSlimApp/%s", vinfo.Current()))

	qs := url.Values{}
	qs.Add("cv", vinfo.Tag())
	req.URL.RawQuery = qs.Encode()

	resp, err := client.Do(req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}

		d.Close()
		os.Remove(localBlobPath)
		return err
	}

	if resp.StatusCode != http.StatusOK {
		logger.Debugf("downloadRelease: unexpected status code - %v", resp.StatusCode)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		d.Close()
		os.Remove(localBlobPath)
		return errUnexpectedHTTPStatus
	}

	if resp.ContentLength == -1 {
		logger.Debug("downloadRelease: content length is unknown")
	}
	blobSize := int(resp.ContentLength)
	blobReader := c(blobSize, resp.Body)

	copied, err := io.Copy(d, blobReader)
	if err != nil {
		blobReader.Close()
		d.Close()
		os.Remove(localBlobPath)
		return err
	}

	logger.Debugf("downloadRelease: data size = %v (content length = %v)", copied, blobSize)

	if err := blobReader.Close(); err != nil {
		logger.Debugf("downloadRelease: error closing downloaded reader - %v", err)
	}

	d.Chmod(artifactsPerms)

	if err := d.Close(); err != nil {
		logger.Debugf("downloadRelease: error closing downloaded release package - %v", err)
	}

	return nil
}

func isGoodDownloadSource(logger *log.Entry, location string) bool {
	client := http.Client{
		Timeout: 13 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest("HEAD", location, nil)
	if err != nil {
		panic(err)
	}

	req.Header.Set(hdrUserAgent, fmt.Sprintf("DockerSlimApp/%s", vinfo.Current()))

	resp, err := client.Do(req)
	if resp != nil && resp.Body != nil {
		defer func() {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}()
	}

	if err != nil {
		logger.Debugf("isGoodDownloadSource: error - %v", err)
		return false
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return true
	}

	return false
}

func unpackRelease(logger *log.Entry, blobPath, releaseRootPath, blobNameBase string) error {
	unpackedBlobDir := filepath.Join(releaseRootPath, blobNameBase)
	commonBlobDir := filepath.Join(releaseRootPath, distDirName)

	if fsutil.DirExists(unpackedBlobDir) {
		//feature: not removing the unpacked directory if it's already there (todo: revisit the feature later)
		logger.Debug("unpackRelease: error - unpacked package blob dir already exists")
		return nil
	}

	//todo: should check if it exists before we download the release package...
	if fsutil.DirExists(commonBlobDir) {
		//feature: not removing the unpacked dist directory if it's already there (todo: revisit the feature later)
		logger.Debug("unpackRelease: error - release package dir already exists")
		return nil
	}

	file, err := os.Open(blobPath)
	if err != nil {
		return err
	}

	defer file.Close()

	destPath, err := unpackit.Unpack(file, releaseRootPath)
	if err != nil {
		return err
	}

	logger.Debugf("unpackRelease: unpacked package directory - %v", destPath)

	if err := os.Remove(blobPath); err != nil {
		logger.Debugf("unpackRelease: could not remove the release package blob - %v", err)
	}

	if err := os.Rename(unpackedBlobDir, commonBlobDir); err != nil {
		return err
	}

	return nil
}

func installRelease(logger *log.Entry, appRootPath, statePath, releaseRootPath string) error {
	newMasterAppPath := filepath.Join(releaseRootPath, distDirName, masterAppName)
	newSensorAppPath := filepath.Join(releaseRootPath, distDirName, sensorAppName)
	sensorAppPath := filepath.Join(appRootPath, sensorAppName)

	err := updateFile(logger, newSensorAppPath, sensorAppPath)
	if err != nil {
		return err
	}

	//will copy the sensor to the state dir if DS is installed in a bad non-shared location on Macs
	fsutil.PreparePostUpdateStateDir(statePath)

	err = updateFile(logger, newMasterAppPath, "")
	if err != nil {
		return err
	}

	return nil
}

func updateFile(logger *log.Entry, sourcePath, targetPath string) error {
	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()

	options := update.Options{}
	if targetPath != "" {
		options.TargetPath = targetPath
	}

	err = update.Apply(file, options)
	if err != nil {
		if rerr := update.RollbackError(err); rerr != nil {
			logger.Debugf("updateFile(%s,%s): Failed to rollback from bad update: %v",
				sourcePath, targetPath, rerr)
		}
	}
	return err
}

type BlobReaderConstructor func(size int, rc io.ReadCloser) io.ReadCloser

func newPassThroughReader(size int, rc io.ReadCloser) io.ReadCloser {
	return rc
}

func newProgressReader(size int, rc io.ReadCloser) io.ReadCloser {
	pr := progressReader{
		rc:       rc,
		size:     size,
		progress: uiprogress.New(),
	}

	if pr.progress != nil {
		pr.progress.SetRefreshInterval(time.Millisecond * 100)
		pr.progress.Start()

		pr.bar = pr.progress.AddBar(pr.size).AppendCompleted().PrependElapsed()
		pr.bar.Width = 50
	}

	return &pr
}

type progressReader struct {
	rc       io.ReadCloser
	size     int
	current  int
	progress *uiprogress.Progress
	bar      *uiprogress.Bar
}

func (pr *progressReader) Read(b []byte) (int, error) {
	count, err := pr.rc.Read(b)
	if err == nil {
		pr.current += count
		if pr.bar != nil {
			pr.bar.Set(pr.current)
		}
	}

	return count, err
}

func (pr *progressReader) Close() error {
	if pr.progress != nil {
		pr.progress.Stop()
	}
	io.Copy(io.Discard, pr.rc)
	return pr.rc.Close()
}
