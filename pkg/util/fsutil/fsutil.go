package fsutil

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/slimtoolkit/slim/pkg/pdiscover"
	"github.com/slimtoolkit/slim/pkg/util/errutil"

	"github.com/bmatcuk/doublestar"
	log "github.com/sirupsen/logrus"
)

// File permission bits (execute bits only)
const (
	FilePermUserExe  = 0100
	FilePermGroupExe = 0010
	FilePermOtherExe = 0001
)

// Native FileMode special bits mask
const FMSpecialBits = os.ModeSticky | os.ModeSetgid | os.ModeSetuid

// Native FileModes for extra flags
const (
	FMSticky = 01000
	FMSetgid = 02000
	FMSetuid = 04000
)

// Native FileMode bits for extra flags
const (
	BitSticky = 1
	BitSetgid = 2
	BitSetuid = 4
)

// Directory and file related errors
var (
	ErrNoFileData                = errors.New("no file data")
	ErrNoSrcDir                  = errors.New("no source directory path")
	ErrNoDstDir                  = errors.New("no destination directory path")
	ErrSameDir                   = errors.New("source and destination directories are the same")
	ErrSrcDirNotExist            = errors.New("source directory doesn't exist")
	ErrSrcNotDir                 = errors.New("source is not a directory")
	ErrSrcNotRegularFile         = errors.New("source is not a regular file")
	ErrUnsupportedFileObjectType = errors.New("unsupported file object type")
)

// FileModeExtraUnix2Go converts the standard unix filemode for the extra flags to the Go version
func FileModeExtraUnix2Go(mode uint32) os.FileMode {
	switch mode {
	case FMSticky:
		return os.ModeSticky
	case FMSetgid:
		return os.ModeSetgid
	case FMSetuid:
		return os.ModeSetuid
	}

	return 0
}

// FileModeExtraBitUnix2Go converts the standard unix filemode bit for the extra flags to the filemode in Go
func FileModeExtraBitUnix2Go(bit uint32) os.FileMode {
	switch bit {
	case BitSticky:
		return os.ModeSticky
	case BitSetgid:
		return os.ModeSetgid
	case BitSetuid:
		return os.ModeSetuid
	}

	return 0
}

// FileModeExtraBitsUnix2Go converts the standard unix filemode bits for the extra flags to the filemode flags in Go
func FileModeExtraBitsUnix2Go(bits uint32) os.FileMode {
	var mode os.FileMode

	if bits&BitSticky != 0 {
		mode |= os.ModeSticky
	}

	if bits&BitSetgid != 0 {
		mode |= os.ModeSetgid
	}

	if bits&BitSetuid != 0 {
		mode |= os.ModeSetuid
	}

	return mode
}

// FileModeIsSticky checks if FileMode has the sticky bit set
func FileModeIsSticky(mode os.FileMode) bool {
	if mode&os.ModeSticky != 0 {
		return true
	}

	return false
}

// FileModeIsSetgid checks if FileMode has the setgid bit set
func FileModeIsSetgid(mode os.FileMode) bool {
	if mode&os.ModeSetgid != 0 {
		return true
	}

	return false
}

// FileModeIsSetuid checks if FileMode has the setuid bit set
func FileModeIsSetuid(mode os.FileMode) bool {
	if mode&os.ModeSetuid != 0 {
		return true
	}

	return false
}

const (
	rootStateKey           = ".slim-state"
	releasesStateKey       = "releases"
	imageStateBaseKey      = "images"
	imageStateArtifactsKey = "artifacts"
	stateArtifactsPerms    = 0777
	releaseArtifactsPerms  = 0740
)

var badInstallPaths = [...]string{
	"/usr/local/bin",
	"/usr/local/sbin",
	"/usr/bin",
	"/usr/sbin",
	"/bin",
	"/sbin",
}

const (
	tmpPath        = "/tmp"
	stateTmpPath   = "/tmp/slim-state"
	sensorFileName = "slim-sensor"
)

// AccessInfo provides the file object access properties
type AccessInfo struct {
	Flags     os.FileMode
	PermsOnly bool
	UID       int
	GID       int
}

func NewAccessInfo() *AccessInfo {
	return &AccessInfo{
		Flags: 0,
		UID:   -1,
		GID:   -1,
	}
}

// Remove removes the artifacts generated during the current application execution
func Remove(artifactLocation string) error {
	return os.RemoveAll(artifactLocation)
}

// Touch creates the target file or updates its timestamp
func Touch(target string) error {
	targetDirPath := FileDir(target)
	if _, err := os.Stat(targetDirPath); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(targetDirPath, 0777)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	tf, err := os.OpenFile(target, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	tf.Close()

	tnow := time.Now().UTC()
	err = os.Chtimes(target, tnow, tnow)
	if err != nil {
		return err
	}

	return nil
}

// Exists returns true if the target file system object exists
func Exists(target string) bool {
	if _, err := os.Stat(target); err != nil {
		return false
	}

	return true
}

// DirExists returns true if the target exists and it's a directory
func DirExists(target string) bool {
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		return true
	}

	return false
}

// IsDir returns true if the target file system object is a directory
func IsDir(target string) bool {
	info, err := os.Stat(target)
	if err != nil {
		return false
	}

	return info.IsDir()
}

// IsRegularFile returns true if the target file system object is a regular file
func IsRegularFile(target string) bool {
	info, err := os.Lstat(target)
	if err != nil {
		return false
	}

	return info.Mode().IsRegular()
}

// IsSymlink returns true if the target file system object is a symlink
func IsSymlink(target string) bool {
	info, err := os.Lstat(target)
	if err != nil {
		return false
	}

	return (info.Mode() & os.ModeSymlink) == os.ModeSymlink
}

// IsTarFile returns true if the target file system object is a tar archive
func IsTarFile(target string) bool {
	tf, err := os.Open(target)
	if err != nil {
		log.Debugf("fsutil.IsTarFile(%s): error - %v", target, err)
		return false
	}

	defer tf.Close()
	tr := tar.NewReader(tf)
	_, err = tr.Next()
	if err != nil {
		log.Debugf("fsutil.IsTarFile(%s): error - %v", target, err)
		return false
	}

	return true
}

func HasReadAccess(dst string) (bool, error) {
	err := unix.Access(dst, unix.R_OK)
	if err == nil {
		return true, nil
	}

	if err == unix.EACCES {
		return false, nil
	}

	return false, err
}

func HasWriteAccess(dst string) (bool, error) {
	err := unix.Access(dst, unix.W_OK)
	if err == nil {
		return true, nil
	}

	if err == unix.EACCES {
		return false, nil
	}

	return false, err
}

// SetAccess updates the access permissions on the destination
func SetAccess(dst string, access *AccessInfo) error {
	if dst == "" || access == nil {
		return nil
	}

	if access.Flags != 0 {
		dstInfo, err := os.Stat(dst)
		if err != nil {
			return err
		}

		fmode := dstInfo.Mode()
		fmode = fmode &^ os.ModePerm
		fmode |= (access.Flags & os.ModePerm)

		if !access.PermsOnly {
			fmode = fmode &^ FMSpecialBits
			fmode |= (access.Flags & FMSpecialBits)
		}

		if err := os.Chmod(dst, fmode); err != nil {
			return err
		}
	}

	if access.UID > -1 || access.GID > -1 {
		if err := os.Chown(dst, access.UID, access.GID); err != nil {
			return err
		}
	}

	return nil
}

// CopyFile copies the source file system object to the desired destination
func CopyFile(clone bool, src, dst string, makeDir bool) error {
	log.Debugf("CopyFile(%v,%v,%v,%v)", clone, src, dst, makeDir)

	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	switch {
	case info.Mode().IsRegular():
		return CopyRegularFile(clone, src, dst, makeDir)
	case (info.Mode() & os.ModeSymlink) == os.ModeSymlink:
		return CopySymlinkFile(clone, src, dst, makeDir)
	default:
		return ErrUnsupportedFileObjectType
	}
}

// CopySymlinkFile copies a symlink file
func CopySymlinkFile(clone bool, src, dst string, makeDir bool) error {
	log.Debugf("CopySymlinkFile(%v,%v,%v)", src, dst, makeDir)

	if makeDir {
		//srcDirName := FileDir(src)
		dstDirName := FileDir(dst)

		if _, err := os.Stat(dstDirName); err != nil {
			if os.IsNotExist(err) {
				var dirMode os.FileMode = 0777
				//need to make it work for non-default user use cases
				//if clone {
				//	srcDirInfo, err := os.Stat(srcDirName)
				//	if err != nil {
				//		return err
				//	}
				//
				//	dirMode = srcDirInfo.Mode()
				//}

				err = os.MkdirAll(dstDirName, dirMode)
				if err != nil {
					return err
				}

			} else {
				return err
			}
		}
	}

	linkRef, err := os.Readlink(src)
	if err != nil {
		return err
	}

	err = os.Symlink(linkRef, dst)
	if err != nil {
		return err
	}

	if clone {
		srcInfo, err := os.Lstat(src)
		if err != nil {
			return err
		}

		if sysStat, ok := srcInfo.Sys().(*syscall.Stat_t); ok {
			ssi := SysStatInfo(sysStat)
			if ssi.Ok {
				if err := UpdateSymlinkTimes(dst, ssi.Atime, ssi.Mtime); err != nil {
					log.Warnf("CopySymlinkFile(%v,%v) - UpdateSymlinkTimes error", src, dst)
				}

				if err := os.Lchown(dst, int(ssi.Uid), int(ssi.Gid)); err != nil {
					log.Warnf("CopySymlinkFile(%v,%v)- unable to change owner", src, dst)
				}
			}
		} else {
			log.Warnf("CopySymlinkFile(%v,%v)- unable to get Stat_t", src, dst)
		}
	}

	return nil
}

type dirInfo struct {
	src   string
	dst   string
	perms os.FileMode
	sys   SysStat
}

func cloneDirPath(src, dst string) {
	src, err := filepath.Abs(src)
	if err != nil {
		errutil.FailOn(err)
	}
	dst, err = filepath.Abs(dst)
	if err != nil {
		errutil.FailOn(err)
	}

	var dirs []dirInfo
	for {
		if src == "/" {
			break
		}

		srcDirName := filepath.Base(src)
		dstDirName := filepath.Base(dst)

		if srcDirName != dstDirName {
			break
		}

		srcInfo, err := os.Stat(src)
		if err != nil {
			errutil.FailOn(err)
		}

		if !srcInfo.IsDir() {
			errutil.Fail("not a directory")
		}

		if Exists(dst) {
			break
		}

		di := dirInfo{
			src:   src,
			dst:   dst,
			perms: srcInfo.Mode(),
		}

		if sysStat, ok := srcInfo.Sys().(*syscall.Stat_t); ok {
			di.sys = SysStatInfo(sysStat)
		}

		dirs = append([]dirInfo{di}, dirs...)

		src = FileDir(src)
		dst = FileDir(dst)
	}

	for _, dir := range dirs {
		if Exists(dir.dst) {
			log.Debugf("cloneDirPath() - dst dir exists - %v", dir.dst)
			continue
		}

		//using os.MkdirAll instead of os.Mkdir to make sure we don't miss any intermediate directories
		//need to research when we might miss intermediate directories
		err = os.MkdirAll(dir.dst, 0777)
		if err != nil {
			log.Errorf("cloneDirPath() - os.MkdirAll(%v) error - %v", dir.dst, err)
		}
		//if err != nil && !os.IsExist(err) {
		//	errutil.FailOn(err)
		//}

		if err == nil {
			if err := os.Chmod(dir.dst, dir.perms); err != nil {
				log.Warnf("cloneDirPath() - unable to set perms (%v) - %v", dir.dst, err)
			}

			if dir.sys.Ok {
				if err := UpdateFileTimes(dir.dst, dir.sys.Atime, dir.sys.Mtime); err != nil {
					log.Warnf("cloneDirPath() - UpdateFileTimes error (%v) - %v", dir.dst, err)
				}

				if err := os.Chown(dir.dst, int(dir.sys.Uid), int(dir.sys.Gid)); err != nil {
					log.Warnf("cloneDirPath()- unable to change owner (%v) - %v", dir.dst, err)
				}
			}
		}
	}
}

// CopyRegularFile copies a regular file
func CopyRegularFile(clone bool, src, dst string, makeDir bool) error {
	log.Debugf("CopyRegularFile(%v,%v,%v,%v)", clone, src, dst, makeDir)
	//'clone' should be true only for the dst files that need to clone the dir properties from src
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	srcFileInfo, err := s.Stat()
	if err != nil {
		return err
	}

	if !srcFileInfo.Mode().IsRegular() {
		return ErrSrcNotRegularFile
	}

	if makeDir {
		dstDirPath := FileDir(dst)
		if _, err := os.Stat(dstDirPath); err != nil {
			if os.IsNotExist(err) {
				srcDirPath := FileDir(src)

				if clone {
					cloneDirPath(srcDirPath, dstDirPath)
				} else {
					var dirMode os.FileMode = 0777
					err = os.MkdirAll(dstDirPath, dirMode)
					if err != nil {
						return err
					}

					//try copying the timestamps too (even without cloning)
					srcDirInfo, err := os.Stat(srcDirPath)
					if err == nil {
						if sysStat, ok := srcDirInfo.Sys().(*syscall.Stat_t); ok {
							ssi := SysStatInfo(sysStat)
							if ssi.Ok {
								if err := UpdateFileTimes(dstDirPath, ssi.Atime, ssi.Mtime); err != nil {
									log.Warnf("CopyRegularFile() - UpdateFileTimes(%v) error - %v", dstDirPath, err)
								}
							}
						}
					} else {
						log.Warnf("CopyRegularFile() - os.Stat(%v) error - %v", srcDirPath, err)
					}
				}
			} else {
				return err
			}
		}
	}

	d, err := os.Create(dst)
	if err != nil {
		return err
	}

	if srcFileInfo.Size() > 0 {
		written, err := io.Copy(d, s)
		if err != nil {
			d.Close()
			return err
		}

		if written != srcFileInfo.Size() {
			log.Debugf("CopyRegularFile(%v,%v,%v) - copy data mismatch - %v/%v",
				src, dst, makeDir, written, srcFileInfo.Size())
			d.Close()
			return fmt.Errorf("%s -> %s: partial copy - %d/%d",
				src, dst, written, srcFileInfo.Size())
		}
	}

	//Need to close dst file before chmod works the right way
	if err := d.Close(); err != nil {
		log.Debugf("CopyRegularFile() - d.Close error - %v", err)
		return err
	}

	if clone {
		if err := os.Chmod(dst, srcFileInfo.Mode()); err != nil {
			log.Warnf("CopyRegularFile(%v,%v) - unable to set mode", src, dst)
			return err
		}

		if sysStat, ok := srcFileInfo.Sys().(*syscall.Stat_t); ok {
			ssi := SysStatInfo(sysStat)
			if ssi.Ok {
				if err := UpdateFileTimes(dst, ssi.Atime, ssi.Mtime); err != nil {
					log.Warnf("CopyRegularFile(%v,%v) - UpdateFileTimes error", src, dst)
				}

				if err := os.Chown(dst, int(ssi.Uid), int(ssi.Gid)); err != nil {
					log.Warnf("CopyRegularFile(%v,%v)- unable to change owner", src, dst)
				}
			}
		} else {
			log.Warnf("CopyRegularFile(%v,%v)- unable to get Stat_t", src, dst)
		}
	} else {
		if err := os.Chmod(dst, 0777); err != nil {
			log.Warnf("CopyRegularFile(%v,%v) - unable to set mode", src, dst)
		}

		if sysStat, ok := srcFileInfo.Sys().(*syscall.Stat_t); ok {
			ssi := SysStatInfo(sysStat)
			if ssi.Ok {
				if err := UpdateFileTimes(dst, ssi.Atime, ssi.Mtime); err != nil {
					log.Warnf("CopyRegularFile(%v,%v) - UpdateFileTimes error", src, dst)
				}
			}
		} else {
			log.Warnf("CopyRegularFile(%v,%v)- unable to get Stat_t", src, dst)
		}
	}

	return nil
}

// CopyAndObfuscateFile copies a regular file and performs basic file reference obfuscation
func CopyAndObfuscateFile(
	clone bool,
	src string,
	dst string,
	makeDir bool) error {
	log.Debugf("CopyAndObfuscateFile(%v,%v,%v,%v)", clone, src, dst, makeDir)

	//need to preserve the extension because some of the app stacks
	//depend on it for its compile/run time behavior
	base := filepath.Base(dst)
	ext := filepath.Ext(base)
	base = strings.ReplaceAll(base, ".", "..")
	base = fmt.Sprintf(".d.%s", base)
	if ext != "" {
		base = fmt.Sprintf("%s%s", base, ext)
	}

	dirPart := filepath.Dir(dst)
	dstData := filepath.Join(dirPart, base)
	err := CopyRegularFile(clone, src, dstData, makeDir)
	if err != nil {
		return err
	}

	err = os.Symlink(base, dst)
	if err != nil {
		return err
	}

	return nil
}

// AppendToFile appends the provided data to the target file
func AppendToFile(target string, data []byte, preserveTimes bool) error {
	if target == "" || len(data) == 0 {
		return nil
	}

	tfi, err := os.Stat(target)
	if err != nil {
		return err
	}

	var ssi SysStat
	if rawSysStat, ok := tfi.Sys().(*syscall.Stat_t); ok {
		ssi = SysStatInfo(rawSysStat)
	}

	tf, err := os.OpenFile(target, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	defer tf.Close()

	_, err = tf.Write(data)
	if err != nil {
		return err
	}

	if preserveTimes && ssi.Ok {
		if err := UpdateFileTimes(target, ssi.Atime, ssi.Mtime); err != nil {
			log.Warnf("AppendToFile(%v) - UpdateFileTimes error", target)
		}
	}

	return nil
}

type ReplaceInfo struct {
	PathSuffix   string
	IsMatchRegex string
	Match        string
	Replace      string
}

// ReplaceFileData replaces the selected file bytes with the caller provided bytes
func ReplaceFileData(target string, replace []ReplaceInfo, preserveTimes bool) error {
	if target == "" || len(replace) == 0 {
		return nil
	}

	tfi, err := os.Stat(target)
	if err != nil {
		return err
	}

	var ssi SysStat
	if rawSysStat, ok := tfi.Sys().(*syscall.Stat_t); ok {
		ssi = SysStatInfo(rawSysStat)
	}

	raw, err := os.ReadFile(target)
	if err != nil {
		return err
	}

	var replaced bool
	for _, r := range replace {
		if r.PathSuffix != "" {
			if !strings.HasSuffix(target, r.PathSuffix) {
				continue
			}
		}

		if r.Match == "" || r.Replace == "" {
			continue
		}

		raw = bytes.ReplaceAll(raw, []byte(r.Match), []byte(r.Replace))
		replaced = true
	}

	if replaced {
		err = os.WriteFile(target, raw, 0644)
		if err != nil {
			return err
		}

		if preserveTimes && ssi.Ok {
			if err := UpdateFileTimes(target, ssi.Atime, ssi.Mtime); err != nil {
				log.Warnf("ReplaceFileData(%v) - UpdateFileTimes error", target)
			}
		}
	}

	return nil
}

type DataUpdaterFn func(target string, data []byte) ([]byte, error)

// UpdateFileData updates all file data in target file using the updater function
func UpdateFileData(target string, updater DataUpdaterFn, preserveTimes bool) error {
	if target == "" || updater == nil {
		return nil
	}

	tfi, err := os.Stat(target)
	if err != nil {
		return err
	}

	var ssi SysStat
	if rawSysStat, ok := tfi.Sys().(*syscall.Stat_t); ok {
		ssi = SysStatInfo(rawSysStat)
	}

	raw, err := os.ReadFile(target)
	if err != nil {
		return err
	}

	raw, err = updater(target, raw)
	if err != nil {
		return err
	}

	err = os.WriteFile(target, raw, 0644)
	if err != nil {
		return err
	}

	if preserveTimes && ssi.Ok {
		if err := UpdateFileTimes(target, ssi.Atime, ssi.Mtime); err != nil {
			log.Warnf("ReplaceFileData(%v) - UpdateFileTimes error", target)
		}
	}

	return nil
}

func copyFileObjectHandler(
	clone bool,
	srcBase, dstBase string,
	copyRelPath, skipErrors bool,
	excludePatterns []string, ignoreDirNames, ignoreFileNames map[string]struct{},
	errs *[]error) filepath.WalkFunc {
	var foCount uint64

	return func(path string, info os.FileInfo, err error) error {
		foCount++

		if err != nil {

			if skipErrors {
				*errs = append(*errs, err)
				return nil
			}

			return err
		}

		foBase := filepath.Base(path)

		var isIgnored bool
		for _, xpattern := range excludePatterns {
			found, err := doublestar.Match(xpattern, path)
			if err != nil {
				log.Warnf("copyFileObjectHandler - [%v] excludePatterns Match error - %v\n", path, err)
				//should only happen when the pattern is malformed
				continue
			}
			if found {
				isIgnored = true
				break
			}
		}
		/*
			if _, ok := ignorePaths[path]; ok {
				isIgnored = true
			}

			for prefix := range ignorePrefixes {
				if strings.HasPrefix(path, prefix) {
					isIgnored = true
					break
				}
			}
		*/

		var targetPath string
		if copyRelPath {
			targetPath = filepath.Join(dstBase, strings.TrimPrefix(path, srcBase))
		} else {
			targetPath = filepath.Join(dstBase, path)
		}

		switch {
		case info.Mode().IsDir():
			if isIgnored {
				log.Debugf("dir path (%v) is ignored (skipping dir)...", path)
				return filepath.SkipDir
			}

			//todo: refactor
			if _, ok := ignoreDirNames[foBase]; ok {
				log.Debugf("dir name (%v) in ignoreDirNames list (skipping dir)...", foBase)
				return filepath.SkipDir
			}

			if _, err := os.Stat(targetPath); err != nil {
				if os.IsNotExist(err) {
					if clone {
						cloneDirPath(path, targetPath)
					} else {
						err = os.MkdirAll(targetPath, 0777)
						if err != nil {
							if skipErrors {
								*errs = append(*errs, err)
								return nil
							}

							return err
						}

						srcDirInfo, err := os.Stat(path)
						if err == nil {
							if sysStat, ok := srcDirInfo.Sys().(*syscall.Stat_t); ok {
								ssi := SysStatInfo(sysStat)
								if ssi.Ok {
									if err := UpdateFileTimes(targetPath, ssi.Atime, ssi.Mtime); err != nil {
										log.Warnf("copyFileObjectHandler() - UpdateFileTimes(%v) error - %v", targetPath, err)
									}
								}
							}
						} else {
							log.Warnf("copyFileObjectHandler() - os.Stat(%v) error - %v", path, err)
						}
					}
				} else {
					log.Warnf("copyFileObjectHandler() - os.Stat(%v) error - %v", targetPath, err)
				}
			}

		case info.Mode().IsRegular():
			if isIgnored {
				log.Debugf("file path (%v) is ignored (skipping file)...", path)
				return nil
			}

			//todo: refactor
			if _, ok := ignoreFileNames[foBase]; ok {
				log.Debugf("file name (%v) in ignoreFileNames list (skipping file)...", foBase)
				return nil
			}

			err = CopyRegularFile(clone, path, targetPath, true)
			if err != nil {
				if skipErrors {
					*errs = append(*errs, err)
					return nil
				}

				return err
			}
		case (info.Mode() & os.ModeSymlink) == os.ModeSymlink:
			if isIgnored {
				log.Debugf("link path (%v) is ignored (skipping file)...", path)
				return nil
			}

			//todo: refactor
			if _, ok := ignoreFileNames[foBase]; ok {
				log.Debugf("link file name (%v) in ignoreFileNames list (skipping file)...", foBase)
				return nil
			}

			//TODO: should call CopySymlinkFile() here (instead of using os.Readlink/os.Symlink directly)

			//TODO: (add a flag)
			//to make it more generic need to support absolute path link rewriting
			//if they point to other copied file objects
			linkRef, err := os.Readlink(path)
			if err != nil {
				if skipErrors {
					*errs = append(*errs, err)
					return nil
				}

				return err
			}

			err = os.Symlink(linkRef, targetPath)
			if err != nil {
				if skipErrors {
					*errs = append(*errs, err)
					return nil
				}

				return err
			}
		default:
			log.Debug("is other file object type... (ignoring)")
		}

		return nil
	}
}

// CopyDirOnly copies a directory without any files
func CopyDirOnly(clone bool, src, dst string) error {
	log.Debugf("CopyDirOnly(%v,%v,%v)", clone, src, dst)

	if src == "" {
		return ErrNoSrcDir
	}

	if dst == "" {
		return ErrNoDstDir
	}

	var err error
	if src, err = filepath.Abs(src); err != nil {
		return err
	}

	if dst, err = filepath.Abs(dst); err != nil {
		return err
	}

	if src == dst {
		return ErrSameDir
	}

	//TODO: better symlink support
	//when 'src' is a directory (need to better define the expected behavior)
	//should use Lstat() first
	srcInfo, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrSrcDirNotExist
		}

		return err
	}

	if !srcInfo.IsDir() {
		return ErrSrcNotDir
	}

	if !DirExists(dst) {
		if clone {
			cloneDirPath(src, dst)
		} else {
			var dirMode os.FileMode = 0777
			err = os.MkdirAll(dst, dirMode)
			if err != nil {
				return err
			}

			//try copying the timestamps too (even without cloning)
			if sysStat, ok := srcInfo.Sys().(*syscall.Stat_t); ok {
				ssi := SysStatInfo(sysStat)
				if ssi.Ok {
					if err := UpdateFileTimes(dst, ssi.Atime, ssi.Mtime); err != nil {
						log.Warnf("CopyDirOnly() - UpdateFileTimes(%v) error - %v", dst, err)
					}
				}
			}
		}
	}

	return nil
}

// CopyDir copies a directory
func CopyDir(clone bool,
	src string,
	dst string,
	copyRelPath bool,
	skipErrors bool,
	excludePatterns []string,
	ignoreDirNames map[string]struct{},
	ignoreFileNames map[string]struct{}) (error, []error) {
	log.Debugf("CopyDir(%v,%v,%v,%v,%#v,...)", src, dst, copyRelPath, skipErrors, excludePatterns)

	if src == "" {
		return ErrNoSrcDir, nil
	}

	if dst == "" {
		return ErrNoDstDir, nil
	}

	var err error
	if src, err = filepath.Abs(src); err != nil {
		return err, nil
	}

	if dst, err = filepath.Abs(dst); err != nil {
		return err, nil
	}

	if src == dst {
		return ErrSameDir, nil
	}

	//TODO: better symlink support
	//when 'src' is a directory (need to better define the expected behavior)
	//should use Lstat() first
	srcInfo, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrSrcDirNotExist, nil
		}

		return err, nil
	}

	if !srcInfo.IsDir() {
		return ErrSrcNotDir, nil
	}

	//TODO: should clone directory permission, ownership and timestamps info

	var errs []error
	err = filepath.Walk(src, copyFileObjectHandler(
		clone, src, dst, copyRelPath, skipErrors, excludePatterns, ignoreDirNames, ignoreFileNames, &errs))
	if err != nil {
		return err, nil
	}

	return nil, errs
}

///////////////////////////////////////////////////////////////////////////////

func FileMode(fileName string) string {
	finfo, err := os.Lstat(fileName)
	if err != nil {
		log.Errorf("fsutil.FileMode(%s) - os.Lstat error - %v", fileName, err)
		return ""
	}

	return finfo.Mode().String()
}

// ExeDir returns the directory information for the application
func ExeDir() string {
	exePath, err := pdiscover.GetOwnProcPath()
	errutil.FailOn(err)
	return filepath.Dir(exePath)
}

// FileDir returns the directory information for the given file
func FileDir(fileName string) string {
	abs, err := filepath.Abs(fileName)
	errutil.FailOn(err)
	return filepath.Dir(abs)
}

// PreparePostUpdateStateDir ensures that the updated sensor is copied to the state directory if necessary
func PreparePostUpdateStateDir(statePrefix string) {
	log.Debugf("PreparePostUpdateStateDir(%v)", statePrefix)

	appDir := ExeDir()
	if statePrefix == "" {
		statePrefix = appDir
	}

	for _, badPath := range badInstallPaths {
		if appDir == badPath {
			if pinfo, err := os.Stat(tmpPath); err == nil && pinfo.IsDir() {
				log.Debugf("PreparePostUpdateStateDir - overriding state path to %v", stateTmpPath)

				srcSensorPath := filepath.Join(appDir, sensorFileName)
				dstSensorPath := filepath.Join(stateTmpPath, sensorFileName)
				if Exists(dstSensorPath) {
					log.Debugf("PreparePostUpdateStateDir - remove existing sensor binary - %v", dstSensorPath)
					if err := Remove(dstSensorPath); err != nil {
						log.Debugf("PreparePostUpdateStateDir - error removing existing sensor binary - %v", err)
					}
				}

				err = CopyRegularFile(false, srcSensorPath, dstSensorPath, true)
				errutil.FailOn(err)
			} else {
				log.Debugf("PreparePostUpdateStateDir - did not find tmp")
			}
		}
	}
}

// ResolveImageStateBasePath resolves the base path for the state path
func ResolveImageStateBasePath(statePrefix string) string {
	log.Debugf("ResolveImageStateBasePath(%s)", statePrefix)

	appDir := ExeDir()
	if statePrefix == "" {
		statePrefix = appDir
	}

	for _, badPath := range badInstallPaths {
		if strings.HasPrefix(statePrefix, badPath) {
			log.Debugf("ResolveImageStateBasePath - statePrefix=%s appDir=%s badPath=%s", statePrefix, appDir, badPath)
			if pinfo, err := os.Stat(tmpPath); err == nil && pinfo.IsDir() {
				log.Debugf("ResolveImageStateBasePath - overriding state path to %v", stateTmpPath)
				statePrefix = stateTmpPath
			} else {
				log.Debugf("ResolveImageStateBasePath - did not find tmp")
			}

			break
		}
	}

	return statePrefix
}

// PrepareImageStateDirs ensures that the required application directories exist
func PrepareImageStateDirs(statePrefix, imageID string) (string, string, string, string) {
	//prepares the image processing directories
	//creating the root state directory if it doesn't exist
	log.Debugf("PrepareImageStateDirs(%v,%v)", statePrefix, imageID)

	stateKey := imageID
	//images IDs in Docker 1.9+ are prefixed with a hash type...
	if strings.Contains(stateKey, ":") {
		parts := strings.Split(stateKey, ":")
		stateKey = parts[1]
	}

	appDir := ExeDir()
	if statePrefix == "" {
		statePrefix = appDir
	}

	for _, badPath := range badInstallPaths {
		//Note:
		//Should be a prefix check ideally
		//and should check if it's actually one of the 'shared' directories in Docker for Mac (on Macs)
		if statePrefix == badPath {
			log.Debugf("PrepareImageStateDirs - statePrefix=%s appDir=%s badPath=%s", statePrefix, appDir, badPath)
			if pinfo, err := os.Stat(tmpPath); err == nil && pinfo.IsDir() {
				log.Debugf("PrepareImageStateDirs - overriding state path to %v", stateTmpPath)
				statePrefix = stateTmpPath
			} else {
				log.Debugf("PrepareImageStateDirs - did not find tmp")
			}
		}

		if appDir == badPath {
			log.Debugf("PrepareImageStateDirs - statePrefix=%s appDir=%s badPath=%s", statePrefix, appDir, badPath)
			if pinfo, err := os.Stat(tmpPath); err == nil && pinfo.IsDir() {
				log.Debugf("PrepareImageStateDirs - copying sensor to state path (to %v)", stateTmpPath)

				srcSensorPath := filepath.Join(appDir, sensorFileName)
				dstSensorPath := filepath.Join(statePrefix, sensorFileName)
				err = CopyRegularFile(false, srcSensorPath, dstSensorPath, true)
				errInfo := map[string]string{
					"op":            "PrepareImageStateDirs",
					"call":          "CopyRegularFile",
					"srcSensorPath": srcSensorPath,
					"dstSensorPath": dstSensorPath,
				}

				errutil.FailOnWithInfo(err, errInfo)
			} else {
				log.Debugf("PrepareImageStateDirs - did not find tmp")
			}
		}
	}

	localVolumePath := filepath.Join(statePrefix, rootStateKey, imageStateBaseKey, stateKey)
	artifactLocation := filepath.Join(localVolumePath, imageStateArtifactsKey)
	artifactDir, err := os.Stat(artifactLocation)

	switch {
	case err == nil:
		log.Debugf("PrepareImageStateDirs - removing existing state location: %v", artifactLocation)
		err = Remove(artifactLocation)
		if err != nil {
			log.Debugf("PrepareImageStateDirs - failed to remove existing state location: %v", artifactLocation)
			errutil.FailOn(err)
		}
	case os.IsNotExist(err):
		log.Debugf("PrepareImageStateDirs - will create new state location: %v", artifactLocation)
	default:
		errutil.FailOn(err)
	}

	err = os.MkdirAll(artifactLocation, stateArtifactsPerms)
	errutil.FailOn(err)
	artifactDir, err = os.Stat(artifactLocation)
	errutil.FailOn(err)
	log.Debug("PrepareImageStateDirs - created new image state location: ", artifactLocation)

	errutil.FailWhen(!artifactDir.IsDir(), "artifact location is not a directory")

	return localVolumePath, artifactLocation, statePrefix, stateKey
}

// PrepareReleaseStateDirs ensures that the required app release directories exist
func PrepareReleaseStateDirs(statePrefix, version string) (string, string) {
	//prepares the app release directories (used to update the app binaries)
	//creating the root state directory if it doesn't exist
	log.Debugf("PrepareReleaseStateDirs(%v,%v)", statePrefix, version)

	if statePrefix == "" {
		statePrefix = ExeDir()
	}

	for _, badPath := range badInstallPaths {
		if statePrefix == badPath {
			if pinfo, err := os.Stat(tmpPath); err == nil && pinfo.IsDir() {
				log.Debugf("PrepareReleaseStateDirs - overriding state path to %v", stateTmpPath)
				statePrefix = stateTmpPath
			} else {
				log.Debugf("PrepareReleaseStateDirs - did not find tmp")
			}
		}
	}

	releaseDirPath := filepath.Join(statePrefix, rootStateKey, releasesStateKey, version)
	releaseDir, err := os.Stat(releaseDirPath)

	switch {
	case err == nil:
		log.Debugf("PrepareReleaseStateDirs - release state location already exists: %v", releaseDirPath)
		//not deleting existing release artifacts (todo: revisit this feature in the future)
	case os.IsNotExist(err):
		log.Debugf("PrepareReleaseStateDirs - will create new release state location: %v", releaseDirPath)
	default:
		errutil.FailOn(err)
	}

	err = os.MkdirAll(releaseDirPath, releaseArtifactsPerms)
	errutil.FailOn(err)
	releaseDir, err = os.Stat(releaseDirPath)
	errutil.FailOn(err)
	log.Debug("PrepareReleaseStateDirs - created new release state location: ", releaseDirPath)

	errutil.FailWhen(!releaseDir.IsDir(), "release state location is not a directory")

	return releaseDirPath, statePrefix
}

/* use - TBD
func createDummyFile(src, dst string) error {
	_, err := os.Stat(dst)
	if err != nil && os.IsNotExist(err) {

		f, err := os.Create(dst)
		if err != nil {
			return err
		}

		defer f.Close()
		f.WriteString(" ")

		s, err := os.Open(src)
		if err != nil {
			return err
		}
		defer s.Close()

		srcFileInfo, err := s.Stat()
		if err != nil {
			return err
		}

		f.Chmod(srcFileInfo.Mode())

		sysStat, ok := srcFileInfo.Sys().(*syscall.Stat_t)
		if !ok {
			log.Warnln("sensor: createDummyFile - unable to get Stat_t =>", src)
			return nil
		}

		//note: doing it only for regular files
		if srcFileInfo.Mode()&os.ModeSymlink != 0 {
			log.Warnln("sensor: createDummyFile - source is a symlink =>", src)
			return nil
		}

		//note: need to do the same for symlinks too
		if err := fsutil.UpdateFileTimes(dst, sysStat.Mtim, sysStat.Atim); err != nil {
			log.Warnln("sensor: createDummyFile - UpdateFileTimes error =>", dst)
			return err
		}
	}

	return nil
}
*/

///////////////////////////////////////////////////////////////////////////////

func ArchiveFiles(afname string,
	files []string,
	removePath bool,
	addPrefix string) error {
	tf, err := os.Create(afname)
	if err != nil {
		return err
	}

	defer close(tf)

	tw := tar.NewWriter(tf)
	defer close(tw)

	fpRewrite := func(filePath string,
		removePath bool,
		addPrefix string) string {
		if removePath {
			filePath = filepath.Base(filePath)
		}

		if addPrefix != "" {
			filePath = fmt.Sprintf("%s%s", addPrefix, filePath)
		}

		return filePath
	}

	for _, fname := range files {
		if Exists(fname) && IsRegularFile(fname) {
			info, err := os.Stat(fname)
			if err != nil {
				log.Errorf("fsutil.ArchiveFiles: bad file - %s - %v", fname, err)
				return err
			}

			th, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}

			th.Name = fpRewrite(fname, true, addPrefix)
			if err := tw.WriteHeader(th); err != nil {
				return err
			}

			f, err := os.Open(fname)
			if err != nil {
				return err
			}

			defer close(f)
			if _, err := io.CopyN(tw, f, th.Size); err != nil {
				return fmt.Errorf("cannot write %s file: %w", fname, err)
			}
		} else {
			log.Errorf("fsutil.ArchiveFiles: bad file - %s", fname)
			return fmt.Errorf("bad file - %s", fname)
		}
	}

	return nil
}

func ArchiveDir(afname string,
	d2aname string,
	trimPrefix string,
	addPrefix string) error {
	dirInfo, err := os.Stat(d2aname)
	if err != nil {
		return err
	}

	if !dirInfo.IsDir() {
		return fmt.Errorf("not a directory")
	}

	tf, err := os.Create(afname)
	if err != nil {
		return err
	}

	defer close(tf)

	tw := tar.NewWriter(tf)
	defer close(tw)

	onFSObject := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Errorf("fsutil.ArchiveDir.onFSObject: path=%q err=%q", path, err)
			return err
		}

		if info == nil {
			log.Errorf("fsutil.ArchiveDir.onFSObject: path=%q skipping... no file info", path)
			return nil
		}

		log.Debugf("fsutil.ArchiveDir.onFSObject: path=%q name=%v size=%v mode=%o isDir=%v isReg=%v",
			path, info.Name(), info.Size(), info.Mode(), info.IsDir(), info.Mode().IsRegular())

		switch {
		case info.IsDir():
			th, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}

			th.Name = fmt.Sprintf("%s/", fpUpdate(path, trimPrefix, addPrefix))
			if err := tw.WriteHeader(th); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			th, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}

			th.Name = fpUpdate(path, trimPrefix, addPrefix)
			if err := tw.WriteHeader(th); err != nil {
				return err
			}

			f, err := os.Open(path)
			if err != nil {
				return err
			}

			defer close(f)
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		case info.Mode()&os.ModeSymlink != 0:
			linkRef, err := os.Readlink(path)
			if err != nil {
				return err
			}

			th, err := tar.FileInfoHeader(info, linkRef)
			if err != nil {
				return err
			}

			th.Name = fpUpdate(path, trimPrefix, addPrefix)
			if err := tw.WriteHeader(th); err != nil {
				return err
			}
		default:
			log.Debugf("fsutil.ArchiveDir.onFSObject: ignoring other file object types - %q", path)
			return nil
		}

		return nil
	}

	filepath.Walk(d2aname, onFSObject)

	return nil
}

func close(ref io.Closer) {
	if err := ref.Close(); err != nil {
		log.Errorf("close - error closing: %v", err)
	}
}

func fpUpdate(filePath string,
	trimPrefix string,
	addPrefix string) string {
	if trimPrefix != "" {
		filePath = strings.TrimPrefix(filePath, trimPrefix)
	}

	if addPrefix != "" {
		filePath = fmt.Sprintf("%s%s", addPrefix, filePath)
	}

	return filePath
}

///////////////////////////////////////////////////////////////////////////////

// UpdateFileTimes updates the atime and mtime timestamps on the target file
func UpdateFileTimes(target string, atime, mtime syscall.Timespec) error {
	ts := []syscall.Timespec{atime, mtime}
	return syscall.UtimesNano(target, ts)
}

// UpdateSymlinkTimes updates the atime and mtime timestamps on the target symlink
func UpdateSymlinkTimes(target string, atime, mtime syscall.Timespec) error {
	ts := []unix.Timespec{unix.Timespec(atime), unix.Timespec(mtime)}
	return unix.UtimesNanoAt(unix.AT_FDCWD, target, ts, unix.AT_SYMLINK_NOFOLLOW)
}

// LoadStructFromFile creates a struct from a file
func LoadStructFromFile(filePath string, out interface{}) error {
	if _, err := os.Stat(filePath); err != nil {
		return err
	}

	raw, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	if len(raw) == 0 {
		return ErrNoFileData
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	err = decoder.Decode(out)
	if err != nil {
		return err
	}

	return nil
}
