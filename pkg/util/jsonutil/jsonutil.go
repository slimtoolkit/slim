package jsonutil

import (
	"bytes"
	"encoding/json"
)

func ToString(input interface{}) string {
	return toString(input, false)
}

func ToPretty(input interface{}) string {
	return toString(input, true)
}

func toString(input interface{}, pretty bool) string {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if pretty {
		encoder.SetIndent(" ", " ")
	}
	_ = encoder.Encode(input)
	return out.String()
}
