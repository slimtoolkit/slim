package commands

import (
	"fmt"
)

func OnProfile(imageRef string) {
	fmt.Println("docker-slim: [profile] image=", imageRef)

	fmt.Println("docker-slim: [profile] done.")
}
