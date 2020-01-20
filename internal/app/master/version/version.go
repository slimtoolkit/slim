package version

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	"github.com/docker-slim/docker-slim/pkg/system"
	v "github.com/docker-slim/docker-slim/pkg/version"
)

const (
	versionCheckEndpoint = "https://versions.api.dockerslim.com/check"
	jsonContentType      = "application/json"
	versionAuthKey       = "1JZg1RXvS6mZ0ANgf7p9PoYWQ9q.1JZg3zytWMmBVH50c0RvtBvVpq8"
)

type CheckVersionRequest struct {
	AppVersion string `json:"app_version"`
}

type CheckVersionInfo struct {
	Status   string `json:"status"`
	Outdated bool   `json:"outdated,omitempty"`
	Current  string `json:"current,omitempty"`
}

// PrintCheckVersion shows if the current version is outdated
func PrintCheckVersion(printPrefix string, info *CheckVersionInfo) {
	if info != nil && info.Status == "success" && info.Outdated {
		fmt.Printf("%s info=version status=OUTDATED local=%s current=%s\n", printPrefix, v.Tag(), info.Current)
		fmt.Printf("%s info=message message='Your version of DockerSlim is out of date! Use the \"update\" command or download the new version from https://dockersl.im/downloads.html'\n", printPrefix)
	}
}

// GetCheckVersionVerdict returns the version status of the locally installed package
func GetCheckVersionVerdict(info *CheckVersionInfo) string {
	if info != nil && info.Status == "success" {
		if info.Outdated {
			return fmt.Sprintf("your installed version is OUTDATED (local=%s current=%s)", v.Tag(), info.Current)
		} else {
			return "your have the latest version"
		}
	}

	return "version status information is not available"
}

// Print shows the master app version information
func Print(printPrefix string, logger *log.Entry, client *docker.Client, checkVersion, inContainer, isDSImage bool) {
	fmt.Printf("%s info=app version='%s' container=%v dsimage=%v\n", printPrefix, v.Current(), inContainer, isDSImage)
	if checkVersion {
		vinfo := Check(inContainer, isDSImage)
		outdated := "unknown"
		current := "unknown"
		if vinfo != nil && vinfo.Status == "success" {
			outdated = fmt.Sprintf("%v", vinfo.Outdated)
			current = vinfo.Current
		}
		fmt.Printf("%s info=app outdated=%v current=%v verdict='%v'\n", 
			printPrefix, outdated, current, GetCheckVersionVerdict(vinfo))
	}

	fmt.Printf("%s info=app location='%v'\n", printPrefix, fsutil.ExeDir())
	
	hostInfo := system.GetSystemInfo()
	fmt.Printf("%s info=host osname='%v'\n", printPrefix, hostInfo.OsName)
	fmt.Printf("%s info=host osbuild=%v\n", printPrefix, hostInfo.OsBuild)
	fmt.Printf("%s info=host version='%v'\n", printPrefix, hostInfo.Version)
	fmt.Printf("%s info=host release=%v\n", printPrefix, hostInfo.Release)
	fmt.Printf("%s info=host sysname=%v\n", printPrefix, hostInfo.Sysname)

	info, err := client.Info()
	if err != nil {
		fmt.Printf("%s error='error getting docker info'\n", printPrefix)
		logger.Debugf("Error getting docker info => %v", err)
		return
	}

	fmt.Printf("%s info=docker name=%v\n", printPrefix, info.Name)
	fmt.Printf("%s info=docker kernel_version=%v\n", printPrefix, info.KernelVersion)
	fmt.Printf("%s info=docker operating_system=%v\n", printPrefix, info.OperatingSystem)
	fmt.Printf("%s info=docker ostype=%v\n", printPrefix, info.OSType)
	fmt.Printf("%s info=docker server_version=%v\n", printPrefix, info.ServerVersion)
	fmt.Printf("%s info=docker architecture=%v\n", printPrefix, info.Architecture)

	ver, err := client.Version()
	if err != nil {
		fmt.Printf("%s error='error getting docker client version'\n", printPrefix)
		logger.Debugf("Error getting docker client version => %v", err)
		return
	}

	fmt.Printf("%s info=dclient api_version=%v\n", printPrefix, ver.Get("ApiVersion"))
	fmt.Printf("%s info=dclient min_api_version=%v\n", printPrefix, ver.Get("MinAPIVersion"))
	fmt.Printf("%s info=dclient build_time=%v\n", printPrefix, ver.Get("BuildTime"))
	fmt.Printf("%s info=dclient git_commit=%v\n", printPrefix, ver.Get("GitCommit"))
}

// Check checks the app version
func Check(inContainer, isDSImage bool) *CheckVersionInfo {
	logger := log.WithFields(log.Fields{"app": "docker-slim"})

	client := http.Client{
		Timeout: 13 * time.Second,
	}

	data := CheckVersionRequest{
		AppVersion: v.Current(),
	}

	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(&data); err != nil {
		logger.Debugf("Check - error encoding data => %v", err)
		return nil
	}

	req, err := http.NewRequest("POST", versionCheckEndpoint, &b)
	if err != nil {
		logger.Debugf("Check - error creating version check request => %v", err)
		return nil
	}
	hinfo := system.GetSystemInfo()
	req.Header.Set("User-Agent", fmt.Sprintf("DockerSlimApp/%v/%v/%v/%v",
		v.Current(), inContainer, isDSImage, hinfo.OsName))
	req.Header.Set("Content-Type", jsonContentType)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", versionAuthKey))

	resp, err := client.Do(req)
	if resp != nil && resp.Body != nil {
		defer func() {
			io.Copy(ioutil.Discard, resp.Body)
			resp.Body.Close()
		}()
	}

	if err != nil {
		logger.Debugf("Check - error checking version => %v", err)
		return nil
	}

	logger.Debug("version.Check: http status = ", resp.Status)
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var checkInfo CheckVersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&checkInfo); err != nil {
		logger.Debugf("Check - error decoding response => %v", err)
		return nil
	}

	return &checkInfo
}

// CheckAsync checks the app version without blocking
func CheckAsync(doCheckVersion, inContainer, isDSImage bool) <-chan *CheckVersionInfo {
	resultCh := make(chan *CheckVersionInfo, 1)

	if doCheckVersion {
		go func() {
			resultCh <- Check(inContainer, isDSImage)
		}()
	} else {
		close(resultCh)
	}

	return resultCh
}

// CheckAndPrintAsync check the app version and prints the results
func CheckAndPrintAsync(printPrefix string, inContainer, isDSImage bool) {
	go func() {
		PrintCheckVersion(printPrefix, Check(inContainer, isDSImage))
	}()
}
