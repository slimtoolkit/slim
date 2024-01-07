package images

import (
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jedib0t/go-pretty/v6/table"
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/app/master/version"
	cmd "github.com/slimtoolkit/slim/pkg/command"
	"github.com/slimtoolkit/slim/pkg/docker/dockerclient"
	"github.com/slimtoolkit/slim/pkg/docker/dockerutil"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
	"github.com/slimtoolkit/slim/pkg/util/jsonutil"
	v "github.com/slimtoolkit/slim/pkg/version"
)

const appName = command.AppName

type ovars = app.OutVars

// OnCommand implements the 'images' command
func OnCommand(
	xc *app.ExecutionContext,
	gparams *command.GenericParams) {
	const cmdName = Name
	logger := log.WithFields(log.Fields{"app": appName, "cmd": cmdName})

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewImagesCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = cmd.StateStarted

	xc.Out.State("started")
	xc.Out.Info("params",
		ovars{
			//"target": targetRef, - todo: add command params here when added
		})

	client, err := dockerclient.New(gparams.ClientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if gparams.InContainer && gparams.IsDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the slim app container"
		}

		xc.Out.Info("docker.connect.error",
			ovars{
				"message": exitMsg,
			})

		exitCode := command.ECTCommon | command.ECCNoDockerConnectInfo
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})
		xc.Exit(exitCode)
	}
	xc.FailOn(err)

	if gparams.Debug {
		version.Print(xc, cmdName, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	images, err := dockerutil.ListImages(client, "")
	xc.FailOn(err)

	if xc.Out.Quiet {
		if xc.Out.OutputFormat == command.OutputFormatJSON {
			fmt.Printf("%s\n", jsonutil.ToPretty(images))
			return
		}

		printImagesTable(images)
		return
	} else {
		for name, info := range images {
			fields := ovars{
				"name":    name,
				"id":      info.ID,
				"size":    humanize.Bytes(uint64(info.Size)),
				"created": time.Unix(info.Created, 0).Format(time.RFC3339),
			}

			xc.Out.Info("image", fields)
		}
	}

	xc.Out.State("completed")
	cmdReport.State = cmd.StateCompleted
	xc.Out.State("done")

	vinfo := <-viChan
	version.PrintCheckVersion(xc, "", vinfo)

	cmdReport.State = cmd.StateDone
	if cmdReport.Save() {
		xc.Out.Info("report",
			ovars{
				"file": cmdReport.ReportLocation(),
			})
	}
}

func printImagesTable(images map[string]dockerutil.BasicImageProps) {
	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"Name", "ID", "Size", "Created"})

	for name, info := range images {
		tw.AppendRow(table.Row{
			name,
			info.ID,
			humanize.Bytes(uint64(info.Size)),
			time.Unix(info.Created, 0).Format(time.RFC3339),
		})
	}

	tw.SetStyle(table.StyleLight)
	tw.Style().Options.DrawBorder = false
	fmt.Printf("%s\n", tw.Render())
}
