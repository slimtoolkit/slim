package main

import (
    "time"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "strings"
    "io/ioutil"
    "bytes"
    "strconv"

    "github.com/dustin/go-humanize"
    "github.com/fsouza/go-dockerclient"
)

func failOnError(err error) {
    if err != nil {
        log.Fatalln("docker-slim: ERROR =>",err)
    }
}

func warnOnError(err error) {
    if err != nil {
        log.Println("docker-slim: ERROR =>",err)
    }
}

func failWhen(cond bool,msg string) {
    if cond {
        log.Fatalln("docker-slim: ERROR =>",msg)
    }
}

func myFileDir() string {
    dirName, err := filepath.Abs(filepath.Dir(os.Args[0]))
    failOnError(err)
    return dirName
}


func main() {
    failWhen(len(os.Args) < 2,"docker-slim: error => missing image info")

    imageId := os.Args[1]

    doRemoveImageArtifacts := false
    if (len(os.Args) > 2) && (os.Args[2] == "rm-artifacts") {
        doRemoveImageArtifacts = true
    }

    client, _ := docker.NewClientFromEnv()
    
    log.Println("docker-slim: inspecting \"fat\" image...")
    imageInfo, err := client.InspectImage(imageId)
    if err != nil {
    	if err == docker.ErrNoSuchImage {
    		log.Fatalf("docker-slim: could not find target image")
    	}
        log.Fatalf("docker-slim: InspectImage(%v) error => %v",imageId,err)
    }
    
    log.Println("IMAGE ID:",imageInfo.ID)

    var imageRecord docker.APIImages
    imageList, err := client.ListImages(docker.ListImagesOptions{All:true})
    failOnError(err)
    for _,r := range imageList {
        if r.ID == imageInfo.ID {
            imageRecord = r
            break
        }
    }

    if imageRecord.ID == "" {
        log.Fatalf("docker-slim: could not find target image in the image list")
    }

    slimImageRepo := "slim"
    if len(imageRecord.RepoTags) > 0 {
        if rtInfo := strings.Split(imageRecord.RepoTags[0], ":"); len(rtInfo) > 1 {
            slimImageRepo = fmt.Sprintf("%s.slim",rtInfo[0])
        }
    }

    log.Printf("docker-slim: \"fat\" image size => %v (%v)\n",
        imageInfo.VirtualSize,humanize.Bytes(uint64(imageInfo.VirtualSize)))

    imageMeta := struct {
        RepoName     string
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
        fatContainerCmd = append(fatContainerCmd,imageInfo.Config.Entrypoint...) 
    }

    if len(imageInfo.Config.Cmd) > 0 {
        fatContainerCmd = append(fatContainerCmd,imageInfo.Config.Cmd...) 
    }

    localVolumePath := fmt.Sprintf("%s/container",myFileDir())
    artifactLocation := fmt.Sprintf("%v/artifacts",localVolumePath)

    artifactDir, err := os.Stat(artifactLocation)
    if os.IsNotExist(err) {
        os.MkdirAll(artifactLocation,0777)
    }
    
    failWhen(!artifactDir.IsDir(),"artifact location is not a directory")
    
    mountInfo := fmt.Sprintf("%s:/opt/dockerslim",localVolumePath)

    containerOptions := docker.CreateContainerOptions{
        Name: "dockerslimk",
        Config: &docker.Config {
            Image: imageId,
            // NOTE: specifying Mounts here doesn't work :)
            //Mounts: []docker.Mount{{
            //        Source: localVolumePath,
            //        Destination: "/opt/dockerslim",
            //        Mode: "",
            //        RW: true,
            //    },
            //},
            Entrypoint: []string{"/opt/dockerslim/bin/alauncher"},
            Cmd: fatContainerCmd,
            Labels: map[string]string{"type":"dockerslim"},
        },
        HostConfig: &docker.HostConfig {
            Binds: []string{mountInfo},
            PublishAllPorts: true,
            CapAdd: []string{"SYS_ADMIN"},
            Privileged: true,
        },
    }

    log.Println("docker-slim: creating instrumented \"fat\" container...")
    containerInfo, err := client.CreateContainer(containerOptions)
    failOnError(err)
    log.Println("docker-slim: created container =>",containerInfo.ID)

    log.Println("docker-slim: starting \"fat\" container...")
    
    err = client.StartContainer(containerInfo.ID, &docker.HostConfig{
        PublishAllPorts: true,
        CapAdd: []string{"SYS_ADMIN"},
        Privileged: true,
    })
    failOnError(err)

    //TODO: keep checking the monitor state until no new files (and processes) are discovered
    log.Println("docker-slim: watching container monitor...")
    endTime := time.After(time.Second * 130)
    work := 0
    doneWatching:
    for {
        select {
            case <- endTime:
                log.Println("docker-slim: done with work!")
                break doneWatching
            case <- time.After(time.Second * 3):
                work++
                log.Println("docker-slim: still watching =>", work)
        }
    }

    //log.Println("docker-slim: exporting \"fat\" container artifacts...")
    //time.Sleep(5 * time.Second)

    log.Println("docker-slim: stopping \"fat\" container...")
    err = client.StopContainer(containerInfo.ID, 9)
    warnOnError(err)

    log.Println("docker-slim: removing \"fat\" container...")
    removeOption := docker.RemoveContainerOptions {
        ID: containerInfo.ID,
        RemoveVolumes: true,
        Force: true,
    }
    err = client.RemoveContainer(removeOption)
    warnOnError(err)

    log.Println("docker-slim: creating \"slim\" image...")

    dockerfileLocation := fmt.Sprintf("%v/Dockerfile",artifactLocation)

    var dfData bytes.Buffer
    dfData.WriteString("FROM scratch\n")
    dfData.WriteString("COPY files /\n")

    dfData.WriteString("WORKDIR ")
    dfData.WriteString(imageMeta.WorkingDir)
    dfData.WriteByte('\n')

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
            quotedEntryPoint = append(quotedEntryPoint,strconv.Quote(imageMeta.Entrypoint[idx]))
        }
        dfData.WriteString("ENTRYPOINT [")
        dfData.WriteString(strings.Join(quotedEntryPoint,","))
        dfData.WriteByte(']')
        dfData.WriteByte('\n')
    }

    if len(imageMeta.Cmd) > 0 {
        var quotedCmd []string
        for idx := range imageMeta.Entrypoint {
            quotedCmd = append(quotedCmd,strconv.Quote(imageMeta.Cmd[idx]))
        }
        dfData.WriteString("CMD [")
        dfData.WriteString(strings.Join(quotedCmd,","))
        dfData.WriteByte(']')
        dfData.WriteByte('\n')
    }

    err = ioutil.WriteFile(dockerfileLocation,dfData.Bytes(),0644)
    failOnError(err)

    buildOptions := docker.BuildImageOptions {
        Name: imageMeta.RepoName,
        RmTmpContainer: true,
        ContextDir: artifactLocation,
        Dockerfile: "Dockerfile",
        OutputStream: os.Stdout,
    }

    err = client.BuildImage(buildOptions)
    failOnError(err)
    log.Println("docker-slim: created new image:",imageMeta.RepoName)

    if doRemoveImageArtifacts {
        log.Println("docker-slim: removing temporary artifacts (TODO)...")
        err = os.RemoveAll(artifactLocation)
        warnOnError(err)
    }
}

// eval "$(docker-machine env default)"
// ./dockerslim 6f74095b68c9
