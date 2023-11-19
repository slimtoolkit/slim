package container

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/moby/term"
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/config"
)

//Custom I/O for terminal later

const (
	ContainerNamePat = "ds.run_%v_%v"
)

type ovars = app.OutVars

type ExecutionState string

const (
	XSNone        ExecutionState = "xs.none"
	XSCreated                    = "xs.created"
	XSStarted                    = "xs.started"
	XSStopping                   = "xs.stopping"
	XSStopped                    = "xs.stopped"
	XSRemoved                    = "xs.removed"
	XSExited                     = "xs.exited"
	XSExitedCrash                = "xs.exited.crash"
	XSError                      = "xs.error"
)

type ExecutionEvent string

const (
	XECreated     ExecutionEvent = "xe.container.created"
	XEStarted                    = "xe.container.started"
	XEStopping                   = "xe.container.stopping"
	XEStopped                    = "xe.container.stopped"
	XERemoved                    = "xe.container.removed"
	XEExited                     = "xe.container.exited"
	XEExitedCrash                = "xe.container.exited.crash"
	XEAPIError                   = "xe.api.error"
	XEInterrupt                  = "xe.interrupt"
)

type ExecutionEvenInfo struct {
	Event ExecutionEvent
	Data  map[string]string
}

type VolumeInfo struct {
	Source      string
	Destination string
	Options     string
}

type ExecutionOptions struct {
	ContainerName string
	Entrypoint    []string
	Cmd           []string
	PublishPorts  map[dockerapi.Port][]dockerapi.PortBinding
	EnvVars       []string
	Volumes       []config.VolumeMount
	LiveLogs      bool
	Terminal      bool
	IO            ExecutionIO
}

type ExecutionIO struct {
	Input  io.Reader
	Output io.Writer
	Error  io.Writer
}

type Execution struct {
	ContainerInfo *dockerapi.Container
	ContainerName string
	ContainerID   string
	State         ExecutionState
	Crashed       bool
	StopTimeout   uint

	/// the following fields are forwarded to dockerapi.HostConfig and take
	/// the same parameters as docker's `--pid`, `--network` and `--ipc` CLI
	/// flags
	PidMode     string
	NetworkMode string
	IpcMode     string

	imageRef          string
	APIClient         *dockerapi.Client
	options           *ExecutionOptions
	cleanupOnSysExit  bool
	eventCh           chan *ExecutionEvenInfo
	printState        bool
	dockerEventCh     chan *dockerapi.APIEvents
	dockerEventStopCh chan struct{}
	xc                *app.ExecutionContext
	logger            *log.Entry
	terminalExitChan  chan error
	termFd            uintptr
	isInterrupted     bool
}

const defaultStopTimeout = 7 //7 seconds

func NewExecution(
	xc *app.ExecutionContext,
	logger *log.Entry,
	client *dockerapi.Client,
	imageRef string,
	options *ExecutionOptions,
	eventCh chan *ExecutionEvenInfo,
	cleanupOnSysExit bool,
	printState bool) (*Execution, error) {
	if logger != nil {
		logger = logger.WithFields(log.Fields{"com": "container.execution"})
	}

	exe := &Execution{
		State:             XSNone,
		StopTimeout:       defaultStopTimeout,
		imageRef:          imageRef,
		APIClient:         client,
		options:           options,
		eventCh:           eventCh,
		cleanupOnSysExit:  cleanupOnSysExit,
		printState:        printState,
		xc:                xc,
		logger:            logger,
		dockerEventCh:     make(chan *dockerapi.APIEvents),
		dockerEventStopCh: make(chan struct{}),
	}

	//allow liveLogs only if terminal is not enabled
	if exe.options != nil && exe.options.Terminal {
		exe.options.LiveLogs = false
	}

	return exe, nil
}

// Start starts a new container execution
func (ref *Execution) Start() error {
	if ref.options.ContainerName != "" {
		//we have an explicitly provided container name to use
		//ideally we first validate that this name can be used...
		ref.ContainerName = ref.options.ContainerName
	} else {
		ref.ContainerName = fmt.Sprintf(ContainerNamePat, os.Getpid(), time.Now().UTC().Format("20060102150405"))
	}

	hostConfig := &dockerapi.HostConfig{
		NetworkMode: ref.NetworkMode,
		PidMode:     ref.PidMode,
		IpcMode:     ref.IpcMode,
	}

	containerOptions := dockerapi.CreateContainerOptions{
		Name: ref.ContainerName,
		Config: &dockerapi.Config{
			Image: ref.imageRef,
		},
		HostConfig: hostConfig,
	}

	if ref.options != nil {
		if len(ref.options.Entrypoint) > 0 {
			containerOptions.Config.Entrypoint = ref.options.Entrypoint
		}

		if len(ref.options.Cmd) > 0 {
			containerOptions.Config.Cmd = ref.options.Cmd
		}

		if len(ref.options.EnvVars) > 0 {
			containerOptions.Config.Env = ref.options.EnvVars
		}

		if len(ref.options.Volumes) > 0 {
			mounts := []dockerapi.HostMount{}
			for _, vol := range ref.options.Volumes {
				mount := dockerapi.HostMount{
					Target: vol.Destination,
				}

				if vol.Source == "" {
					mount.Type = "volume"
				} else {
					if strings.HasPrefix(vol.Source, "/") {
						mount.Source = vol.Source
						mount.Type = "bind"
					} else if strings.HasPrefix(vol.Source, "~/") {
						hd, _ := os.UserHomeDir()
						mount.Source = filepath.Join(hd, vol.Source[2:])
						mount.Type = "bind"
					} else if strings.HasPrefix(vol.Source, "./") ||
						strings.HasPrefix(vol.Source, "../") ||
						(vol.Source == "..") ||
						(vol.Source == ".") {
						mount.Source, _ = filepath.Abs(vol.Source)
						mount.Type = "bind"
					} else {
						//todo: list volumes and check vol.Source instead of defaulting to named volume
						mount.Source = vol.Source
						mount.Type = "volume"
					}
				}

				if vol.Options == "ro" {
					mount.ReadOnly = true
				}

				mounts = append(mounts, mount)
			}
			containerOptions.HostConfig.Mounts = mounts
		}

		if len(ref.options.PublishPorts) > 0 {
			containerOptions.HostConfig.PortBindings = ref.options.PublishPorts
		}
	}

	if ref.options != nil && ref.options.Terminal {
		fmt.Println("adding more container params for Terminal")
		containerOptions.Config.OpenStdin = true
		//containerOptions.Config.StdinOnce = true
		containerOptions.Config.AttachStdin = true
		containerOptions.Config.Tty = true
	}

	containerInfo, err := ref.APIClient.CreateContainer(containerOptions)
	if err != nil {
		return err
	}

	ref.State = XSCreated

	if ref.ContainerName != containerInfo.Name {
		if ref.logger != nil {
			ref.logger.Debugf("RunContainer: Container name mismatch expected=%v got=%v",
				ref.ContainerName, ref.ContainerName)
		}
	}

	ref.ContainerID = containerInfo.ID

	if ref.printState && ref.xc != nil {
		ref.xc.Out.Info("container",
			ovars{
				"status": "created",
				"name":   ref.ContainerName,
				"id":     ref.ContainerID,
			})

		if ref.logger != nil {
			ref.logger.Tracef("container created: name=%s id=%s",
				ref.ContainerName, ref.ContainerID)
		}
	}

	if ref.eventCh != nil {
		ref.eventCh <- &ExecutionEvenInfo{
			Event: XECreated,
		}
	}

	go ref.monitorContainerExitSync()

	if ref.cleanupOnSysExit {
		go ref.monitorSysExitSync()
	}

	if ref.options != nil {
		if ref.options.Terminal {
			var oldState *term.State
			var isTerminal bool
			ref.termFd, isTerminal = term.GetFdInfo(os.Stdout)
			if !isTerminal {
				return errors.New("not a terminal")
			}

			oldState, err = term.SetRawTerminal(ref.termFd)
			if err != nil {
				return err
			}

			defer term.RestoreTerminal(ref.termFd, oldState)

			ref.terminalExitChan = make(chan error)

			go ref.startTerminal()
		} else if ref.options.LiveLogs {
			go ref.startLiveLogs()
		}
	}

	if err := ref.APIClient.StartContainer(ref.ContainerID, nil); err != nil {
		ref.State = "error"
		return err
	}

	ref.State = XSStarted

	if ref.ContainerInfo, err = ref.APIClient.InspectContainer(ref.ContainerID); err != nil {
		ref.State = XSError
		if ref.eventCh != nil {
			ref.eventCh <- &ExecutionEvenInfo{
				Event: XEAPIError,
				Data: map[string]string{
					"error": err.Error(),
				},
			}
		}

		return err
	}

	if ref.printState && ref.xc != nil {
		ref.xc.Out.Info("container",
			ovars{
				"status": "started",
				"name":   containerInfo.Name,
				"id":     ref.ContainerID,
			})

		if ref.logger != nil {
			ref.logger.Tracef("container started = name=%s id=%s\n",
				ref.ContainerName, ref.ContainerID)
		}
	}

	if ref.eventCh != nil {
		ref.eventCh <- &ExecutionEvenInfo{
			Event: XEStarted,
		}
	}

	if ref.options != nil && ref.options.Terminal {
		go ref.monitorTerminalSizeSync()

		if ref.terminalExitChan != nil {
			attachErr := <-ref.terminalExitChan
			return attachErr
		}
	}

	return nil
}

// Stop stops the container execution
func (ref *Execution) Stop() error {
	ref.State = XSStopping
	if ref.eventCh != nil {
		ref.eventCh <- &ExecutionEvenInfo{
			Event: XEStopping,
		}
	}

	err := ref.APIClient.StopContainer(ref.ContainerID, ref.StopTimeout)

	if err != nil {
		if _, ok := err.(*dockerapi.ContainerNotRunning); ok {
			if ref.logger != nil {
				ref.logger.Info("can't stop the 'slim' container (container is not running)...")
			}
		} else {
			if ref.logger != nil {
				ref.logger.Infof("Execution.Stop: apiClient.StopContainer error - %v", err)
			}
		}
	}

	ref.State = XSStopped
	if ref.eventCh != nil {
		ref.eventCh <- &ExecutionEvenInfo{
			Event: XEStopped,
		}
	}

	return err
}

// Cleanup removes stopped container for the execution
func (ref *Execution) Cleanup() error {
	removeOption := dockerapi.RemoveContainerOptions{
		ID:            ref.ContainerID,
		RemoveVolumes: true,
		Force:         true,
	}

	err := ref.APIClient.RemoveContainer(removeOption)
	if err != nil {
		if ref.logger != nil {
			ref.logger.Info("error removing container =>", err)
		}
	}

	ref.State = XSRemoved
	if ref.eventCh != nil {
		ref.eventCh <- &ExecutionEvenInfo{
			Event: XERemoved,
		}
	}

	return err
}

// Wait waits the container execution
func (ref *Execution) Wait() (int, error) {
	//TODO: use WaitContainerWithContext
	return ref.APIClient.WaitContainer(ref.ContainerID)
}

func (ref *Execution) monitorContainerExitSync() {
	ref.APIClient.AddEventListener(ref.dockerEventCh)

	for {
		select {
		case devent := <-ref.dockerEventCh:
			if devent == nil || devent.ID == "" || devent.Status == "" {
				break
			}

			if devent.ID == ref.ContainerID {
				if devent.Status == "die" {
					ref.State = XSExited

					exitEvent := &ExecutionEvenInfo{
						Event: XEExited,
					}

					nonZeroExitCode := false
					exitCodeStr, ok := devent.Actor.Attributes["exitCode"]
					if ok && exitCodeStr != "" && exitCodeStr != "0" {
						nonZeroExitCode = true
					}

					if nonZeroExitCode {
						if ref.isInterrupted && exitCodeStr == "137" {
							if ref.logger != nil {
								ref.logger.Tracef("container interrupted (expected) = %s", ref.ContainerID)
							}
						} else {
							ref.State = XSExitedCrash
							ref.Crashed = true
							if ref.printState && ref.xc != nil {
								ref.xc.Out.Info("container",
									ovars{
										"status":    "crashed",
										"id":        ref.ContainerID,
										"exit.code": exitCodeStr,
									})

								if ref.logger != nil {
									ref.logger.Tracef("container crashed = %s", ref.ContainerID)
								}
							}

							exitEvent.Event = XEExitedCrash
						}
					}

					if exitEvent.Event == XEExited {
						if ref.printState && ref.xc != nil {
							ref.xc.Out.Info("container",
								ovars{
									"status":    "exited",
									"id":        ref.ContainerID,
									"exit.code": exitCodeStr,
								})

							if ref.logger != nil {
								ref.logger.Tracef("container exited = %s", ref.ContainerID)
							}
						}
					}

					if ref.eventCh != nil {
						ref.eventCh <- exitEvent
					}
				}
			}

		case <-ref.dockerEventStopCh:
			if ref.logger != nil {
				ref.logger.Debug("container.Execution.monitorContainerExitSync: Docker event monitor stopped")
			}
			return
		}
	}
}

func (ref *Execution) monitorSysExitSync() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT)

	<-signals
	ref.isInterrupted = true
	//_ = ref.APIClient.KillContainer(dockerapi.KillContainerOptions{ID: ref.ContainerID})
	//if ref.logger != nil {
	//	ref.logger.Debugf("Execution.monitorSysExitSync: received SIGINT, killing container %s", ref.ContainerID)
	//}

	if ref.eventCh != nil {
		ref.eventCh <- &ExecutionEvenInfo{
			Event: XEInterrupt,
		}
	}

	err := ref.Stop()
	if err != nil {
		if ref.logger != nil {
			ref.logger.Debugf("ref.Stop error: id=%s err=%v", ref.ContainerID, err)
		}
	}
}

func (ref *Execution) startTerminal() {
	r, w := io.Pipe()
	go io.Copy(w, os.Stdin)
	options := dockerapi.AttachToContainerOptions{
		Container:    ref.ContainerID,
		InputStream:  r,
		OutputStream: os.Stdout,
		ErrorStream:  os.Stderr,
		Stdin:        true,
		Stdout:       true,
		Stderr:       true,
		Stream:       true,
		RawTerminal:  true,
		Logs:         true,
	}

	err := ref.APIClient.AttachToContainer(options)
	ref.terminalExitChan <- err
}

func (ref *Execution) startLiveLogs() {
	options := dockerapi.AttachToContainerOptions{
		Container:    ref.ContainerID,
		OutputStream: os.Stdout,
		ErrorStream:  os.Stderr,
		Stdin:        false,
		Stdout:       true,
		Stderr:       true,
		Logs:         true,
		Stream:       true,
		RawTerminal:  true,
	}

	if ref.options != nil {
		if ref.options.IO.Output != nil {
			options.OutputStream = ref.options.IO.Output
		}

		if ref.options.IO.Error != nil {
			options.ErrorStream = ref.options.IO.Error
		}
	}

	err := ref.APIClient.AttachToContainer(options)
	if err != nil {
		panic(err)
	}
}

func (ref *Execution) ShowContainerLogs() {
	var outData bytes.Buffer
	outw := bufio.NewWriter(&outData)
	var errData bytes.Buffer
	errw := bufio.NewWriter(&errData)

	if ref.logger != nil {
		ref.logger.Debug("getting container logs => ", ref.ContainerID)
	}

	logsOptions := dockerapi.LogsOptions{
		Container:    ref.ContainerID,
		OutputStream: outw,
		ErrorStream:  errw,
		Stdout:       true,
		Stderr:       true,
	}

	err := ref.APIClient.Logs(logsOptions)
	if err != nil {
		if ref.logger != nil {
			ref.logger.Infof("error getting container logs => %v - %v", ref.ContainerID, err)
		}
	} else {
		outw.Flush()
		errw.Flush()
		fmt.Printf("[%s] CONTAINER STDOUT:\n", ref.ContainerID)
		outData.WriteTo(os.Stdout)
		fmt.Printf("[%s] CONTAINER STDERR:\n", ref.ContainerID)
		errData.WriteTo(os.Stdout)
		fmt.Printf("[%s] END OF CONTAINER LOGS =============\n", ref.ContainerID)
	}
}

func (ref *Execution) monitorTerminalSizeSync() {
	ref.updateTerminalSize()

	winchCh := make(chan os.Signal, 1)
	signal.Notify(winchCh, syscall.SIGWINCH)
	defer signal.Stop(winchCh)

	for range winchCh {
		ref.updateTerminalSize()
	}
}

func (ref *Execution) updateTerminalSize() error {
	height, width := terminalSize(ref.termFd)
	if height == 0 && width == 0 {
		return nil
	}
	return ref.APIClient.ResizeContainerTTY(ref.ContainerID, height, width)
}

func terminalSize(fd uintptr) (int, int) {
	ws, err := term.GetWinsize(fd)
	if err != nil {
		if ws == nil {
			return 0, 0
		}
	}

	return int(ws.Height), int(ws.Width)
}
