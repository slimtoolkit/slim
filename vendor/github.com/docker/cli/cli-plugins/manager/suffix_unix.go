//go:build !windows
// +build !windows

package manager

func trimExeSuffix(s string) (string, error) {
	return s, nil
}
func addExeSuffix(s string) string {
	return s
}
