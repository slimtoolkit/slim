package app

import (
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app/master/signals"
)

// Run starts the master app
func Run() {
	signals.InitHandlers()
	cli := newCLI()
	if err := cli.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
