package monitor

import (
	"context"
	"errors"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	stubmonitor "github.com/slimtoolkit/slim/pkg/test/stub/sensor/monitor"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func TestCompositeMonitor_Lifecycle(t *testing.T) {
	mon := &monitor{
		fanMon: stubmonitor.NewFanMonitor(context.Background()),
		ptMon:  stubmonitor.NewPtMonitor(context.Background()),
	}

	mon.Start()

	select {
	case <-mon.Done():
		t.Fatal("composite monitor must not be done yet")
	default:
	}

	mon.Cancel()

	select {
	case <-mon.Done():
		break
	case <-time.After(1*time.Second + minPassiveMonitoring):
		t.Fatal("composite monitor must be done by this time")
	}

	select {
	case <-mon.Done():
		break
	default:
		t.Fatal("composite monitor Done() method must be reentrant")
	}

	// TODO: Check the status (reports & final error).
}

func TestCompositeMonitor_DrainErrors(t *testing.T) {
	mon := &monitor{
		errorCh: make(chan error, errorChanBufSize),
	}

	go func() {
		// Definitely within the drain window.
		mon.errorCh <- errors.New("err1")

		time.Sleep(errorChanDrainTime / 2)

		// Still within the drain window.
		mon.errorCh <- errors.New("err2")

		time.Sleep(errorChanDrainTime * 2)

		// Definitely outside of the drain window.
		mon.errorCh <- errors.New("err3")
	}()

	drained := mon.DrainErrors()
	if len(drained) != 2 {
		t.Errorf("Unexpected number of drained errors: %d", len(drained))
	}
	if drained[0].Error() != "err1" {
		t.Errorf("Unexpected drained error: %q", drained[0])
	}
	if drained[1].Error() != "err2" {
		t.Errorf("Unexpected drained error: %q", drained[1])
	}
}
