package main

import (
	"os"

	"github.com/slimtoolkit/slim/pkg/app/master"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "slim" {
		//hack to handle plugin invocations
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	}

	app.Run()
}
