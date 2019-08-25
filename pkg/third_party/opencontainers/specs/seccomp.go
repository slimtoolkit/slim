package specs

// Seccomp represents syscall restrictions
type Seccomp struct {
	DefaultAction Action         `json:"defaultAction"`
	Architectures []Arch         `json:"architectures,omitempty"`
	ArchMap       []Architecture `json:"archMap,omitempty"`
	Syscalls      []*Syscall     `json:"syscalls,omitempty"`
}

//ArchMap - in Docker, but not in the Opencontainers spec (yet)

type Architecture struct {
	Arch      Arch   `json:"architecture"`
	SubArches []Arch `json:"subArchitectures"`
}

// Arch - architecture type
// Additional architectures permitted to be used for system calls
// By default only the native architecture of the kernel is permitted
type Arch string

// Architecture types
const (
	ArchX86     Arch = "SCMP_ARCH_X86"
	ArchX86_64  Arch = "SCMP_ARCH_X86_64"
	ArchX32     Arch = "SCMP_ARCH_X32"
	ArchARM     Arch = "SCMP_ARCH_ARM"
	ArchAARCH64 Arch = "SCMP_ARCH_AARCH64"
)

// Action taken upon Seccomp rule match
type Action string

// Action types
const (
	ActKill  Action = "SCMP_ACT_KILL"
	ActTrap  Action = "SCMP_ACT_TRAP"
	ActErrno Action = "SCMP_ACT_ERRNO"
	ActTrace Action = "SCMP_ACT_TRACE"
	ActAllow Action = "SCMP_ACT_ALLOW"
)

// Operator used to match syscall arguments in Seccomp
type Operator string

// Operator types
const (
	OpNotEqual     Operator = "SCMP_CMP_NE"
	OpLessThan     Operator = "SCMP_CMP_LT"
	OpLessEqual    Operator = "SCMP_CMP_LE"
	OpEqualTo      Operator = "SCMP_CMP_EQ"
	OpGreaterEqual Operator = "SCMP_CMP_GE"
	OpGreaterThan  Operator = "SCMP_CMP_GT"
	OpMaskedEqual  Operator = "SCMP_CMP_MASKED_EQ"
)

// Arg used for matching specific syscall arguments in Seccomp
type Arg struct {
	Index    uint     `json:"index"`
	Value    uint64   `json:"value"`
	ValueTwo uint64   `json:"valueTwo,omitempty"`
	Op       Operator `json:"op"`
}

// Syscall is used to match a syscall in Seccomp
type Syscall struct {
	Name     string   `json:"name,omitempty"`
	Names    []string `json:"names,omitempty"`
	Action   Action   `json:"action"`
	Args     []*Arg   `json:"args,omitempty"`
	Comment  string   `json:"comment,omitempty"`
	Includes Filter   `json:"includes,omitempty"`
	Excludes Filter   `json:"excludes,omitempty"`
}

//Opencontainers spec only includes the 'Names' field
//Docker also includes the old/original 'Name' field
//Docker only: Comment, Includes, Excludes

// Filter is used to conditionally apply Seccomp rules
type Filter struct {
	Caps      []string `json:"caps,omitempty"`
	Arches    []string `json:"arches,omitempty"`
	MinKernel string   `json:"minKernel,omitempty"`
}
