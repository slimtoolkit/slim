//go:build !windows
// +build !windows

package text

func areANSICodesSupported() bool {
	return true
}
