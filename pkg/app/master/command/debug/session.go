package debug

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/segmentio/ksuid"
	log "github.com/sirupsen/logrus"
)

const (
	containerNamePrefix = "mint-debugger-"
	containerNamePat    = "mint-debugger-%v"
)

const (
	CSWaiting    = "WAITING"
	CSRunning    = "RUNNING"
	CSTerminated = "TERMINATED"
)

type DebugContainerInfo struct {
	TargetContainerName string
	Name                string
	SpecImage           string
	Command             []string
	Args                []string
	WorkingDir          string
	TTY                 bool
	ContainerID         string
	RunningImage        string
	RunningImageID      string
	StartTime           string
	FinishTime          string
	State               string
	ExitCode            int32
	ExitReason          string
	ExitMessage         string
	WaitReason          string
	WaitMessage         string
}

func generateSessionID() string {
	id, err := ksuid.NewRandom()
	if err != nil {
		log.WithField("op", "debug.generateSessionID").WithError(err).Error("ksuid.NewRandom")
		return fmt.Sprintf("%v%v", time.Now().UTC().UnixNano(), os.Getpid())
	}

	return hex.EncodeToString(id.Bytes())
}

func generateContainerName(sid string) string {
	if sid == "" {
		sid = generateSessionID()
	}

	return fmt.Sprintf(containerNamePat, sid)
}
