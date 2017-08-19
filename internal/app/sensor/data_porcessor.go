package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/docker-slim/docker-slim/internal/app/sensor/monitors/fanotify"
	"github.com/docker-slim/docker-slim/pkg/messages"
	"github.com/docker-slim/docker-slim/pkg/report"

	log "github.com/Sirupsen/logrus"
)

func processReports(mountPoint string,
	fanReport *report.FanMonitorReport,
	ptReport *report.PtMonitorReport,
	peReport *report.PeMonitorReport,
	cmd *messages.StartMonitor) {

	fileCount := 0
	for _, processFileMap := range fanReport.ProcessFiles {
		fileCount += len(processFileMap)
	}
	fileList := make([]string, 0, fileCount)
	for _, processFileMap := range fanReport.ProcessFiles {
		for fpath := range processFileMap {
			fileList = append(fileList, fpath)
		}
	}

	allFilesMap := findSymlinks(fileList, mountPoint)
	saveResults(fanReport, allFilesMap, ptReport, peReport, cmd)
}

func getProcessChildren(pid int, targetPidList map[int]bool, processChildrenMap map[int][]int) {
	if children, ok := processChildrenMap[pid]; ok {
		for _, cpid := range children {
			targetPidList[cpid] = true
			getProcessChildren(cpid, targetPidList, processChildrenMap)
		}
	}
}

func findTargetAppProcesses(rootPidList []int, processChildrenMap map[int][]int) map[int]bool {
	var targetPidList map[int]bool

	for _, pid := range rootPidList {
		targetPidList[pid] = true
		getProcessChildren(pid, targetPidList, processChildrenMap)
	}

	return targetPidList
}

func filterFileEvents(fileEvents map[fanotify.Event]bool, targetPidList map[int]bool) []string {
	var files []string
	for evt := range fileEvents {
		if _, ok := targetPidList[int(evt.Pid)]; ok {
			files = append(files, evt.File)
		}
	}

	return files
}

///////////////////////////////////////////////////////////////////////////////////////

func findSymlinks(files []string, mp string) map[string]*report.ArtifactProps {
	result := make(map[string]*report.ArtifactProps, 0)

	//getting the root device is a leftover from the legacy code (not really necessary anymore)
	devID, err := getFileDevice(mp)
	if err != nil {
		return result
	}

	log.Debugf("findSymlinks - deviceId=%v", devID)

	inodes, devices := filesToInodesNative(files)
	inodeToFiles := make(map[uint64][]string)

	//native filepath.Walk is a bit slow (compared to the "find" command)
	//but it's fast enough for now
	filepath.Walk(mp, func(fullName string, fileInfo os.FileInfo, err error) error {
		sysStatInfo, ok := fileInfo.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("findSymlinks - could not convert fileInfo to Stat_t for %s", fullName)
		}

		if _, ok := devices[uint64(sysStatInfo.Dev)]; !ok {
			return filepath.SkipDir
		}

		if fileInfo.Mode()&os.ModeSymlink != 0 {
			if info, err := getFileSysStats(fullName); err == nil {

				if _, ok := inodes[info.Ino]; ok {
					//not using the inode for the link (using the target inode instead)
					inodeToFiles[info.Ino] = append(inodeToFiles[info.Ino], fullName)
				} else {
					//log.Debugf("findSymlinks - don't care about this symlink (%s)",fullName)
				}

			} else {
				log.Infof("findSymlinks - could not get target stats info for %v", fullName)
			}

		} else {
			if _, ok := inodes[sysStatInfo.Ino]; ok {
				inodeToFiles[sysStatInfo.Ino] = append(inodeToFiles[sysStatInfo.Ino], fullName)
			} else {
				//log.Debugf("findSymlinks - don't care about this file (%s)",fullName)
			}
		}

		return nil
	})

	for inodeID := range inodes {
		v := inodeToFiles[inodeID]
		for _, f := range v {
			//result[f] = inodeID
			result[f] = nil
		}
	}

	return result
}

func filesToInodesNative(files []string) (map[uint64]struct{}, map[uint64]struct{}) {
	inodes := map[uint64]struct{}{}
	devices := map[uint64]struct{}{}

	for _, fullName := range files {
		info, err := getFileSysStats(fullName)
		if err != nil {
			log.Debugf("filesToInodesNative - could not get inode for %s", fullName)
			continue
		}

		inodes[info.Ino] = struct{}{}
		devices[uint64(info.Dev)] = struct{}{}
	}

	return inodes, devices
}

func getFileSysStats(fullName string) (*syscall.Stat_t, error) {
	statInfo, err := os.Stat(fullName)
	if err != nil {
		return nil, err
	}

	sysStatInfo, ok := statInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("failed to get system stat info for %s", fullName)
	}

	return sysStatInfo, nil
}

func getFileDevice(fullName string) (uint64, error) {
	info, err := getFileSysStats(fullName)
	if err != nil {
		return 0, err
	}

	return uint64(info.Dev), nil
}

func getFileInode(fullName string) (uint64, error) {
	info, err := getFileSysStats(fullName)
	if err != nil {
		return 0, err
	}

	log.Debugf("getFileInode(%s) => %v", fullName, info)

	return info.Ino, nil
}

///////////////////////////////////////////////////////////////////////////////////////
//DEPRICATING
///////////////////////////////////////////////////////////////////////////////////////

func filesToInodesLegacy(files []string) []int {
	cmd := "/usr/bin/stat"
	args := []string{"-L", "-c", "%i"}
	args = append(args, files...)
	var inodes []int

	c := exec.Command(cmd, args...)
	out, _ := c.Output()
	c.Wait()
	for _, i := range strings.Split(string(out), "\n") {
		inode, err := strconv.Atoi(strings.TrimSpace(i))
		if err != nil {
			continue
		}
		inodes = append(inodes, inode)
	}
	return inodes
}

func findSymlinksLegacy(files []string, mp string) map[string]*report.ArtifactProps {
	cmd := "/usr/bin/find"
	args := []string{"-L", mp, "-mount", "-printf", "%i %p\n"}
	c := exec.Command(cmd, args...)
	out, _ := c.Output()
	c.Wait()

	inodes := filesToInodesLegacy(files)
	inodeToFiles := make(map[int][]string)

	for _, v := range strings.Split(string(out), "\n") {
		v = strings.TrimSpace(v)
		info := strings.Split(v, " ")
		inode, err := strconv.Atoi(info[0])
		if err != nil {
			continue
		}
		inodeToFiles[inode] = append(inodeToFiles[inode], info[1])
	}

	result := make(map[string]*report.ArtifactProps, 0)
	for _, i := range inodes {
		v := inodeToFiles[i]
		for _, f := range v {
			result[f] = nil
		}
	}
	return result
}
