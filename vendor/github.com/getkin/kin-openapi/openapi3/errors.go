package openapi3

import (
	"bytes"
	"errors"
)

// MultiError is a collection of errors, intended for when
// multiple issues need to be reported upstream
type MultiError []error

func (me MultiError) Error() string {
	buff := &bytes.Buffer{}
	for _, e := range me {
		buff.WriteString(e.Error())
		buff.WriteString(" | ")
	}
	return buff.String()
}

//Is allows you to determine if a generic error is in fact a MultiError using `errors.Is()`
//It will also return true if any of the contained errors match target
func (me MultiError) Is(target error) bool {
	if _, ok := target.(MultiError); ok {
		return true
	}
	for _, e := range me {
		if errors.Is(e, target) {
			return true
		}
	}
	return false
}

//As allows you to use `errors.As()` to set target to the first error within the multi error that matches the target type
func (me MultiError) As(target interface{}) bool {
	for _, e := range me {
		if errors.As(e, target) {
			return true
		}
	}
	return false
}
