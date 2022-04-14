//go:build !windows
// +build !windows

package sshprovider

import (
	"github.com/pkg/errors"
)

func getFallbackAgentPath() (string, error) {
	return "", errors.Errorf("make sure SSH_AUTH_SOCK is set")
}

func getWindowsPipeDialer(path string) *socketDialer {
	return nil
}
