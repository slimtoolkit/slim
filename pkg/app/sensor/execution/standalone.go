package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app/sensor/standalone/control"
	"github.com/slimtoolkit/slim/pkg/ipc/command"
	"github.com/slimtoolkit/slim/pkg/ipc/event"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
)

type standaloneExe struct {
	hookExecutor

	commandCh chan command.Message
	eventFile io.WriteCloser
}

func NewStandalone(
	ctx context.Context,
	commandFileName string,
	eventFileName string,
	lifecycleHookCommand string,
) (Interface, error) {
	// fsutil.Touch() creates (potentially missing) folder(s).
	if err := fsutil.Touch(eventFileName); err != nil {
		return nil, fmt.Errorf(
			"cannot create execution - touch event file %q failed: %w",
			eventFileName, err,
		)
	}

	eventFile, err := os.OpenFile(eventFileName, os.O_APPEND|os.O_WRONLY|os.O_SYNC, 0644)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot create execution - open event file %q failed: %w",
			eventFileName, err,
		)
	}

	cmd, err := readCommandFile(commandFileName)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot create execution - cannot read command file %q: %w",
			commandFileName, err,
		)
	}

	commandCh := make(chan command.Message, 10)
	commandCh <- &cmd

	go control.HandleControlCommandQueue(ctx, commandFileName, commandCh)

	return &standaloneExe{
		hookExecutor: hookExecutor{
			ctx: ctx,
			cmd: lifecycleHookCommand,
		},
		commandCh: commandCh,
		eventFile: eventFile,
	}, nil
}

func (e *standaloneExe) Close() {
	e.eventFile.Close()
	close(e.commandCh)
}

func (e *standaloneExe) Commands() <-chan command.Message {
	return e.commandCh
}

func (e *standaloneExe) PubEvent(name event.Type, data ...interface{}) {
	encoder := json.NewEncoder(e.eventFile)
	encoder.SetEscapeHTML(false)
	evt := event.Message{Name: name}
	if len(data) > 0 {
		evt.Data = data[0]
	}

	if err := encoder.Encode(evt); err != nil {
		log.WithError(err).Warn("sensor: failed dumping event")
	}
}

// TODO: Make this function return a list of commands.
func readCommandFile(filename string) (command.StartMonitor, error) {
	var cmd command.StartMonitor

	data, err := os.ReadFile(filename)
	if err != nil {
		return cmd, fmt.Errorf("could not read command file %q: %w", filename, err)
	}
	data = bytes.Split(data, []byte("\n"))[0]

	if err := json.Unmarshal(data, &cmd); err != nil {
		return cmd, fmt.Errorf("could not decode command %q: %w", string(data), err)
	}

	// The instrumented image will always have the ENTRYPOINT overwritten
	// by the instrumentor to make the sensor the PID1 process in the monitored
	// container.
	// The original ENTRYPOINT & CMD will be preserved as part of the
	// `commands.json` file. However, it's also possible to override the
	// CMD at runtime by supplying extra args to the `docker run` (or alike)
	// command. Sensor needs to be able to detect this and replace the
	// baked in CMD with the new list of args. For that, the instrumented image's
	// ENTRYPOINT has to contain a special separator value `--` denoting the end
	// of the sensor's flags sequence. Example:
	//
	// ENTRYPOINT ["/path/to/sensor", "-m=standalone", "-c=/path/to/commands.json", "--" ]

	// Note on CMD & ENTRYPOINT override: Historically, sensor used
	// AppName + AppArgs[] to start the target process. With the addition
	// of the standalone mode, the need for supporting Docker's original
	// CMD[] + ENTRYPOINT[] approach arose. However, to keep things simple:
	//
	//  - These two "modes" of starting the target app will be mutually exclusive
	//    (from the sensor's users standpoint).
	//  - The CMD + ENTRYPOINT mode will be converted back to the AppName + AppArgs
	//    early on in the sensor's processing (to prevent cascading changes).

	// First, check if there is a run-time override of the CMD[] part
	// (i.e., anything after the `--` separator).
	// If there is some, it'll essentially "activate" the CMD + ENTRYPOINT mode.
	if args := flag.Args(); len(args) > 0 {
		cmd.AppCmd = args
	}

	// If it's ENTRYPOINT + CMD mode, converting back to AppName + AppArgs.
	if len(cmd.AppEntrypoint)+len(cmd.AppCmd) > 0 {
		if len(cmd.AppName)+len(cmd.AppArgs) > 0 {
			return cmd, errors.New("ambiguous start command: cannot use [app_name,app_args] and [app_entrypoint,app_cmd] simultaneously")
		}

		if len(cmd.AppEntrypoint) > 0 {
			cmd.AppName = cmd.AppEntrypoint[0]
			cmd.AppArgs = append(cmd.AppEntrypoint[1:], cmd.AppCmd...)
		} else {
			cmd.AppName = cmd.AppCmd[0]
			cmd.AppArgs = cmd.AppCmd[1:]
		}
	}

	return cmd, nil
}
