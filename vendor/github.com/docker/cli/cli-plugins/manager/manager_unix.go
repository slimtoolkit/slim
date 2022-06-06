//go:build !windows
// +build !windows

package manager

var defaultSystemPluginDirs = []string{
	"/usr/local/lib/docker/cli-plugins", "/usr/local/libexec/docker/cli-plugins",
	"/usr/lib/docker/cli-plugins", "/usr/libexec/docker/cli-plugins",
}
