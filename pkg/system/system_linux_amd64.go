package system

import (
	"syscall"
)

func CallNumber(regs syscall.PtraceRegs) uint64 {
	return regs.Orig_rax
}

func CallReturnValue(regs syscall.PtraceRegs) uint64 {
	return regs.Rax
}
