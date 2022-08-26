//go:build (linux && arm) || (linux && ppc64) || (linux && ppc64le) || s390x
// +build linux,arm linux,ppc64 linux,ppc64le s390x

package system

func nativeCharsToString(cstr [65]uint8) string {
	max := len(cstr)
	data := make([]byte, max)

	var pos int
	for ; (pos < max) && (cstr[pos] != 0); pos++ {
		data[pos] = cstr[pos]
	}

	return string(data[0:pos])
}
