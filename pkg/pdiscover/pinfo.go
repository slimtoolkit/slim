package pdiscover

import (
	"errors"
)

var (
	ErrInvalidProcArgsLen = errors.New("invalid ProcArgs length")
	ErrInvalidProcInfo    = errors.New("invalid proc info")
)
