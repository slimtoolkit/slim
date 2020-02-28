package fsutil

import (
	"syscall"
)

type SysStat struct {
	Ok    bool
	Uid   uint32
	Gid   uint32
	Atime syscall.Timespec
	Mtime syscall.Timespec
	Ctime syscall.Timespec
}

/*
Linux:
	Atim      Timespec
	Mtim      Timespec
Mac:
	Atimespec     Timespec
	Mtimespec     Timespec

MAC:
type Stat_t struct {
	Dev           int32
	Mode          uint16
	Nlink         uint16
	Ino           uint64
	Uid           uint32
	Gid           uint32
	Rdev          int32
	Pad_cgo_0     [4]byte
	Atimespec     Timespec
	Mtimespec     Timespec
	Ctimespec     Timespec
	Birthtimespec Timespec
	Size          int64
	Blocks        int64
	Blksize       int32
	Flags         uint32
	Gen           uint32
	Lspare        int32
	Qspare        [2]int64
}
LINUX:
type Stat_t struct {
	Dev       uint64
	Ino       uint64
	Nlink     uint64
	Mode      uint32
	Uid       uint32
	Gid       uint32
	X__pad0   int32
	Rdev      uint64
	Size      int64
	Blksize   int64
	Blocks    int64
	Atim      Timespec
	Mtim      Timespec
	Ctim      Timespec
	X__unused [3]int64
}
*/
