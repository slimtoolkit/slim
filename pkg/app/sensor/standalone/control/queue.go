package control

import (
	"bufio"
	"context"
	"io"
	"os"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/ipc/command"
)

func HandleControlCommandQueue(ctx context.Context, commandsFile string, commandCh chan command.Message) {
	fifoPath := getFIFOPath(commandsFile)
	if !createFIFOIfNeeded(fifoPath) {
		return
	}
	go func() {
		<-ctx.Done()
		os.Remove(fifoPath)
	}()

	processCommandsFromFIFO(ctx, fifoPath, commandCh)
}

func getFIFOPath(commandsFile string) string {
	return commandsFile + ".fifo"
}

func createFIFOIfNeeded(fifoPath string) bool {
	if _, err := os.Stat(fifoPath); os.IsNotExist(err) {
		if err = syscall.Mkfifo(fifoPath, 0600); err != nil {
			log.Warnf("sensor: control commands not activated - cannot create %s FIFO file: %s", fifoPath, err)
			return false
		}
		log.Info("sensor: control commands activated")
	}
	return true
}

func processCommandsFromFIFO(ctx context.Context, fifoPath string, commandCh chan command.Message) {
	for ctx.Err() == nil {
		fifo, err := os.Open(fifoPath)
		if err != nil {
			log.Debugf("sensor: control commands - cannot open %s FIFO file: %s", fifoPath, err)
			time.Sleep(1 * time.Second)
			continue
		}

		readAndHandleCommands(fifo, commandCh)
		fifo.Close()
	}
}

func readAndHandleCommands(fifo *os.File, commandCh chan command.Message) {
	reader := bufio.NewReader(fifo)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			handleCommand(line, commandCh)
		}

		if err == io.EOF {
			return
		}
		if err != nil {
			log.Warnf("sensor: error reading control command: %s", err)
			time.Sleep(1 * time.Second)
		}
	}
}

func handleCommand(line []byte, commandCh chan command.Message) {
	msg, err := command.Decode(line)
	if err == nil {
		commandCh <- msg
	} else {
		log.Warnf("sensor: cannot decode control command %#q: %s", line, err)
	}
}
