package main

import (
    "time"
    "fmt"
    "log"
    "os"
    "path/filepath"

    "github.com/fsouza/go-dockerclient"
    "github.com/dustin/go-humanize"
)

func failOnError(err error) {
    if err != nil {
        log.Fatalln("ERROR =>",err)
    }
}

func warnOnError(err error) {
    if err != nil {
        log.Println("ERROR =>",err)
    }
}

func failWhen(cond bool,msg string) {
    if cond {
        log.Fatalln("ERROR =>",msg)
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

    client, _ := docker.NewClientFromEnv()
    
    log.Println("docker-slim: inspecting \"fat\" image...")
    imageInfo, err := client.InspectImage(imageId)
    if err != nil {
    	if err == docker.ErrNoSuchImage {
    		log.Fatalf("docker-slim: could not find target image")
    	}
        log.Fatalf("docker-slim: InspectImage(%v) error => %v",imageId,err)
    }

    log.Printf("docker-slim: \"fat\" image size => %v (%v)\n",
        imageInfo.VirtualSize,humanize.Bytes(uint64(imageInfo.VirtualSize)))

    imageMeta := struct {
        Entrypoint   []string
        Cmd          []string
        WorkingDir   string
        Env          []string
        ExposedPorts map[docker.Port]struct{}
        Volumes      map[string]struct{}
        OnBuild      []string
        User         string
    }{
        imageInfo.Config.Entrypoint,
        imageInfo.Config.Cmd,
        imageInfo.Config.WorkingDir,
        imageInfo.Config.Env,
        imageInfo.Config.ExposedPorts,
        imageInfo.Config.Volumes,
        imageInfo.Config.OnBuild,
        imageInfo.Config.User,
    }

    _ = imageMeta

    var fatContainerCmd []string
    if len(imageInfo.Config.Entrypoint) > 0 {
        fatContainerCmd = append(fatContainerCmd,imageInfo.Config.Entrypoint...) 
    }

    if len(imageInfo.Config.Cmd) > 0 {
        fatContainerCmd = append(fatContainerCmd,imageInfo.Config.Cmd...) 
    }
    
    localVolumePath := fmt.Sprintf("%s/container",myFileDir())
    mountInfo := fmt.Sprintf("%s:/opt/dockerslim",localVolumePath)
    
    containerOptions := docker.CreateContainerOptions{
        Name: "dockerslimk",
        Config: &docker.Config {
            Image: imageId,
            /* NOTE: specifying Mounts here doesn't work :)
            Mounts: []docker.Mount{{
                    Source: localVolumePath,
                    Destination: "/opt/dockerslim",
                    Mode: "",
                    RW: true,
                },
            },
            */
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
    //NOTE: still need to set PublishAllPorts here to map the ports...
    err = client.StartContainer(containerInfo.ID, &docker.HostConfig{
        PublishAllPorts: true,
        CapAdd: []string{"SYS_ADMIN"},
        Privileged: true,
    })
    failOnError(err)

    //keep checking the monitor state until no new files (and processes) are discovered
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

    log.Println("docker-slim: exporting \"fat\" container artifacts...")
    time.Sleep(5 * time.Second)

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

    log.Println("docker-slim: packaging \"slim\" container artifacts (TODO)...")
    time.Sleep(5 * time.Second)

    log.Println("docker-slim: creating \"slim\" image (TODO)...")
    time.Sleep(5 * time.Second)

    log.Println("docker-slim: removing temporary artifacts...")
    time.Sleep(5 * time.Second)
}

// eval "$(docker-machine env default)"
// ./dockerslim 6f74095b68c9


