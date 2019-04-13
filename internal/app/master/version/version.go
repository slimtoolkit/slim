package version

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/cloudimmunity/go-dockerclientx"
	"github.com/cloudimmunity/system"
	v "github.com/docker-slim/docker-slim/pkg/version"

	log "github.com/Sirupsen/logrus"
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
func PrintCheckVersion(info *CheckVersionInfo) {
	if info != nil && info.Status == "success" && info.Outdated {
		fmt.Printf("docker-slim[version]: info=version status=OUTDATED version=%s current=%s\n", v.Tag(), info.Current)
		fmt.Printf("docker-slim[version]: info=message message='Your version of DockerSlim is out of date!'\n")
	}
}

// Print shows the master app version information
func Print(client *docker.Client, checkVersion bool) {
	fmt.Printf("docker-slim[version]: %s\n", v.Current())
	if checkVersion {
		PrintCheckVersion(Check())
	}

	fmt.Println("host:")
	hostInfo := system.GetSystemInfo()
	fmt.Printf("OsName=%v\n", hostInfo.OsName)
	fmt.Printf("OsBuild=%v\n", hostInfo.OsBuild)
	fmt.Printf("Version=%v\n", hostInfo.Version)
	fmt.Printf("Release=%v\n", hostInfo.Release)
	fmt.Printf("Sysname=%v\n", hostInfo.Sysname)

	fmt.Println("docker:")
	info, err := client.Info()
	if err != nil {
		fmt.Println("error getting docker info")
		return
	}

	fmt.Printf("Name=%v\n", info.Name)
	fmt.Printf("KernelVersion=%v\n", info.KernelVersion)
	fmt.Printf("OperatingSystem=%v\n", info.OperatingSystem)
	fmt.Printf("OSType=%v\n", info.OSType)
	fmt.Printf("ServerVersion=%v\n", info.ServerVersion)
	fmt.Printf("Architecture=%v\n", info.Architecture)

	ver, err := client.Version()
	if err != nil {
		fmt.Println("error getting docker version")
		return
	}

	fmt.Printf("ApiVersion=%v\n", ver.Get("ApiVersion"))
	fmt.Printf("MinAPIVersion=%v\n", ver.Get("MinAPIVersion"))
	fmt.Printf("BuildTime=%v\n", ver.Get("BuildTime"))
	fmt.Printf("GitCommit=%v\n", ver.Get("GitCommit"))
}

// Check checks the app version
func Check() *CheckVersionInfo {
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
		logger.Info("Check - error encoding data => %v", err)
		return nil
	}

	//resp, err := client.Post(versionCheckEndpoint, jsonContentType, &b)
	//versionAuthKey
	req, err := http.NewRequest("POST", versionCheckEndpoint, &b)
	if err != nil {
		logger.Info("Check - error creating version check request => %v", err)
		return nil
	}

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
		logger.Info("Check - error checking version => %v", err)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		logger.Info("Check - unexpected response status =", resp.Status)
		return nil
	}

	var checkInfo CheckVersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&checkInfo); err != nil {
		logger.Info("Check - error decoding response => %v", err)
		return nil
	}

	return &checkInfo
}

// CheckAsync checks the app version without blocking
func CheckAsync(doCheckVersion bool) <-chan *CheckVersionInfo {
	resultCh := make(chan *CheckVersionInfo, 1)

	if doCheckVersion {
		go func() {
			resultCh <- Check()
		}()
	} else {
		close(resultCh)
	}

	return resultCh
}

// CheckAndPrintAsync check the app version and prints the results
func CheckAndPrintAsync() {
	go func() {
		PrintCheckVersion(Check())
	}()
}
