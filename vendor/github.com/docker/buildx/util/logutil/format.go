package logutil

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

type Formatter struct {
	logrus.TextFormatter
}

func (f *Formatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(fmt.Sprintf("%s: %s\n", strings.ToUpper(entry.Level.String()), entry.Message)), nil
}
