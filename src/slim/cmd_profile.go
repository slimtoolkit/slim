package main

import (
	"fmt"
)

func onProfileCommand(imageRef string) {
	fmt.Println("docker-slim: [profile] image=", imageRef)

	fmt.Println("docker-slim: [profile] done.")
}
