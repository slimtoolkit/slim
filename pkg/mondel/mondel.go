package mondel

//Monitor Data Event Logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/acounter"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
)

const eventBufSize = 10000

var (
	ErrEventDropped = errors.New("event dropped")
)

type Publisher interface {
	Publish(event *report.MonitorDataEvent) error
	Stop()
}

type publisher struct {
	stopped   chan struct{}
	ctx       context.Context
	enable    bool
	output    *os.File
	eventCh   chan *report.MonitorDataEvent
	seqNumber acounter.Type
}

func NewPublisher(ctx context.Context, enable bool, outputFile string) *publisher {
	logger := log.WithField("op", "NewPublisher")
	logger.WithFields(log.Fields{
		"enable":      enable,
		"output.file": outputFile,
	}).Trace("call")
	defer logger.Trace("exit")

	ref := &publisher{
		stopped: make(chan struct{}),
		ctx:     ctx,
		enable:  enable,
	}

	if !ref.enable {
		return ref
	}

	// fsutil.Touch() creates potentially missing folder(s).
	if err := fsutil.Touch(outputFile); err != nil {
		log.WithError(err).Errorf("cannot create mondel file %q - fsutil.Touch() failed", outputFile)
		ref.enable = false
		return ref
	}

	// Using O_SYNC because there is another process (art_collector) that is
	// reading from this file. If we don't use O_SYNC, then the file may not
	// be flushed to disk for too long.
	f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_WRONLY|os.O_SYNC, 0644)
	if err != nil {
		log.WithError(err).Errorf("os.OpenFile(%v)", outputFile)
		ref.enable = false
		return ref
	}

	ref.output = f
	ref.eventCh = make(chan *report.MonitorDataEvent, eventBufSize)

	go ref.process()

	return ref
}

func (ref *publisher) Stop() {
	close(ref.stopped)
}

func (ref *publisher) Publish(event *report.MonitorDataEvent) error {
	if !ref.enable || event == nil {
		return nil
	}

	event.Timestamp = time.Now().UTC().UnixNano()
	event.SeqNumber = ref.seqNumber.Inc()

	logger := log.WithField("op", "mondel.publisher.Publish")
	select {
	case <-ref.stopped:
		logger.Debugf("publisher stopped - dropped event (%#v)", event)
		return ErrEventDropped
		return nil
	case <-ref.ctx.Done():
		return ref.ctx.Err()

	case ref.eventCh <- event:
		return nil

	default:
		logger.Debugf("dropped event (%#v)", event)
		return ErrEventDropped
	}
}

func (ref *publisher) process() {
	logger := log.WithField("op", "mondel.publisher.process")
	logger.Trace("call")
	defer logger.Trace("exit")
	defer func() {
		err := ref.output.Close()
		logger.Tracef("closed ref.output (e=%v)", err)
	}()

	var buf bytes.Buffer
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var once sync.Once
	finish := func() {
		once.Do(func() {
			close(ref.eventCh)

			logger.Debugf("draining eventCh - %d", len(ref.eventCh))
			for evt := range ref.eventCh {
				encoded, err := encodeEvent(evt)
				if err != nil {
					logger.Debugf("could not encode - %v", encoded)
					continue
				}
				buf.WriteString(encoded)
			}

			if buf.Len() > 0 {
				logger.Debugf("saving leftover data - %d", buf.Len())
				if _, err := ref.output.Write(buf.Bytes()); err != nil {
					logger.Errorf("Error writing batch: %v", err)
				}
			}

			logger.Debug("finish(): done")
		})
	}

	for {
		select {
		case <-ref.stopped:
			logger.Debug("done - finishing....")
			finish()
			return
		case <-ref.ctx.Done():
			logger.Debug("done - finishing....")
			finish()
			return

		case evt := <-ref.eventCh:
			encoded, err := encodeEvent(evt)
			if err != nil {
				logger.Debugf("could not encode - %v", encoded)
				continue
			}
			buf.WriteString(encoded)

		case <-ticker.C:
			// Flush the buffer every second
			if buf.Len() > 0 {
				if _, err := ref.output.Write(buf.Bytes()); err != nil {
					logger.Errorf("Error writing batch: %v", err)
				}
				buf.Reset()
			}
		}
	}
}

func encodeEvent(event *report.MonitorDataEvent) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(event); err != nil {
		return "", fmt.Errorf("error encoding data - %v", err)
	}

	return b.String(), nil
}
