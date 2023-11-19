package control

import (
	"context"
	"fmt"

	"github.com/slimtoolkit/slim/pkg/ipc/command"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
)

func ExecuteStopTargetAppCommand(ctx context.Context, commandsFile string) error {
	msg, err := command.Encode(&command.StopMonitor{})
	if err != nil {
		return fmt.Errorf("cannot encode stop command: %w", err)
	}

	if err := fsutil.AppendToFile(getFIFOPath(commandsFile), msg, false); err != nil {
		return fmt.Errorf("cannot append stop command to FIFO file: %w", err)
	}

	return nil
}
