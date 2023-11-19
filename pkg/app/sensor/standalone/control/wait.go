package control

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/slimtoolkit/slim/pkg/ipc/event"
)

func ExecuteWaitEvenCommand(
	ctx context.Context,
	eventsFile string,
	evt event.Type,
) error {
	if err := waitForEvent(ctx, eventsFile, evt); err != nil {
		return fmt.Errorf("waiting for %v event: %w", evt, err)
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
