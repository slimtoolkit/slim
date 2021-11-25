package system

import (
	"syscall"
)

/*
ARM SYSCALL REGISTER USE:

Syscall Number:   r7
Return Value:     r0
1st Param (arg0): r0
2nd Param (arg1): r1
3rd Param (arg2): r2
4th Param (arg3): r3
5th Param (arg4): r4
6th Param (arg5): r5

*/

func LookupCallName(num uint32) string {
	return callNameArmFamily32(num)
}

func LookupCallNumber(name string) (uint32, bool) {
	return callNumberArmFamily32(name)
}

func CallNumber(regs syscall.PtraceRegs) uint64 {
	return uint64(regs.Uregs[7])
}

func CallReturnValue(regs syscall.PtraceRegs) uint64 {
	return uint64(regs.Uregs[0])
}

func CallFirstParam(regs syscall.PtraceRegs) uint64 {
	return uint64(regs.Uregs[0])
}

func CallSecondParam(regs syscall.PtraceRegs) uint64 {
	return uint64(regs.Uregs[1])
}
