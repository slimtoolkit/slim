//go:build (linux && 386) || (linux && amd64) || (linux && arm64)
// +build linux,386 linux,amd64 linux,arm64

package system

func nativeCharsToString(cstr [65]int8) string {
	max := len(cstr)
	data := make([]byte, max)

	var pos int
	for ; (pos < max) && (cstr[pos] != 0); pos++ {
		data[pos] = byte(cstr[pos])
	}

	return string(data[0:pos])
}
