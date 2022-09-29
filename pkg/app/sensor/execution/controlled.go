package execution

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/app/sensor/ipc"
	"github.com/docker-slim/docker-slim/pkg/ipc/command"
	"github.com/docker-slim/docker-slim/pkg/ipc/event"
)

type controlledExe struct {
	hookExecutor

	ipcServer *ipc.Server
}

func NewControlled(
	ctx context.Context,
	lifecycleHookCommand string,
) (Interface, error) {
	log.Debug("sensor: starting IPC server...")

	ipcServer, err := ipc.NewServer(ctx.Done())
	if err != nil {
		return nil, err
	}

	if err := ipcServer.Run(); err != nil {
		return nil, err
	}

	return &controlledExe{
		hookExecutor: hookExecutor{
			ctx: ctx,
			cmd: lifecycleHookCommand,
		},
		ipcServer: ipcServer,
	}, nil
}

func (e *controlledExe) Close() {
	e.ipcServer.Stop()
}

func (e *controlledExe) Commands() <-chan command.Message {
	return e.ipcServer.CommandChan()
}

func (e *controlledExe) PubEvent(etype event.Type, data ...interface{}) {
	e.ipcServer.TryPublishEvt(&event.Message{Name: etype, Data: data}, 3)
}
