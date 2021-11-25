package system

import (
	"golang.org/x/sys/unix"
)

/*
ARM64 SYSCALL REGISTER USE:

Syscall Number:   x8
Return Value:     x0
1st Param (arg0): x0
2nd Param (arg1): x1
3rd Param (arg2): x2
4th Param (arg3): x3
5th Param (arg4): x4
6th Param (arg5): x5

*/

func LookupCallName(num uint32) string {
	return callNameArmFamily64(num)
}

func LookupCallNumber(name string) (uint32, bool) {
	return callNumberArmFamily64(name)
}

func CallNumber(regs unix.PtraceRegsArm64) uint64 {
	return uint64(regs.Regs[8])
}

func CallReturnValue(regs unix.PtraceRegsArm64) uint64 {
	return uint64(regs.Regs[0])
}

func CallFirstParam(regs unix.PtraceRegsArm64) uint64 {
	return uint64(regs.Regs[0])
}

func CallSecondParam(regs unix.PtraceRegsArm64) uint64 {
	return uint64(regs.Regs[1])
}
