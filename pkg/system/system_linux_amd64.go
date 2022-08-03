package system

import (
	"syscall"
)

/*
AMD64/X86_64 SYSCALL REGISTER USE:

Syscall Number:   rax
Return Value:     rax
1st Param (arg0): rdi
2nd Param (arg1): rsi
3rd Param (arg2): rdx
4th Param (arg3): r10
5th Param (arg4): r8
6th Param (arg5): r9

*/

func LookupCallName(num uint32) string {
	return callNameX86Family64(num)
}

func LookupCallNumber(name string) (uint32, bool) {
	return callNumberX86Family64(name)
}

func CallNumber(regs syscall.PtraceRegs) uint64 {
	return regs.Orig_rax
}

func CallReturnValue(regs syscall.PtraceRegs) uint64 {
	return regs.Rax
}

func CallFirstParam(regs syscall.PtraceRegs) uint64 {
	return regs.Rdi
}

func CallSecondParam(regs syscall.PtraceRegs) uint64 {
	return regs.Rsi
}

func CallThirdParam(regs syscall.PtraceRegs) uint64 {
	return regs.Rdx
}

func CallFourthParam(regs syscall.PtraceRegs) uint64 {
	return regs.Rcx
}

/*
X86_32 SYSCALL REGISTER USE:

Syscall Number:   eax
Return Value:     eax
1st Param (arg0): ebx
2nd Param (arg1): ecx
3rd Param (arg2): edx
4th Param (arg3): esi
5th Param (arg4): edi
6th Param (arg5): ebp

*/
