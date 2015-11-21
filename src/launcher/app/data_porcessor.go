package main

import (
	"os/exec"
	"strconv"
	"strings"

	"internal/report"
	"launcher/monitors/fanotify"
)

func processReports(mountPoint string,
	fanReport *report.FanMonitorReport,
	ptReport *report.PtMonitorReport,
	peReport *report.PeMonitorReport) {

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
	saveResults(fanReport, allFilesMap, ptReport, peReport)
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

func filesToInodes(files []string) []int {
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

func findSymlinks(files []string, mp string) map[string]*report.ArtifactProps {
	cmd := "/usr/bin/find"
	args := []string{"-L", mp, "-mount", "-printf", "%i %p\n"}
	c := exec.Command(cmd, args...)
	out, _ := c.Output()
	c.Wait()

	inodes := filesToInodes(files)
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
