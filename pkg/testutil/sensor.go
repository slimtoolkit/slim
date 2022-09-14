package testutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/inspectors/ipc"
	"github.com/docker-slim/docker-slim/pkg/app/master/inspectors/sensor"
	"github.com/docker-slim/docker-slim/pkg/ipc/channel"
	"github.com/docker-slim/docker-slim/pkg/ipc/command"
	"github.com/docker-slim/docker-slim/pkg/ipc/event"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
)

const (
	commandsFileName = "commands.json"
)

var (
	errNotStarted error = errors.New("test sensor container hasn't been started yet")
)

type Sensor struct {
	image          dockerapi.Image
	contName       string
	sensorExePath  string
	contextDirPath string

	// "Nullable"
	contID  string
	client  *ipc.Client
	creport *report.ContainerReport
	stopped bool
}

func NewSensor(
	ctx context.Context,
	contextDirPath,
	contName string,
	imageName string,
) (*Sensor, error) {
	sensorExePath, err := exec.LookPath(sensor.LocalBinFile)
	if err != nil {
		return nil, fmt.Errorf("cannot locate %s executable on the host system", sensor.LocalBinFile)
	}

	if err := imagePull(ctx, imageName); err != nil {
		return nil, fmt.Errorf("cannot pull image %q: %w", imageName, err)
	}

	image, err := imageInspect(ctx, imageName)
	if err != nil {
		return nil, fmt.Errorf("cannot inspect image %q: %w", imageName, err)
	}

	log.
		WithField("image", imageName).
		WithField("context", contextDirPath).
		WithField("exe", sensorExePath).
		Debug("New test sensor created")

	return &Sensor{
		image:          image,
		contName:       strings.ToLower(contName),
		sensorExePath:  sensorExePath,
		contextDirPath: contextDirPath,
	}, nil
}

func NewSensorOrFail(
	t *testing.T,
	ctx context.Context,
	contextDirPath,
	contName string,
	imageName string,
) *Sensor {
	s, err := NewSensor(ctx, contextDirPath, contName, imageName)
	if err != nil {
		t.Fatal("Cannot initialize sensor:", err)
	}
	return s
}

func (s *Sensor) StartControlled(ctx context.Context) error {
	log.Debug("Starting test sensor (controlled mode)...")

	contID, err := containerCreate(
		ctx,
		[]string{
			"--name", s.contName,
			"--cap-add", "ALL",
			"--user", "root",
			"--volume", s.sensorExePath + ":/opt/dockerslim/sensor",
			"--publish", fmt.Sprintf("%d", channel.CmdPort),
			"--publish", fmt.Sprintf("%d", channel.EvtPort),
			"--entrypoint", "/opt/dockerslim/sensor",
		},
		s.image.ID,
		"-l", "debug",
		"-d",
	)
	if err != nil {
		return fmt.Errorf("cannot create target container (controlled mode): %w", err)
	}

	log.WithField("containerId", contID).Debug("Test sensor container created (controlled mode)")
	s.contID = contID

	if err := containerStart(ctx, s.contID); err != nil {
		return fmt.Errorf("cannot start target container (controlled mode): %w", err)
	}

	log.WithField("containerId", contID).Debug("Test sensor container started (controlled mode)")

	cont, err := containerInspect(ctx, s.contID)
	if err != nil {
		return fmt.Errorf("cannot inspect container %q: %w", s.contID, err)
	}

	cmdPort, ok := hostPort(cont, channel.CmdPort)
	if !ok {
		return fmt.Errorf("container %q - no host port found for port %q", s.contID, channel.CmdPort)
	}
	evtPort, ok := hostPort(cont, channel.EvtPort)
	if !ok {
		return fmt.Errorf("container %q - no host port found for port %q", s.contID, channel.EvtPort)
	}

	// TODO: Refactor the IPC code to use context with a deadline.
	client, err := ipc.NewClient("127.0.0.1", cmdPort, evtPort, 10) // Seconds, I guess
	if err != nil {
		return fmt.Errorf("cannot start IPC client: %w", err)
	}

	log.
		WithField("containerId", contID).
		Debug("IPC client connected to the target container")
	s.client = client

	return nil
}

func (s *Sensor) StartControlledOrFail(t *testing.T, ctx context.Context) {
	if err := s.StartControlled(ctx); err != nil {
		t.Fatal("Cannot start sensor (controlled mode):", err)
	}
}

func (s *Sensor) StartStandalone(
	ctx context.Context,
	runArgs []string,
	cmdOverride ...command.StartMonitor,
) error {
	cmd := startCommandStandalone(s.image, cmdOverride...)
	log.
		WithField("command", fmt.Sprintf("%+v", cmd)).
		Debug("Starting test sensor (standalone mode)...")

	commandsFilePath := filepath.Join(s.contextDirPath, commandsFileName)
	if err := jsonDump(commandsFilePath, cmd); err != nil {
		return fmt.Errorf("cannot create commands.json file: %w", err)
	}

	stopSignal := "SIGTERM"
	if len(s.image.Config.StopSignal) > 0 {
		stopSignal = s.image.Config.StopSignal
	}

	stopTimeout := 5 * time.Second
	if s.image.Config.StopTimeout != 0 {
		// TODO: Make sure we never pass 0s.
		stopTimeout = time.Duration(s.image.Config.StopTimeout/2) * time.Second
	}

	contID, err := containerCreate(
		ctx,
		[]string{
			"--name", s.contName,
			"--cap-add", "ALL",
			"--user", "root",
			"--volume", s.sensorExePath + ":/opt/dockerslim/sensor",
			"--volume", commandsFilePath + ":/opt/dockerslim/commands.json",
			"--entrypoint", "/opt/dockerslim/sensor",
		},
		s.image.ID,
		append(
			[]string{
				"-m", "standalone",
				"-c", "/opt/dockerslim/commands.json",
				"-s", stopSignal,
				"-w", stopTimeout.String(),
				"-l", "debug",
				"-d",
				"--",
			},
			runArgs...,
		)...,
	)
	if err != nil {
		return fmt.Errorf("cannot create target container (standalone mode): %w", err)
	}

	log.WithField("containerId", contID).Debug("Test sensor container created (standalone mode)")
	s.contID = contID

	if err := containerStart(ctx, s.contID); err != nil {
		return fmt.Errorf("cannot start target container (standalone mode): %w", err)
	}

	log.WithField("containerId", contID).Debug("Test sensor container started (standalone mode)")

	return nil
}

func (s *Sensor) StartStandaloneOrFail(
	t *testing.T,
	ctx context.Context,
	runArgs []string,
	cmdOverride ...command.StartMonitor,
) {
	if err := s.StartStandalone(ctx, runArgs, cmdOverride...); err != nil {
		t.Fatal("Cannot start sensor (standalone mode):", err)
	}
}

func (s *Sensor) SendCommand(ctx context.Context, cmd command.Message) error {
	if s.client == nil {
		return errors.New("IPC client isn't initialized - is sensor running?")
	}

	// TODO: Use timeout from ctx.
	resp, err := s.client.SendCommand(cmd)
	if err != nil || resp.Status != command.ResponseStatusOk {
		return fmt.Errorf("IPC client.SendCommand() failed with response %q: %w", resp, err)
	}

	return nil
}

func (s *Sensor) SendStartCommand(
	ctx context.Context,
	cmdOverride ...command.StartMonitor,
) error {
	cmd := startCommandControlled(s.image, cmdOverride...)
	return s.SendCommand(ctx, &cmd)
}

func (s *Sensor) SendStartCommandOrFail(
	t *testing.T,
	ctx context.Context,
	cmdOverride ...command.StartMonitor,
) {
	if err := s.SendStartCommand(ctx, cmdOverride...); err != nil {
		t.Fatal("Failed sending StartMonitor command:", err)
	}
}

func (s *Sensor) SendStopCommand(ctx context.Context) error {
	return s.SendCommand(ctx, &command.StopMonitor{})
}

func (s *Sensor) SendStopCommandOrFail(t *testing.T, ctx context.Context) {
	if err := s.SendStopCommand(ctx); err != nil {
		t.Fatal("Failed sending StopMonitor command:", err)
	}
}

func (s *Sensor) Shutdown(ctx context.Context) error {
	if err := s.SendCommand(ctx, &command.ShutdownSensor{}); err != nil {
		return err
	}

	if err := s.client.Stop(); err != nil {
		return fmt.Errorf("IPC client.Stop() failed: %w", err)
	}

	return nil
}

func (s *Sensor) ShutdownOrFail(t *testing.T, ctx context.Context) {
	if err := s.Shutdown(ctx); err != nil {
		t.Fatal("Test sensor shutdown failed:", err)
	}
}

func (s *Sensor) Wait(ctx context.Context) (int, error) {
	if len(s.contID) == 0 {
		return -1, errNotStarted
	}

	exitCode, err := containerWait(ctx, s.contID)
	if err == nil {
		s.stopped = true
		return exitCode, nil
	}

	return -1, err
}

func (s *Sensor) WaitOrFail(t *testing.T, ctx context.Context) int {
	exitCode, err := s.Wait(ctx)
	if err != nil {
		t.Fatal("Failed waiting for test sensor container:", err)
	}
	return exitCode
}

func (s *Sensor) Signal(ctx context.Context, sig syscall.Signal) error {
	if len(s.contID) == 0 {
		return errNotStarted
	}

	return containerKill(ctx, s.contID, sig)
}

func (s *Sensor) SignalOrFail(t *testing.T, ctx context.Context, sig syscall.Signal) {
	if err := s.Signal(ctx, sig); err != nil {
		t.Fatal("Cannot signal test sensor container:", err)
	}
}

func (s *Sensor) DownloadArtifacts(ctx context.Context) error {
	if len(s.contID) == 0 {
		return errNotStarted
	}

	localArtifactPath := filepath.Join(s.contextDirPath, "artifacts")
	if err := containerCopyFrom(
		ctx,
		s.contID,
		app.DefaultArtifactDirPath,
		localArtifactPath,
	); err != nil {
		return fmt.Errorf("cannot download test sensor's artifacts: %w", err)
	}

	var creport report.ContainerReport
	if err := fsutil.LoadStructFromFile(
		filepath.Join(localArtifactPath, report.DefaultContainerReportFileName),
		&creport,
	); err != nil {
		return fmt.Errorf("cannot load test sensor's report: %w", err)
	}

	s.creport = &creport
	return nil
}

func (s *Sensor) DownloadArtifactsOrFail(t *testing.T, ctx context.Context) {
	if err := s.DownloadArtifacts(ctx); err != nil {
		t.Fatal("Cannot download test sensor's artifacts:", err)
	}
}

func (s *Sensor) Cleanup(t *testing.T, ctx context.Context) {
	if t.Failed() {
		s.PrintState(ctx)
	}

	if len(s.contID) == 0 || s.stopped {
		return
	}

	if err := s.Signal(ctx, syscall.SIGKILL); err != nil {
		log.WithError(err).Warnf("Sensor cleanup: cannot signal container %q", s.contID)
	} else {
		time.Sleep(2 * time.Second)
	}

	if err := containerRemove(ctx, s.contID); err != nil {
		log.WithError(err).Warnf("Sensor cleanup: cannot remove container %q", s.contID)
	}
}

func (s *Sensor) PrintState(ctx context.Context) {
	log.
		WithField("image", s.image).
		WithField("container", s.contID).
		WithField("context", s.contextDirPath).
		WithField("exe", s.sensorExePath).
		WithField("creport downloaded", s.creport != nil).
		Info("Printing out test sensor state")

	if s.creport != nil {
		fmt.Fprintln(os.Stderr, "-=== Container report ===-")
		encoder := json.NewEncoder(os.Stderr)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(s.creport); err != nil {
			log.WithError(err).Error("Cannot print out container report")
		}
		fmt.Fprintln(os.Stderr, "-=== eof: Container report ===-")
	}

	if len(s.contID) > 0 {
		fmt.Fprintln(os.Stderr, "-=== Container logs ===-")
		if contLogs, err := containerLogs(ctx, s.contID); err == nil {
			fmt.Fprintln(os.Stderr, contLogs)
		} else {
			log.WithError(err).Error("Cannot obtain target container logs")
		}
		fmt.Fprintln(os.Stderr, "-=== eof: Container logs ===-")
	}
}

func (s *Sensor) ExpectEvent(t *testing.T, name event.Type) {
	if s.client == nil {
		t.Fatal("IPC client isn't initialized - is sensor running?")
	}

	evt, err := s.client.GetEvent()
	if err != nil {
		t.Fatalf("IPC client.GetEvent() failed with response %q: %w", evt, err)
	}

	if evt.Name != name {
		t.Fatalf("unexpected event type %q (expected %q)", evt.Name, name)
	}
}

func (s *Sensor) AssertSensorLogsContain(t *testing.T, ctx context.Context, what ...string) {
	// TODO: Write a proper implementation after the split of the sensor's and the target app's logs.
	s.AssertTargetAppLogsContain(t, ctx, what...)
}

func (s *Sensor) AssertTargetAppLogsContain(t *testing.T, ctx context.Context, what ...string) {
	if len(s.contID) == 0 {
		t.Fatal("Test sensor container hasn't been started yet")
	}

	contLogs, err := containerLogs(ctx, s.contID)
	if err != nil {
		t.Error("Cannot retrieve container logs", err)
		return
	}

	for _, w := range what {
		if strings.Index(contLogs, w) == -1 {
			t.Errorf("Cannot find string %q in container logs", w)
		}
	}
}

func (s *Sensor) AssertReportIncludesFiles(t *testing.T, filepath ...string) {
	if s.creport == nil {
		t.Fatal("No sensor report found")
	}

	index := artifactsByFilePath(s.creport.Image.Files)
	for _, f := range filepath {
		if index[f] == nil {
			t.Errorf("Expected file %q not found in the container report", f)
		}
	}
}

func (s *Sensor) AssertReportNotIncludesFiles(t *testing.T, filepath ...string) {
	if s.creport == nil {
		t.Fatal("No sensor report found")
	}

	index := artifactsByFilePath(s.creport.Image.Files)
	for _, f := range filepath {
		if index[f] != nil {
			t.Errorf("Unexpected file %q found in the container report", f)
		}
	}
}

func startCommandControlled(
	image dockerapi.Image,
	cmdOverride ...command.StartMonitor,
) command.StartMonitor {
	cmd := NewMonitorStartCommand(WithSaneDefaults())
	if len(cmdOverride) > 0 {
		cmd = cmdOverride[0]
	}

	if len(cmd.AppName) == 0 {
		if len(image.Config.Entrypoint) > 0 {
			cmd.AppName = image.Config.Entrypoint[0]
			cmd.AppArgs = append(image.Config.Entrypoint[1:], image.Config.Cmd...)
		} else {
			cmd.AppName = image.Config.Cmd[0]
			cmd.AppArgs = image.Config.Cmd[1:]
		}
	}

	if len(image.Config.User) > 0 {
		cmd.AppUser = image.Config.User
		cmd.RunTargetAsUser = true
	}

	return cmd
}

func startCommandStandalone(
	image dockerapi.Image,
	cmdOverride ...command.StartMonitor,
) command.StartMonitor {
	cmd := NewMonitorStartCommand(WithSaneDefaults())
	if len(cmdOverride) > 0 {
		cmd = cmdOverride[0]
	}

	cmd.AppEntrypoint = image.Config.Entrypoint
	cmd.AppCmd = image.Config.Cmd

	if len(image.Config.User) > 0 {
		cmd.AppUser = image.Config.User
		cmd.RunTargetAsUser = true
	}

	return cmd
}

func jsonDump(filename string, val interface{}) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("JSON dump failed: cannot create|open file %q: %w", filename, err)
	}

	encoder := json.NewEncoder(f)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(val); err != nil {
		return fmt.Errorf("JSON dump failed: encoding of value %q failed: %w", val, err)
	}

	return nil
}

func artifactsByFilePath(files []*report.ArtifactProps) map[string]*report.ArtifactProps {
	dict := make(map[string]*report.ArtifactProps)
	for _, props := range files {
		if props != nil {
			dict[props.FilePath] = props
		}
	}
	return dict
}

func hostPort(cont dockerapi.Container, contPort int) (string, bool) {
	if cont.NetworkSettings != nil {
		for port, bindings := range cont.NetworkSettings.Ports {
			if port.Port() == fmt.Sprintf("%d", contPort) && len(bindings) > 0 {
				return bindings[0].HostPort, true
			}
		}
	}

	return "", false
}
