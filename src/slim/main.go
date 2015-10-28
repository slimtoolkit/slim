package main

import (
	"os"
	"fmt"
	"log"
	"net"
	"net/url"
	"time"
	"bytes"
	"bufio"
	"strings"
	//"syscall"
	"strconv"
	//"os/signal"
	"io/ioutil"
	"encoding/json"
	"path/filepath"
	"text/template"
	
	"github.com/cloudimmunity/go-dockerclientx"
	"github.com/dustin/go-humanize"
	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/req"
	"github.com/gdamore/mangos/protocol/sub"
	//"github.com/gdamore/mangos/transport/ipc"
	"github.com/gdamore/mangos/transport/tcp"
)

func failOnError(err error) {
	if err != nil {
		log.Fatalln("docker-slim: ERROR =>", err)
	}
}

func warnOnError(err error) {
	if err != nil {
		log.Println("docker-slim: ERROR =>", err)
	}
}

func failWhen(cond bool, msg string) {
	if cond {
		log.Fatalln("docker-slim: ERROR =>", msg)
	}
}

func myFileDir() string {
	dirName, err := filepath.Abs(filepath.Dir(os.Args[0]))
	failOnError(err)
	return dirName
}

func getDockerHostIp() string {
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		return "127.0.0.1"
	}

	u, err := url.Parse(dockerHost)
	if err != nil {
		return "127.0.0.1"
	}

	switch u.Scheme {
	case "unix":
		return "127.0.0.1"
	default:
		host,_, err := net.SplitHostPort(u.Host)
		if err != nil {
			return "127.0.0.1"
		}

		return host
	}
}

///////////////////////////////////////////////////////////////////////////////

//var cmdChannelAddr = "ipc:///tmp/docker-slim-launcher.cmds.ipc"
var cmdChannelAddr = "tcp://127.0.0.1:65501"
var cmdChannel mangos.Socket

func newCmdClient(addr string) (mangos.Socket,error) {
	socket, err := req.NewSocket()
	if err != nil {
		return nil,err
	}

	if err := socket.SetOption(mangos.OptionSendDeadline,time.Second * 3); err != nil {
		socket.Close()
		return nil,err
	}

	if err := socket.SetOption(mangos.OptionRecvDeadline,time.Second * 3); err != nil {
		socket.Close()
		return nil,err
	}

	//socket.AddTransport(ipc.NewTransport())
	socket.AddTransport(tcp.NewTransport())
	if err := socket.Dial(addr); err != nil {
		socket.Close()
		return nil,err
	}

	return socket,nil
}

func shutdownCmdChannel() {
	if cmdChannel != nil {
		cmdChannel.Close()
		cmdChannel = nil
	}
}

func sendCmd(channel mangos.Socket, cmd string) (string,error) {
	sendTimeouts := 0
	recvTimeouts := 0

	log.Printf("sendCmd(%s)\n",cmd)
	for {
		if err := channel.Send([]byte(cmd)); err != nil {
			switch err {
				case mangos.ErrSendTimeout:
					log.Println("sendCmd(): send timeout...")
					sendTimeouts++
					if sendTimeouts > 3 {
						return "",err
					}
				default:
					return "",err
			}
		}

		response, err := channel.Recv()
		if err != nil {
			switch err {
				case mangos.ErrRecvTimeout:
					log.Println("sendCmd(): receive timeout...")
					recvTimeouts++
					if recvTimeouts > 3 {
						return "",err
					}
				default:
					return "",err
			}
		}

		return string(response),nil
	}
}

var evtChannelAddr = "tcp://127.0.0.1:65502"
//var evtChannelAddr = "ipc:///tmp/docker-slim-launcher.events.ipc"
var evtChannel mangos.Socket

func newEvtChannel(addr string) (mangos.Socket,error) {
	socket, err := sub.NewSocket()
	if err != nil {
		return nil,err
	}

	//if err := socket.SetOption(mangos.OptionRecvDeadline,time.Second * 30); err != nil {
	//	socket.Close()
	//	return nil,err
	//}

	//socket.AddTransport(ipc.NewTransport())
	socket.AddTransport(tcp.NewTransport())
	if err := socket.Dial(addr); err != nil {
		socket.Close()
		return nil,err
	}

	err = socket.SetOption(mangos.OptionSubscribe, []byte(""))
	if err != nil {
		return nil,err
	}

	return socket,nil
}

func shutdownEvtChannel() {
	if evtChannel != nil {
		evtChannel.Close()
		evtChannel = nil
	}
}

func getEvt(channel mangos.Socket) (string,error) {
	log.Println("getEvt()")
	evt, err := channel.Recv()
	if err != nil {
		return "",err
	}

	return string(evt),nil
}

///////////////////////////////////////////////////////////////////////////////

type imageInst struct {
	instCmd      string
	instComment  string
	instType     string
	instTime     int64
	layerImageId string
	imageName    string
	shortTags    []string
	fullTags     []string
}

func genDockerfileFromHistory(apiClient *docker.Client, imageId string) ([]string, error) {
	//NOTE: comment field is missing (TODO: enhance the lib...)
	imageHistory, err := apiClient.ImageHistory(imageId)
	if err != nil {
		return nil, err
	}

	//log.Printf("\n\nIMAGE HISTORY =>\n%#v\n\n",imageHistory)

	var fatImageDockerInstructions []imageInst

	imageLayerCount := len(imageHistory)
	imageLayerStart := imageLayerCount - 1
	if imageLayerCount > 0 {
		for idx := imageLayerStart; idx >= 0; idx-- {
			nopPrefix := "/bin/sh -c #(nop) "
			execPrefix := "/bin/sh -c "
			rawLine := imageHistory[idx].CreatedBy
			var inst string

			if len(rawLine) == 0 {
				inst = "FROM scratch"
			} else if strings.HasPrefix(rawLine, nopPrefix) {
				inst = strings.TrimPrefix(rawLine, nopPrefix)
			} else if strings.HasPrefix(rawLine, execPrefix) {
				runData := strings.TrimPrefix(rawLine, execPrefix)
				if strings.Contains(runData, "&&") {
					parts := strings.Split(runData, "&&")
					for i := range parts {
						partPrefix := ""
						if i != 0 {
							partPrefix = "\t"
						}
						parts[i] = partPrefix + strings.TrimSpace(parts[i])
					}
					runDataFormatted := strings.Join(parts, " && \\\n")
					inst = "RUN " + runDataFormatted
				} else {
					inst = "RUN " + runData
				}
			} else {
				inst = rawLine
			}

			if strings.HasPrefix(inst, "ENTRYPOINT ") {
				inst = strings.Replace(inst, "&{[", "[", -1)
				inst = strings.Replace(inst, "]}", "]", -1)
			}

			instInfo := imageInst{
				instCmd:      inst,
				instTime:     imageHistory[idx].Created,
				layerImageId: imageHistory[idx].ID,
				instComment:  imageHistory[idx].Comment,
			}

			instType := "intermediate"
			if idx == imageLayerStart {
				instType = "first"
			}

			if len(imageHistory[idx].Tags) > 0 {
				instType = "last"

				if tagInfo := strings.Split(imageHistory[idx].Tags[0], ":"); len(tagInfo) > 1 {
					instInfo.imageName = tagInfo[0]
				}

				instInfo.fullTags = imageHistory[idx].Tags

				for _, fullTag := range instInfo.fullTags {
					if tagInfo := strings.Split(fullTag, ":"); len(tagInfo) > 1 {
						instInfo.shortTags = append(instInfo.shortTags, tagInfo[1])
					}
				}
			}

			instInfo.instType = instType

			fatImageDockerInstructions = append(fatImageDockerInstructions, instInfo)
		}
	}

	var fatImageDockerfileLines []string
	for idx, instInfo := range fatImageDockerInstructions {
		if instInfo.instType == "first" {
			fatImageDockerfileLines = append(fatImageDockerfileLines, "# new image")
		}

		fatImageDockerfileLines = append(fatImageDockerfileLines, instInfo.instCmd)
		if instInfo.instType == "last" {
			commentText := fmt.Sprintf("# end of image: %s (id: %s tags: %s)",
				instInfo.imageName, instInfo.layerImageId, strings.Join(instInfo.shortTags, ","))
			fatImageDockerfileLines = append(fatImageDockerfileLines, commentText)
			fatImageDockerfileLines = append(fatImageDockerfileLines, "")
			if idx < (len(fatImageDockerInstructions) - 1) {
				fatImageDockerfileLines = append(fatImageDockerfileLines, "# new image")
			}
		}

		if instInfo.instComment != "" {
			fatImageDockerfileLines = append(fatImageDockerfileLines, "# "+instInfo.instComment)
		}

		//TODO: use time diff to separate each instruction
	}

	//log.Printf("IMAGE INSTRUCTIONS:")
	//for _, iiLine := range fatImageDockerfileLines {
	//	log.Println(iiLine)
	//}

	return fatImageDockerfileLines, nil

	//TODO: try adding comments in the docker file to see if the comments
	//show up in the 'history' command

	/*
	   NOTE:
	   Usually "MAINTAINER" is the first instruction,
	   so it can be used to detect a base image.
	*/

	/*
	   TODO:
	   need to have a set of signature for common base images
	   long path: need to discover base images dynamically
	   https://imagelayers.io/?images=alpine:3.1,ubuntu:14.04.1&lock=alpine:3.1

	   https://imagelayers.io/
	   https://github.com/CenturyLinkLabs/imagelayers
	   https://github.com/CenturyLinkLabs/imagelayers-graph
	*/
}

func saveDockerfileData(fatImageDockerfileLocation string, fatImageDockerInstructions []string) error {
	var data bytes.Buffer
	data.WriteString(strings.Join(fatImageDockerInstructions, "\n"))
	return ioutil.WriteFile(fatImageDockerfileLocation, data.Bytes(), 0644)
}

/////////////////////////////////////////////////////////////////////////////////

const appArmorTemplate = `
profile {{.ProfileName}} flags=(attach_disconnected,mediate_deleted) {

  network,

{{range $value := .ExeFileRules}}  {{$value.FilePath}} {{$value.PermSet}},
{{end}}
{{range $value := .WriteFileRules}}  {{$value.FilePath}} {{$value.PermSet}},
{{end}}
{{range $value := .ReadFileRules}}  {{$value.FilePath}} {{$value.PermSet}},
{{end}}
}
`

type appArmorFileRule struct {
	FilePath string
	PermSet  string
}

type appArmorProfileData struct {
	ProfileName    string
	ExeFileRules   []appArmorFileRule
	WriteFileRules []appArmorFileRule
	ReadFileRules  []appArmorFileRule
}

////////////////////
//TODO: REFACTOR :)

type artifactType int

const (
	DirArtifactType     = 1
	FileArtifactType    = 2
	SymlinkArtifactType = 3
	UnknownArtifactType = 99
)

var artifactTypeNames = map[artifactType]string{
	DirArtifactType:     "Dir",
	FileArtifactType:    "File",
	SymlinkArtifactType: "Symlink",
	UnknownArtifactType: "Unknown",
}

func (t artifactType) String() string {
	return artifactTypeNames[t]
}

var artifactTypeValues = map[string]artifactType{
	"Dir":     DirArtifactType,
	"File":    FileArtifactType,
	"Symlink": SymlinkArtifactType,
	"Unknown": UnknownArtifactType,
}

func getArtifactTypeValue(s string) artifactType {
	return artifactTypeValues[s]
}

type processInfo struct {
	Pid       int32  `json:"pid"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Cmd       string `json:"cmd"`
	Cwd       string `json:"cwd"`
	Root      string `json:"root"`
	ParentPid int32  `json:"ppid"`
}

type fileInfo struct {
	EventCount   uint32 `json:"event_count"`
	FirstEventId uint32 `json:"first_eid"`
	Name         string `json:"-"`
	ReadCount    uint32 `json:"reads,omitempty"`
	WriteCount   uint32 `json:"writes,omitempty"`
	ExeCount     uint32 `json:"execs,omitempty"`
}

type monitorReport struct {
	MonitorPid       int                             `json:"monitor_pid"`
	MonitorParentPid int                             `json:"monitor_ppid"`
	EventCount       uint32                          `json:"event_count"`
	MainProcess      *processInfo                    `json:"main_process"`
	Processes        map[string]*processInfo         `json:"processes"`
	ProcessFiles     map[string]map[string]*fileInfo `json:"process_files"`
}

type artifactProps struct {
	FileType artifactType    `json:"-"` //todo
	FilePath string          `json:"file_path"`
	Mode     os.FileMode     `json:"-"` //todo
	ModeText string          `json:"mode"`
	LinkRef  string          `json:"link_ref,omitempty"`
	Flags    map[string]bool `json:"flags,omitempty"`
	DataType string          `json:"data_type,omitempty"`
	FileSize int64           `json:"file_size"`
	Sha1Hash string          `json:"sha1_hash,omitempty"`
	AppType  string          `json:"app_type,omitempty"`
}

func (p *artifactProps) UnmarshalJSON(data []byte) error {
	type artifactPropsType artifactProps
	props := &struct {
		FileTypeStr string `json:"file_type"`
		*artifactPropsType
	}{
		artifactPropsType: (*artifactPropsType)(p),
	}

	if err := json.Unmarshal(data, &props); err != nil {
		return err
	}
	p.FileType = getArtifactTypeValue(props.FileTypeStr)

	return nil
}

type ImageReport struct {
	Files []*artifactProps `json:"files"`
}

type ContainerReport struct {
	Monitor *monitorReport `json:"monitor"`
	Image   ImageReport    `json:"image"`
}

///////////

func permSetFromFlags(flags map[string]bool) string {
	var b bytes.Buffer
	if flags["R"] {
		b.WriteString("r")
	}

	if flags["W"] {
		b.WriteString("w")
	}

	if flags["X"] {
		b.WriteString("ix")
	}

	return b.String()
}

//TODO:
//need to safe more metadata about the artifacts in the monitor data
//1. exe bit
//2. w/r operation info (so we can add useful write rules)
func genAppArmorProfile(artifactLocation string, profileName string) error {
	containerReportFileName := "creport.json"
	containerReportFilePath := fmt.Sprintf("%s/%s", artifactLocation, containerReportFileName)

	if _, err := os.Stat(containerReportFilePath); err != nil {
		return err
	}
	reportFile, err := os.Open(containerReportFilePath)
	if err != nil {
		return err
	}
	defer reportFile.Close()

	var report ContainerReport
	if err = json.NewDecoder(reportFile).Decode(&report); err != nil {
		return err
	}

	profilePath := fmt.Sprintf("%s/%s", artifactLocation, profileName)

	profileFile, err := os.OpenFile(profilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	defer profileFile.Close()

	profileData := appArmorProfileData{ProfileName: profileName}

	for _, aprops := range report.Image.Files {
		if aprops.Flags["X"] {
			profileData.ExeFileRules = append(profileData.ExeFileRules,
				appArmorFileRule{
					FilePath: aprops.FilePath,
					PermSet:  permSetFromFlags(aprops.Flags),
				})
		} else if aprops.Flags["W"] {
			profileData.WriteFileRules = append(profileData.WriteFileRules,
				appArmorFileRule{
					FilePath: aprops.FilePath,
					PermSet:  permSetFromFlags(aprops.Flags),
				})
		} else if aprops.Flags["R"] {
			profileData.ReadFileRules = append(profileData.ReadFileRules,
				appArmorFileRule{
					FilePath: aprops.FilePath,
					PermSet:  permSetFromFlags(aprops.Flags),
				})
		} else {
			log.Printf("docker-slim: genAppArmorProfile - other artifact => %v\n", aprops)
		}
	}

	t, err := template.New("profile").Parse(appArmorTemplate)
	if err != nil {
		return err
	}

	if err := t.Execute(profileFile, profileData); err != nil {
		return err
	}

	return nil
}

/////////////////////////////////////////////////////////////////////////////////

func main() {
	failWhen(len(os.Args) < 2, "docker-slim: error => missing image info")

	imageId := os.Args[1]

	features := map[string]bool{}
	if len(os.Args) > 2 {
		for _, fname := range os.Args[2:] {
			features[fname] = true
		}
	}

	client, _ := docker.NewClientFromEnv()

	log.Println("docker-slim: inspecting \"fat\" image metadata...")
	imageInfo, err := client.InspectImage(imageId)
	if err != nil {
		if err == docker.ErrNoSuchImage {
			log.Fatalf("docker-slim: could not find target image")
		}
		log.Fatalf("docker-slim: InspectImage(%v) error => %v", imageId, err)
	}

	var imageRecord docker.APIImages
	imageList, err := client.ListImages(docker.ListImagesOptions{All: true})
	failOnError(err)
	for _, r := range imageList {
		if r.ID == imageInfo.ID {
			imageRecord = r
			break
		}
	}

	if imageRecord.ID == "" {
		log.Fatalf("docker-slim: could not find target image in the image list")
	}

	appArmorProfileName := "apparmor-profile"
	slimImageRepo := "slim"
	if len(imageRecord.RepoTags) > 0 {
		if rtInfo := strings.Split(imageRecord.RepoTags[0], ":"); len(rtInfo) > 1 {
			slimImageRepo = fmt.Sprintf("%s.slim", rtInfo[0])
			if nameParts := strings.Split(rtInfo[0], "/"); len(nameParts) > 1 {
				appArmorProfileName = strings.Join(nameParts, "-")
			} else {
				appArmorProfileName = rtInfo[0]
			}
			appArmorProfileName = fmt.Sprintf("%s-apparmor-profile", appArmorProfileName)
		}
	}

	log.Printf("docker-slim: \"fat\" image size => %v (%v)\n",
		imageInfo.VirtualSize, humanize.Bytes(uint64(imageInfo.VirtualSize)))

	fatImageDockerInstructions, err := genDockerfileFromHistory(client, imageId)
	failOnError(err)

	imageMeta := struct {
		RepoName     string
		ID           string
		Entrypoint   []string
		Cmd          []string
		WorkingDir   string
		Env          []string
		ExposedPorts map[docker.Port]struct{}
		Volumes      map[string]struct{}
		OnBuild      []string
		User         string
	}{
		slimImageRepo,
		imageInfo.ID,
		imageInfo.Config.Entrypoint,
		imageInfo.Config.Cmd,
		imageInfo.Config.WorkingDir,
		imageInfo.Config.Env,
		imageInfo.Config.ExposedPorts,
		imageInfo.Config.Volumes,
		imageInfo.Config.OnBuild,
		imageInfo.Config.User,
	}

	var fatContainerCmd []string
	if len(imageInfo.Config.Entrypoint) > 0 {
		fatContainerCmd = append(fatContainerCmd, imageInfo.Config.Entrypoint...)
	}

	if len(imageInfo.Config.Cmd) > 0 {
		fatContainerCmd = append(fatContainerCmd, imageInfo.Config.Cmd...)
	}

	localVolumePath := fmt.Sprintf("%s/container", myFileDir())

	artifactLocation := fmt.Sprintf("%v/artifacts", localVolumePath)
	artifactDir, err := os.Stat(artifactLocation)
	if os.IsNotExist(err) {
		os.MkdirAll(artifactLocation, 0777)
		artifactDir, err = os.Stat(artifactLocation)
		failOnError(err)
	}
	
	ipcLocation := fmt.Sprintf("%v/ipc", localVolumePath)
	_, err = os.Stat(ipcLocation)
	if os.IsNotExist(err) {
		os.MkdirAll(ipcLocation, 0777)
		_, err = os.Stat(ipcLocation)
		failOnError(err)
	}

	failWhen(!artifactDir.IsDir(), "artifact location is not a directory")

	log.Println("docker-slim: saving \"fat\" image info...")
	fatImageDockerfileLocation := fmt.Sprintf("%v/Dockerfile.fat", artifactLocation)
	err = saveDockerfileData(fatImageDockerfileLocation, fatImageDockerInstructions)
	failOnError(err)

	if !features["image-info-only"] {
		mountInfo := fmt.Sprintf("%s:/opt/dockerslim", localVolumePath)

		var cmdPort docker.Port = "65501/tcp"
		var evtPort docker.Port = "65502/tcp"

		containerOptions := docker.CreateContainerOptions{
			Name: "dockerslimk",
			Config: &docker.Config{
				Image: imageId,
				// NOTE: specifying Mounts here doesn't work :)
				//Mounts: []docker.Mount{{
				//        Source: localVolumePath,
				//        Destination: "/opt/dockerslim",
				//        Mode: "",
				//        RW: true,
				//    },
				//},
				ExposedPorts: map[docker.Port]struct{}{
					cmdPort: struct{}{},
					evtPort: struct{}{},
					},
				Entrypoint: []string{"/opt/dockerslim/bin/alauncher"},
				Cmd:        fatContainerCmd,
				Labels:     map[string]string{"type": "dockerslim"},
			},
			HostConfig: &docker.HostConfig{
				Binds:           []string{mountInfo},
				PublishAllPorts: true,
				CapAdd:          []string{"SYS_ADMIN"},
				Privileged:      true,
			},
		}

		log.Println("docker-slim: creating instrumented \"fat\" container...")
		containerInfo, err := client.CreateContainer(containerOptions)
		failOnError(err)
		log.Println("docker-slim: created container =>", containerInfo.ID)

		log.Println("docker-slim: starting \"fat\" container...")

		err = client.StartContainer(containerInfo.ID, &docker.HostConfig{
			PublishAllPorts: true,
			CapAdd:          []string{"SYS_ADMIN"},
			Privileged:      true,
		})
		failOnError(err)

		inspContainerInfo, err := client.InspectContainer(containerInfo.ID)
		failWhen(inspContainerInfo.NetworkSettings == nil, "docker-slim: error => no network info")
		log.Printf("container NetworkSettings.Ports => %#v\n",inspContainerInfo.NetworkSettings.Ports)
		
		cmdPortBindings := inspContainerInfo.NetworkSettings.Ports[cmdPort]
		evtPortBindings := inspContainerInfo.NetworkSettings.Ports[evtPort]
		dockerHostIp := getDockerHostIp()
		cmdChannelAddr = fmt.Sprintf("tcp://%v:%v", dockerHostIp,cmdPortBindings[0].HostPort)
		evtChannelAddr = fmt.Sprintf("tcp://%v:%v", dockerHostIp,evtPortBindings[0].HostPort)
		log.Printf("cmdChannelAddr=%v evtChannelAddr=%v\n",cmdChannelAddr,evtChannelAddr)
		//TODO: keep checking the monitor state until no new files (and processes) are discovered
		log.Println("docker-slim: watching container monitor...")

		//evtChannelAddr = fmt.Sprintf("ipc://%v/ipc/docker-slim-launcher.events.ipc", localVolumePath)
		//cmdChannelAddr = fmt.Sprintf("ipc://%v/ipc/docker-slim-launcher.cmds.ipc", localVolumePath)

		evtChannel,err = newEvtChannel(evtChannelAddr)
		failOnError(err)
		cmdChannel,err = newCmdClient(cmdChannelAddr)
		failOnError(err)

		//endTime := time.After(time.Second * 200)
		//work := 0
		//doneWatching:
		//for {
		//	select {
		//	case <-endTime:
		//		log.Println("docker-slim: done with work!")
		//		break doneWatching
		//	case <-time.After(time.Second * 3):
		//		work++
		//		log.Println("docker-slim: still watching =>", work)
		//	}
		//}
		log.Println("press any key when you are done using the container...")
		creader := bufio.NewReader(os.Stdin)
		_,_,_ = creader.ReadLine() //or _,_ = creaderReadString('\n')
		cmdResponse,err := sendCmd(cmdChannel,"monitor.finish")
		log.Printf("'monitor.finish' response => '%v'\n",cmdResponse)
		log.Println("waiting for the container finish its work...")
		//for now there's only one event ("done")
		evt,err := getEvt(evtChannel)
		log.Printf("got alauncher event => '%v'\n",evt)

		shutdownEvtChannel()
		shutdownCmdChannel()

		//log.Println("docker-slim: exporting \"fat\" container artifacts...")
		//time.Sleep(5 * time.Second)

		log.Println("docker-slim: stopping \"fat\" container...")
		err = client.StopContainer(containerInfo.ID, 9)
		warnOnError(err)

		log.Println("docker-slim: removing \"fat\" container...")
		removeOption := docker.RemoveContainerOptions{
			ID:            containerInfo.ID,
			RemoveVolumes: true,
			Force:         true,
		}
		err = client.RemoveContainer(removeOption)
		warnOnError(err)

		log.Println("docker-slim: generating AppArmor profile...")
		err = genAppArmorProfile(artifactLocation, appArmorProfileName)
		failOnError(err)

		log.Println("docker-slim: creating \"slim\" image...")

		dockerfileLocation := fmt.Sprintf("%v/Dockerfile", artifactLocation)

		var dfData bytes.Buffer
		dfData.WriteString("FROM scratch\n")
		dfData.WriteString("COPY files /\n")

		dfData.WriteString("WORKDIR ")
		dfData.WriteString(imageMeta.WorkingDir)
		dfData.WriteByte('\n')

		if len(imageMeta.Env) > 0 {
			for _, envInfo := range imageMeta.Env {
				if envParts := strings.Split(envInfo, "="); len(envParts) > 1 {
					dfData.WriteString("ENV ")
					envLine := fmt.Sprintf("%s %s", envParts[0], envParts[1])
					dfData.WriteString(envLine)
					dfData.WriteByte('\n')
				}
			}
		}

		if len(imageMeta.ExposedPorts) > 0 {
			for portInfo := range imageMeta.ExposedPorts {
				dfData.WriteString("EXPOSE ")
				dfData.WriteString(string(portInfo))
				dfData.WriteByte('\n')
			}
		}

		if len(imageMeta.Entrypoint) > 0 {
			var quotedEntryPoint []string
			for idx := range imageMeta.Entrypoint {
				quotedEntryPoint = append(quotedEntryPoint, strconv.Quote(imageMeta.Entrypoint[idx]))
			}
			/*
				"Entrypoint": [
				            "/bin/sh",
				            "-c",
				            "node /opt/my/service/server.js"
				        ],
			*/
			dfData.WriteString("ENTRYPOINT [")
			dfData.WriteString(strings.Join(quotedEntryPoint, ","))
			dfData.WriteByte(']')
			dfData.WriteByte('\n')
		}

		if len(imageMeta.Cmd) > 0 {
			var quotedCmd []string
			for idx := range imageMeta.Entrypoint {
				quotedCmd = append(quotedCmd, strconv.Quote(imageMeta.Cmd[idx]))
			}
			dfData.WriteString("CMD [")
			dfData.WriteString(strings.Join(quotedCmd, ","))
			dfData.WriteByte(']')
			dfData.WriteByte('\n')
		}

		err = ioutil.WriteFile(dockerfileLocation, dfData.Bytes(), 0644)
		failOnError(err)

		buildOptions := docker.BuildImageOptions{
			Name:           imageMeta.RepoName,
			RmTmpContainer: true,
			ContextDir:     artifactLocation,
			Dockerfile:     "Dockerfile",
			OutputStream:   os.Stdout,
		}

		err = client.BuildImage(buildOptions)
		failOnError(err)
		log.Println("docker-slim: created new image:", imageMeta.RepoName)

		if features["rm-artifacts"] {
			log.Println("docker-slim: removing temporary artifacts (TODO)...")
			err = os.RemoveAll(artifactLocation)
			warnOnError(err)
		}
	}
}

// eval "$(docker-machine env default)"
// ./dockerslim 6f74095b68c9
// ./dockerslim 6f74095b68c9 rm-artifacts
// ./dockerslim 6f74095b68c9 image-info-only
