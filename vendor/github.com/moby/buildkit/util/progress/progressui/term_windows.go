//go:build windows
// +build windows

package progressui

import "github.com/morikuni/aec"

var (
	colorRun    = aec.CyanF
	colorCancel = aec.YellowF
	colorError  = aec.RedF
)
