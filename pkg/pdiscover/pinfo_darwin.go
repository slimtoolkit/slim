package pdiscover

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	CTLKern      = 1
	KernProcArgs = 38
)

func GetOwnProcPath() (string, error) {
	return GetProcPath(os.Getpid())
}

func GetProcPath(pid int) (string, error) {
	nameMib := []int32{CTLKern, KernProcArgs, int32(pid), -1}

	procArgsLen := uintptr(0)

	if err := getSysCtlInfo(nameMib, nil, &procArgsLen); err != nil {
		return "", err
	}

	if procArgsLen == 0 {
		return "", ErrInvalidProcArgsLen
	}

	procArgs := make([]byte, procArgsLen)

	if err := getSysCtlInfo(nameMib, &procArgs[0], &procArgsLen); err != nil {
		return "", err
	}

	if procArgsLen == 0 {
		return "", ErrInvalidProcArgsLen
	}

	exePath := exePathFromProcArgs(procArgs)

	exePath, err := filepath.Abs(exePath)
	if err != nil {
		return exePath, err
	}

	if exePath, err := filepath.EvalSymlinks(exePath); err != nil {
		return exePath, err
	}

	return exePath, nil
}

func getSysCtlInfo(name []int32, value *byte, valueLen *uintptr) error {
	_, _, errNum := syscall.Syscall6(syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&name[0])),
		uintptr(len(name)),
		uintptr(unsafe.Pointer(value)),
		uintptr(unsafe.Pointer(valueLen)),
		0, 0)
	if errNum != 0 {
		return errNum
	}

	return nil
}

func GetProcInfo(pid int) map[string]string {
	return nil
}

func exePathFromProcArgs(raw []byte) string {
	for i, byteVal := range raw {
		if byteVal == 0 {
			return string(raw[:i])
		}
	}

	return ""
}
