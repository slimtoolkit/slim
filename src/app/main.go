package main

import (
    "log"

    "github.com/fsouza/go-dockerclient"
)

func main() {
    log.Println("docker-slim app...")

    client, _ := docker.NewClientFromEnv()
    info, _ := client.Info()
    log.Println("info:",info)
}


