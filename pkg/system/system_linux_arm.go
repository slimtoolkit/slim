package system

import (
	"syscall"
)

func CallNumber(regs syscall.PtraceRegs) uint64 {
	return uint64(regs.Uregs[7])
}

func CallReturnValue(regs syscall.PtraceRegs) uint64 {
	return uint64(regs.Uregs[0])
}
