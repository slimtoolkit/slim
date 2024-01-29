package version

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app"
	//"github.com/slimtoolkit/slim/pkg/app/master/commands"
	"github.com/slimtoolkit/slim/pkg/system"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
	v "github.com/slimtoolkit/slim/pkg/version"
)

const (
	versionCheckEndpoint = "https://versions.api.dockerslim.com/check"
	jsonContentType      = "application/json"
	versionAuthKey       = "1JZg1RXvS6mZ0ANgf7p9PoYWQ9q.1JZg3zytWMmBVH50c0RvtBvVpq8"
)

type ovars = app.OutVars
type CheckVersionRequest struct {
	AppVersion string `json:"app_version"`
}

type CheckVersionInfo struct {
	Status   string `json:"status"`
	Outdated bool   `json:"outdated,omitempty"`
	Current  string `json:"current,omitempty"`
}

// PrintCheckVersion shows if the current version is outdated
func PrintCheckVersion(
	xc *app.ExecutionContext,
	printPrefix string,
	info *CheckVersionInfo) {
	if info != nil && info.Status == "success" && info.Outdated {
		msg := "Your version of SlimToolkit is out of date! Use `slim update` to get the latest version."
		if xc == nil {
			fmt.Printf("%s info=version status=OUTDATED local=%s current=%s\n", printPrefix, v.Tag(), info.Current)
			fmt.Printf("%s info=message message='%s'\n", printPrefix, msg)
		} else {
			xc.Out.Info("version",
				app.OutVars{
					"status":  "OUTDATED",
					"local":   v.Tag(),
					"current": info.Current,
				})
			xc.Out.Message(msg)
		}
	}
}

// GetCheckVersionVerdict returns the version status of the locally installed package
func GetCheckVersionVerdict(info *CheckVersionInfo) string {
	if info != nil && info.Status == "success" {
		if info.Outdated {
			return fmt.Sprintf("your installed version is OUTDATED (local=%s current=%s)", v.Tag(), info.Current)
		} else {
			return "you have the latest version"
		}
	}

	return "version status information is not available"
}

// Print shows the master app version information
func Print(xc *app.ExecutionContext, cmdNameParam string, logger *log.Entry, client *docker.Client, checkVersion, inContainer, isDSImage bool) {

	ovApp := ovars{
		"cmd":       cmdNameParam,
		"version":   v.Current(),
		"container": inContainer,
		"dsimage":   isDSImage,
		"location":  fsutil.ExeDir(),
	}

	if checkVersion {
		vinfo := Check(inContainer, isDSImage)
		current := "unknown"
		if vinfo != nil && vinfo.Status == "success" {
			if vinfo.Outdated {
				ovApp["status"] = "OUTDATED"
			}
			current = vinfo.Current
		}

		ovApp["current"] = current
		ovApp["verdict"] = GetCheckVersionVerdict(vinfo)
	}
	xc.Out.Info("app", ovApp)

	hostInfo := system.GetSystemInfo()
	ovHost := ovars{
		"cmd":     cmdNameParam,
		"osname":  hostInfo.Distro.DisplayName,
		"osbuild": hostInfo.OsBuild,
		"version": hostInfo.Version,
		"release": hostInfo.Release,
		"sysname": hostInfo.Sysname,
	}
	xc.Out.Info("host", ovHost)

	if client != nil {
		info, err := client.Info()
		if err != nil {
			xc.Out.Error("error getting docker info", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		ovDocker := ovars{
			"cmd":              cmdNameParam,
			"name":             info.Name,
			"kernel.version":   info.KernelVersion,
			"operating.system": info.OperatingSystem,
			"ostype":           info.OSType,
			"server.version":   info.ServerVersion,
			"architecture":     info.Architecture,
		}
		xc.Out.Info("docker", ovDocker)

		ver, err := client.Version()
		if err != nil {
			xc.Out.Error("error getting docker client version", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}
		ovDockerClient := ovars{
			"cmd":             cmdNameParam,
			"api.version":     ver.Get("ApiVersion"),
			"min.api.version": ver.Get("MinAPIVersion"),
			"build.time":      ver.Get("BuildTime"),
			"git.commit":      ver.Get("GitCommit"),
		}
		xc.Out.Info("dclient", ovDockerClient)
	} else {
		xc.Out.Info("no.docker.client", ovars{})
	}

}

// Check checks the app version
func Check(inContainer, isDSImage bool) *CheckVersionInfo {
	logger := log.WithFields(log.Fields{"app": "slim"})

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
		v.Current(), inContainer, isDSImage, hinfo.Distro.DisplayName))
	req.Header.Set("Content-Type", jsonContentType)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", versionAuthKey))

	resp, err := client.Do(req)
	if resp != nil && resp.Body != nil {
		defer func() {
			io.Copy(io.Discard, resp.Body)
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
		PrintCheckVersion(nil, printPrefix, Check(inContainer, isDSImage))
	}()
}
