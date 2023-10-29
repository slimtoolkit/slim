package control

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker-slim/docker-slim/pkg/ipc/command"
	"github.com/docker-slim/docker-slim/pkg/ipc/event"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
)

func ExecuteStopTargetAppCommand(
	ctx context.Context,
	commandsFile string,
	eventsFile string,
) error {
	msg, err := command.Encode(&command.StopMonitor{})
	if err != nil {
		return fmt.Errorf("cannot encode stop command: %w", err)
	}

	if err := fsutil.AppendToFile(getFIFOPath(commandsFile), msg, false); err != nil {
		return fmt.Errorf("cannot append stop command to FIFO file: %w", err)
	}

	if err := waitForEvent(ctx, eventsFile, event.StopMonitorDone); err != nil {
		return fmt.Errorf("waiting for %v event: %w", event.StopMonitorDone, err)
	}

	return nil
}

func waitForEvent(ctx context.Context, eventsFile string, target event.Type) error {
	for ctx.Err() == nil {
		found, err := findEvent(eventsFile, target)
		if err != nil {
			return err
		}

		if found {
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return ctx.Err()
}

func findEvent(eventsFile string, target event.Type) (bool, error) {
	file, err := os.Open(eventsFile)
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// A bit hacky - we probably need to parse the event struct properly.
		if strings.Contains(line, string(target)) {
			return true, nil
		}
	}

	if scanner.Err() != nil {
		return false, scanner.Err()
	}

	return false, nil
}
