package system

type ArchName string

const (
	ArchNameUknown      ArchName = "unknown"
	ArchNameUnsupported ArchName = "unsupported"
	ArchName386         ArchName = "386"
	ArchNameAmd64       ArchName = "amd64"
)

type MachineName string

const (
	MachineNameNamei386   MachineName = "i386"
	MachineNameNamei586   MachineName = "i586"
	MachineNameNamei686   MachineName = "i686"
	MachineNameNamex86_64 MachineName = "x86_64"
	MachineNameNameArm    MachineName = "arm"
	MachineNameNameArm64  MachineName = "arm64"
)

type ArchBits uint8

const (
	ArchBits32 ArchBits = 32
	ArchBits64 ArchBits = 64
)

type ArchFamily string

const (
	ArchFamilyX86 ArchFamily = "x86"
	ArchFamilyArm ArchFamily = "arm"
)

type ArchInfo struct {
	Name   ArchName
	Family ArchFamily
	Bits   ArchBits
}

var x86Family64Arch = ArchInfo{
	Name:   ArchNameAmd64,
	Family: ArchFamilyX86,
	Bits:   ArchBits64,
}

var x86Family32Arch = ArchInfo{
	Name:   ArchName386,
	Family: ArchFamilyX86,
	Bits:   ArchBits32,
}

var unsupportedArch = ArchInfo{
	Name: ArchNameUnsupported,
}

var unknownArch = ArchInfo{
	Name: ArchNameUknown,
}

var archMap = map[MachineName]*ArchInfo{
	MachineNameNamei386:   &x86Family32Arch,
	MachineNameNamei586:   &x86Family32Arch,
	MachineNameNamei686:   &x86Family32Arch,
	MachineNameNamex86_64: &x86Family64Arch,
	MachineNameNameArm:    &unsupportedArch,
	MachineNameNameArm64:  &unsupportedArch,
}

func MachineToArchName(mtype string) ArchName {
	if archInfo, ok := archMap[MachineName(mtype)]; ok {
		return archInfo.Name
	}

	return ArchNameUknown
}

func MachineToArch(mtype string) *ArchInfo {
	if archInfo, ok := archMap[MachineName(mtype)]; ok {
		return archInfo
	}

	return &unknownArch
}
