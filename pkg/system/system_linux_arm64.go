package system

import (
	"golang.org/x/sys/unix"
)

func CallNumber(regs unix.PtraceRegsArm64) uint64 {
	return uint64(regs.Regs[8])
}

func CallReturnValue(regs unix.PtraceRegsArm64) uint64 {
	return uint64(regs.Regs[0])
}
