package system

//NOTES:
//* syscall constants in the "syscall" package are nice, but some syscalls there are missing
//* future versions will include more than just the syscall name
//* 32bit (x86/i386) and 64bit (x86_64) syscall numbers are different

const (
	SyscallX86MinNum      = 0
	SyscallX86UnknownNum  = -1
	SyscallX86UnknownName = "unknown_syscall"
)

type NumberResolverFunc func(uint32) string

func CallNumberResolver(arch ArchName) NumberResolverFunc {
	switch arch {
	case ArchName386:
		return callNameX86Family32
	case ArchNameAmd64:
		return callNameX86Family64
	case ArchNameArm32:
		return callNameArmFamily32
	default:
		return nil
	}
}
