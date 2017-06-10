package system

import (
	"errors"
)

var (
	ErrNotConfigured    = errors.New("feature is not configured")
	ErrNoConfigs        = errors.New("no kernel configs")
	ErrArchNotSupported = errors.New("unsupported architecture")
)
