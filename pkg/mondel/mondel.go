package mondel

//Monitor Data Event Logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/report"
)

const eventBufSize = 10000

var (
	ErrEventDropped = errors.New("event dropped")
)

type Publisher interface {
	Publish(event *report.MonitorDataEvent) error
}

type publisher struct {
	ctx        context.Context
	enable     bool
	outputFile string
	output     *os.File
	eventCh    chan *report.MonitorDataEvent
}

func NewPublisher(ctx context.Context, enable bool, outputFile string) *publisher {
	logger := log.WithField("op", "NewPublisher")
	logger.WithFields(log.Fields{
		"enable":      enable,
		"output.file": outputFile,
	}).Trace("call")
	defer logger.Trace("exit")

	ref := &publisher{
		ctx:        ctx,
		enable:     enable,
		outputFile: outputFile,
	}

	if !ref.enable {
		return ref
	}

	f, err := os.OpenFile(ref.outputFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.WithError(err).Errorf("os.OpenFile(%v)", ref.outputFile)
	} else {
		ref.output = f
	}

	ref.output = f
	ref.eventCh = make(chan *report.MonitorDataEvent, eventBufSize)

	go ref.process()
	return ref
}

func (ref *publisher) Publish(event *report.MonitorDataEvent) error {
	if !ref.enable {
		return nil
	}

	select {
	case ref.eventCh <- event:
		return nil
	default:
		log.Debugf("mondel.publisher.Publish: dropped event (%#v)", event)
		return ErrEventDropped
	}
}

func (ref *publisher) process() {
	logger := log.WithField("op", "mondel.publisher.process")
	logger.Trace("call")
	defer logger.Trace("exit")

done:
	for {
		select {
		case <-ref.ctx.Done():
			logger.Debug("done - stopping...")
			break done

		case evt := <-ref.eventCh:
			encoded, err := encodeEvent(evt)
			if err != nil {
				logger.Debugf("could not encode - %v", encoded)
				continue
			}

			if ref.output != nil {
				_, err := ref.output.WriteString(encoded)
				if err != nil {
					logger.Tracef("TMP: error writing - %v (%s)\n", err, encoded)
				}
			} else {
				fmt.Printf("%s", encoded)
			}
		}
	}

	if ref.output != nil {
		ref.output.Close()
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
