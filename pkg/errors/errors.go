package errors

import (
	"fmt"
	"runtime"
)

type SensorError struct {
	Op      string        `json:"op"`
	Kind    string        `json:"kind"`
	Next    *SensorError  `json:"next,omitempty"`
	Wrapped *WrappedError `json:"wrapped,omitempty"`
}

type WrappedError struct {
	Type string `json:"type"`
	Info string `json:"info"`
	File string `json:"file"`
	Line int    `json:"line"`
}

func (e *SensorError) Error() string {
	errStr := ""
	if e.Next != nil {
		errStr = fmt.Sprintf(",Next:%s", e.Next.Error())
	}
	if e.Wrapped != nil {
		if errStr == "" {
			errStr = fmt.Sprintf(",Wrapped:{Type=%s,Info=%s,Line:%d,File:%s}", e.Wrapped.Type, e.Wrapped.Info, e.Wrapped.Line, e.Wrapped.File)
		} else {
			errStr = fmt.Sprintf("%s,Wrapped:{Type=%s,Info=%s,Line:%d,File:%s}", errStr, e.Wrapped.Type, e.Wrapped.Info, e.Wrapped.Line, e.Wrapped.File)
		}
	}

	return fmt.Sprintf("SensorError{Op:%s,Kind:%s%s}", e.Op, e.Kind, errStr)
}

func SE(op string, kind string, err error) *SensorError {
	e := &SensorError{
		Op:   op,
		Kind: kind,
	}

	if next, ok := err.(*SensorError); ok {
		e.Next = next
	} else {
		e.Wrapped = &WrappedError{
			Type: fmt.Sprintf("%T", err),
			Info: err.Error(),
		}

		if _, file, line, ok := runtime.Caller(1); ok {
			e.Wrapped.File = file
			e.Wrapped.Line = line
		}
	}

	return e
}

func Drain(ch <-chan error) (arr []error) {
	for {
		select {
		case e := <-ch:
			arr = append(arr, e)
		default:
			return arr
		}
	}
}
