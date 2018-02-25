package fsutils

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/docker-slim/docker-slim/pkg/utils/errutils"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/pdiscover"
)

// Directory and file related errors
var (
	ErrNoSrcDir                  = errors.New("no source directory path")
	ErrNoDstDir                  = errors.New("no destination directory path")
	ErrSameDir                   = errors.New("source and destination directories are the same")
	ErrSrcDirNotExist            = errors.New("source directory doesn't exist")
	ErrSrcNotDir                 = errors.New("source is not a directory")
	ErrSrcNotRegularFile         = errors.New("source is not a regular file")
	ErrUnsupportedFileObjectType = errors.New("unsupported file object type")
)

const (
	stateBaseKey        = ".images"
	stateArtifactsKey   = "artifacts"
	stateArtifactsPerms = 0777
)

// Remove removes the artifacts generated during the current application execution
func Remove(artifactLocation string) error {
	return os.RemoveAll(artifactLocation)
}

// Exists returns true if the target file system object exists
func Exists(target string) bool {
	if _, err := os.Stat(target); err != nil {
		return false
	}

	return true
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

// CopyFile copies the source file system object to the desired destination
func CopyFile(src, dst string, makeDir bool) error {
	log.Debugf("CopyFile(%v,%v,%v)", src, dst, makeDir)

	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	switch {
	case info.Mode().IsRegular():
		return CopyRegularFile(src, dst, makeDir)
	case (info.Mode() & os.ModeSymlink) == os.ModeSymlink:
		return CopySymlinkFile(src, dst, makeDir)
	default:
		return ErrUnsupportedFileObjectType
	}
}

// CopySymlinkFile copies a symlink file
func CopySymlinkFile(src, dst string, makeDir bool) error {
	log.Debugf("CopySymlinkFile(%v,%v,%v)", src, dst, makeDir)

	linkRef, err := os.Readlink(src)
	if err != nil {
		return err
	}

	err = os.Symlink(linkRef, dst)
	if err != nil {
		return err
	}

	return nil
}

// CopyRegularFile copies a regular file
func CopyRegularFile(src, dst string, makeDir bool) error {
	log.Debugf("CopyRegularFile(%v,%v,%v)", src, dst, makeDir)

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
		srcDirName, err := filepath.Abs(filepath.Dir(src))
		if err != nil {
			return err
		}

		dstDirName, err := filepath.Abs(filepath.Dir(dst))
		if err != nil {
			return err
		}

		if _, err := os.Stat(dstDirName); err != nil {

			if os.IsNotExist(err) {

				srcDirInfo, err := os.Stat(srcDirName)
				if err != nil {
					return err
				}

				err = os.MkdirAll(dstDirName, srcDirInfo.Mode())
				if err != nil {
					return err
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

	d.Chmod(srcFileInfo.Mode())
	return d.Close()
}

func copyFileObjectHandler(
	srcBase, dstBase string,
	copyRelPath, skipErrors bool,
	ignorePaths, ignoreDirNames, ignoreFileNames map[string]struct{},
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

		var targetPath string
		if copyRelPath {
			targetPath = filepath.Join(dstBase, strings.TrimPrefix(path, srcBase))
		} else {
			targetPath = filepath.Join(dstBase, path)
		}

		foBase := filepath.Base(path)

		switch {
		case info.Mode().IsDir():
			if _, ok := ignorePaths[path]; ok {
				log.Debug("dir path in ignorePath list (skipping dir)...")
				return filepath.SkipDir
			}

			if _, ok := ignoreDirNames[foBase]; ok {
				log.Debug("dir name in ignoreDirNames list (skipping dir)...")
				return filepath.SkipDir
			}

			err = os.MkdirAll(targetPath, info.Mode())
			if err != nil {
				if skipErrors {
					*errs = append(*errs, err)
					return nil
				}

				return err
			}
		case info.Mode().IsRegular():
			if _, ok := ignorePaths[path]; ok {
				log.Debug("file path in ignorePath list (skipping file)...")
				return nil
			}

			if _, ok := ignoreFileNames[foBase]; ok {
				log.Debug("file name in ignoreDirNames list (skipping file)...")
				return nil
			}

			err = CopyRegularFile(path, targetPath, true)
			if err != nil {
				if skipErrors {
					*errs = append(*errs, err)
					return nil
				}

				return err
			}
		case (info.Mode() & os.ModeSymlink) == os.ModeSymlink:
			if _, ok := ignorePaths[path]; ok {
				log.Debug("link file path in ignorePath list (skipping file)...")
				return nil
			}

			if _, ok := ignoreFileNames[foBase]; ok {
				log.Debug("link file name in ignoreDirNames list (skipping file)...")
				return nil
			}

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

// CopyDir copies a directory
func CopyDir(src, dst string,
	copyRelPath, skipErrors bool,
	ignorePaths, ignoreDirNames, ignoreFileNames map[string]struct{}) (error, []error) {
	log.Debugf("CopyDir(%v,%v,%v,%v,...)", src, dst, copyRelPath, skipErrors)

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

	var errs []error
	err = filepath.Walk(src, copyFileObjectHandler(
		src, dst, copyRelPath, skipErrors, ignorePaths, ignoreDirNames, ignoreFileNames, &errs))
	if err != nil {
		return err, nil
	}

	return nil, errs
}

///////////////////////////////////////////////////////////////////////////////

// ExeDir returns the directory information for the application
func ExeDir() string {
	exePath, err := pdiscover.GetOwnProcPath()
	errutils.FailOn(err)
	return filepath.Dir(exePath)
}

// FileDir returns the directory information for the given file
func FileDir(fileName string) string {
	dirName, err := filepath.Abs(filepath.Dir(fileName))
	errutils.FailOn(err)
	return dirName
}

// PrepareStateDirs ensures that the required application directories exist
func PrepareStateDirs(statePrefix, imageID string) (string, string) {
	log.Debugf("PrepareStateDirs(%v,%v)", statePrefix, imageID)

	//images IDs in Docker 1.9+ are prefixed with a hash type...
	if strings.Contains(imageID, ":") {
		parts := strings.Split(imageID, ":")
		imageID = parts[1]
	}

	if statePrefix == "" {
		statePrefix = ExeDir()
	}

	localVolumePath := filepath.Join(statePrefix, stateBaseKey, imageID)
	artifactLocation := filepath.Join(localVolumePath, stateArtifactsKey)
	artifactDir, err := os.Stat(artifactLocation)
	if err == nil {
		log.Debugf("PrepareStateDirs - removing existing state location: %v", artifactLocation)
		err = Remove(artifactLocation)
		if err != nil {
			log.Debugf("PrepareStateDirs - failed to remove existing state location: %v", artifactLocation)
			errutils.FailOn(err)
		}
	} else if os.IsNotExist(err) {
		log.Debugf("PrepareStateDirs - will create new state location: %v", artifactLocation)
	} else {
		errutils.FailOn(err)
	}

	err = os.MkdirAll(artifactLocation, stateArtifactsPerms)
	errutils.FailOn(err)
	artifactDir, err = os.Stat(artifactLocation)
	errutils.FailOn(err)
	log.Debug("PrepareStateDirs - created new state location: ", artifactLocation)

	errutils.FailWhen(!artifactDir.IsDir(), "artifact location is not a directory")

	return localVolumePath, artifactLocation
}

///////////////////////////////////////////////////////////////////////////////

// UpdateFileTimes updates the atime and mtime timestamps on the target file
func UpdateFileTimes(target string, atime, mtime syscall.Timespec) error {
	ts := []syscall.Timespec{atime, mtime}
	return syscall.UtimesNano(target, ts)
}

//todo: UpdateSymlinkTimes()
