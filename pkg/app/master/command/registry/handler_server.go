package registry

import (
	"crypto/tls"
	"errors"
	"fmt"
	"golang.org/x/crypto/acme/autocert"
	stdlog "log"
	"net/http"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/app/master/version"
	cmd "github.com/slimtoolkit/slim/pkg/command"
	//"github.com/slimtoolkit/slim/pkg/docker/dockerclient"
	"github.com/slimtoolkit/slim/pkg/report"
	//"github.com/slimtoolkit/slim/pkg/util/fsutil"
	//v "github.com/slimtoolkit/slim/pkg/version"
)

// OnServerCommand implements the 'registry server' command
func OnServerCommand(
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	cparams *ServerCommandParams) {
	cmdName := fullCmdName(ServerCmdName)
	logger := log.WithFields(log.Fields{
		"app": appName,
		"cmd": cmdName,
		"sub": ServerCmdName})

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewRegistryCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = cmd.StateStarted

	xc.Out.State(cmd.StateStarted)

	var client *docker.Client
	/* NOTE: don't really need a docker client for the server...

	client, err := dockerclient.New(gparams.ClientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if gparams.InContainer && gparams.IsDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
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
	*/

	if gparams.Debug {
		version.Print(xc, cmdName, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	opts := []registry.Option{registry.Logger(stdlog.New(logger.Logger.Out, "", stdlog.LstdFlags))}
	if cparams.ReferrersAPI {
		opts = append(opts, registry.WithReferrersSupport(true))
	}

	//TODO: add the custom blob handler logic
	if cparams.UseMemStore {
		//bh := registry.NewInMemoryBlobHandler()
	} else {
		//cparams.StorePath
		//bh = registry.NewDiskBlobHandler(diskp)
		//opts = append(opts, registry.WithBlobHandler(bh))
	}

	//TODO: wrap http server to record the calls and save them in the report
	go func() {
		time.Sleep(3 * time.Second)

		xc.Out.State(cmd.StateCompleted)
		cmdReport.State = cmd.StateCompleted
		xc.Out.State(cmd.StateDone)

		vinfo := <-viChan
		version.PrintCheckVersion(xc, "", vinfo)

		cmdReport.State = cmd.StateDone
		if cmdReport.Save() {
			xc.Out.Info("report",
				ovars{
					"file": cmdReport.ReportLocation(),
				})
		}
	}()

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cparams.Address, cparams.Port),
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 4 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		Handler:           registry.New(opts...),
	}

	var err error
	if cparams.UseHTTPS {
		server.TLSConfig = &tls.Config{
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
			CurvePreferences: []tls.CurveID{
				tls.CurveP256,
				tls.X25519,
			},
		}

		if cparams.Domain != "" &&
			cparams.CertPath == "" &&
			cparams.KeyPath == "" {
			certManager := autocert.Manager{
				Prompt: autocert.AcceptTOS,
				//TODO: needs to put it as a sub-dir in the state path
				Cache:      autocert.DirCache(".mint_certs"),
				HostPolicy: autocert.HostWhitelist(cparams.Domain),
			}

			server.TLSConfig.GetCertificate = certManager.GetCertificate
		}

		err = server.ListenAndServeTLS(cparams.CertPath, cparams.KeyPath)
	} else {
		err = server.ListenAndServe()
	}

	if errors.Is(err, http.ErrServerClosed) {
		xc.Out.Message("Server is done...")
	} else {
		xc.FailOn(err)
	}
}
