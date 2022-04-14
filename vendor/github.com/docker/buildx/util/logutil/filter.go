package logutil

import (
	"io/ioutil"
	"strings"

	"github.com/sirupsen/logrus"
)

func NewFilter(levels []logrus.Level, filters ...string) logrus.Hook {
	dl := logrus.New()
	dl.SetOutput(ioutil.Discard)
	return &logsFilter{
		levels:        levels,
		filters:       filters,
		discardLogger: dl,
	}
}

type logsFilter struct {
	levels        []logrus.Level
	filters       []string
	discardLogger *logrus.Logger
}

func (d *logsFilter) Levels() []logrus.Level {
	return d.levels
}

func (d *logsFilter) Fire(entry *logrus.Entry) error {
	for _, f := range d.filters {
		if strings.Contains(entry.Message, f) {
			entry.Logger = d.discardLogger
			return nil
		}
	}
	return nil
}
