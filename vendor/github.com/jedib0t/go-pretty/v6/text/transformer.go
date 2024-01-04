package text

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Transformer related constants
const (
	unixTimeMinMilliseconds = int64(10000000000)
	unixTimeMinMicroseconds = unixTimeMinMilliseconds * 1000
	unixTimeMinNanoSeconds  = unixTimeMinMicroseconds * 1000
)

// Transformer related variables
var (
	colorsNumberPositive = Colors{FgHiGreen}
	colorsNumberNegative = Colors{FgHiRed}
	colorsNumberZero     = Colors{}
	colorsURL            = Colors{Underline, FgBlue}
	rfc3339Milli         = "2006-01-02T15:04:05.000Z07:00"
	rfc3339Micro         = "2006-01-02T15:04:05.000000Z07:00"

	possibleTimeLayouts = []string{
		time.RFC3339,
		rfc3339Milli, // strfmt.DateTime.String()'s default layout
		rfc3339Micro,
		time.RFC3339Nano,
	}
)

// Transformer helps format the contents of an object to the user's liking.
type Transformer func(val interface{}) string

// NewNumberTransformer returns a number Transformer that:
//   - transforms the number as directed by 'format' (ex.: %.2f)
//   - colors negative values Red
//   - colors positive values Green
func NewNumberTransformer(format string) Transformer {
	return func(val interface{}) string {
		if valStr := transformInt(format, val); valStr != "" {
			return valStr
		}
		if valStr := transformUint(format, val); valStr != "" {
			return valStr
		}
		if valStr := transformFloat(format, val); valStr != "" {
			return valStr
		}
		return fmt.Sprint(val)
	}
}

func transformInt(format string, val interface{}) string {
	transform := func(val int64) string {
		if val < 0 {
			return colorsNumberNegative.Sprintf("-"+format, -val)
		}
		if val > 0 {
			return colorsNumberPositive.Sprintf(format, val)
		}
		return colorsNumberZero.Sprintf(format, val)
	}

	if number, ok := val.(int); ok {
		return transform(int64(number))
	}
	if number, ok := val.(int8); ok {
		return transform(int64(number))
	}
	if number, ok := val.(int16); ok {
		return transform(int64(number))
	}
	if number, ok := val.(int32); ok {
		return transform(int64(number))
	}
	if number, ok := val.(int64); ok {
		return transform(number)
	}
	return ""
}

func transformUint(format string, val interface{}) string {
	transform := func(val uint64) string {
		if val > 0 {
			return colorsNumberPositive.Sprintf(format, val)
		}
		return colorsNumberZero.Sprintf(format, val)
	}

	if number, ok := val.(uint); ok {
		return transform(uint64(number))
	}
	if number, ok := val.(uint8); ok {
		return transform(uint64(number))
	}
	if number, ok := val.(uint16); ok {
		return transform(uint64(number))
	}
	if number, ok := val.(uint32); ok {
		return transform(uint64(number))
	}
	if number, ok := val.(uint64); ok {
		return transform(number)
	}
	return ""
}

func transformFloat(format string, val interface{}) string {
	transform := func(val float64) string {
		if val < 0 {
			return colorsNumberNegative.Sprintf("-"+format, -val)
		}
		if val > 0 {
			return colorsNumberPositive.Sprintf(format, val)
		}
		return colorsNumberZero.Sprintf(format, val)
	}

	if number, ok := val.(float32); ok {
		return transform(float64(number))
	}
	if number, ok := val.(float64); ok {
		return transform(number)
	}
	return ""
}

// NewJSONTransformer returns a Transformer that can format a JSON string or an
// object into pretty-indented JSON-strings.
func NewJSONTransformer(prefix string, indent string) Transformer {
	return func(val interface{}) string {
		if valStr, ok := val.(string); ok {
			var b bytes.Buffer
			if err := json.Indent(&b, []byte(strings.TrimSpace(valStr)), prefix, indent); err == nil {
				return b.String()
			}
		} else if b, err := json.MarshalIndent(val, prefix, indent); err == nil {
			return string(b)
		}
		return fmt.Sprintf("%#v", val)
	}
}

// NewTimeTransformer returns a Transformer that can format a timestamp (a
// time.Time) into a well-defined time format defined using the provided layout
// (ex.: time.RFC3339).
//
// If a non-nil location value is provided, the time will be localized to that
// location (use time.Local to get localized timestamps).
func NewTimeTransformer(layout string, location *time.Location) Transformer {
	return func(val interface{}) string {
		rsp := fmt.Sprint(val)
		if valTime, ok := val.(time.Time); ok {
			rsp = formatTime(valTime, layout, location)
		} else {
			// cycle through some supported layouts to see if the string form
			// of the object matches any of these layouts
			for _, possibleTimeLayout := range possibleTimeLayouts {
				if valTime, err := time.Parse(possibleTimeLayout, rsp); err == nil {
					rsp = formatTime(valTime, layout, location)
					break
				}
			}
		}
		return rsp
	}
}

// NewUnixTimeTransformer returns a Transformer that can format a unix-timestamp
// into a well-defined time format as defined by 'layout'. This can handle
// unix-time in Seconds, MilliSeconds, Microseconds and Nanoseconds.
//
// If a non-nil location value is provided, the time will be localized to that
// location (use time.Local to get localized timestamps).
func NewUnixTimeTransformer(layout string, location *time.Location) Transformer {
	transformer := NewTimeTransformer(layout, location)

	return func(val interface{}) string {
		if unixTime, ok := val.(int64); ok {
			return formatTimeUnix(unixTime, transformer)
		} else if unixTimeStr, ok := val.(string); ok {
			if unixTime, err := strconv.ParseInt(unixTimeStr, 10, 64); err == nil {
				return formatTimeUnix(unixTime, transformer)
			}
		}
		return fmt.Sprint(val)
	}
}

// NewURLTransformer returns a Transformer that can format and pretty print a string
// that contains a URL (the text is underlined and colored Blue).
func NewURLTransformer(colors ...Color) Transformer {
	colorsToUse := colorsURL
	if len(colors) > 0 {
		colorsToUse = colors
	}

	return func(val interface{}) string {
		return colorsToUse.Sprint(val)
	}
}

func formatTime(t time.Time, layout string, location *time.Location) string {
	rsp := ""
	if t.Unix() > 0 {
		if location != nil {
			t = t.In(location)
		}
		rsp = t.Format(layout)
	}
	return rsp
}

func formatTimeUnix(unixTime int64, timeTransformer Transformer) string {
	if unixTime >= unixTimeMinNanoSeconds {
		unixTime = unixTime / time.Second.Nanoseconds()
	} else if unixTime >= unixTimeMinMicroseconds {
		unixTime = unixTime / (time.Second.Nanoseconds() / 1000)
	} else if unixTime >= unixTimeMinMilliseconds {
		unixTime = unixTime / (time.Second.Nanoseconds() / 1000000)
	}
	return timeTransformer(time.Unix(unixTime, 0))
}
