//go:build windows
// +build windows

package text

import (
	"os"
	"sync"

	"golang.org/x/sys/windows"
)

var enableVTPMutex = sync.Mutex{}

func areANSICodesSupported() bool {
	enableVTPMutex.Lock()
	defer enableVTPMutex.Unlock()

	outHandle := windows.Handle(os.Stdout.Fd())
	var outMode uint32
	if err := windows.GetConsoleMode(outHandle, &outMode); err == nil {
		if outMode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
			return true
		}
		if err := windows.SetConsoleMode(outHandle, outMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err == nil {
			return true
		}
	}
	return false
}
