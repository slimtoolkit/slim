package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
)

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

func filterFileEvents(fileEvents map[event]bool, targetPidList map[int]bool) []string {
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

func findSymlinks(files []string, mp string) map[string]*artifactProps {
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

	result := make(map[string]*artifactProps, 0)
	for _, i := range inodes {
		v := inodeToFiles[i]
		for _, f := range v {
			result[f] = nil
		}
	}
	return result
}

func cpFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		log.Warnln("launcher: monitor - cp - error opening source file =>", src)
		return err
	}
	defer s.Close()

	dstDir := fileDir(dst)
	err = os.MkdirAll(dstDir, 0777)
	if err != nil {
		log.Warnln("launcher: monitor - dir error =>", err)
	}

	d, err := os.Create(dst)
	if err != nil {
		log.Warnln("launcher: monitor - cp - error opening dst file =>", dst)
		return err
	}

	srcFileInfo, err := s.Stat()
	if err == nil {
		d.Chmod(srcFileInfo.Mode())
	}

	if _, err := io.Copy(d, s); err != nil {
		d.Close()
		return err
	}
	return d.Close()
}

func getFileHash(artifactFileName string) (string, error) {
	fileData, err := ioutil.ReadFile(artifactFileName)
	if err != nil {
		return "", err
	}

	hash := sha1.Sum(fileData)
	return hex.EncodeToString(hash[:]), nil
}

func getDataType(artifactFileName string) (string, error) {
	//TODO: use libmagic (pure impl)
	var cerr bytes.Buffer
	var cout bytes.Buffer

	cmd := exec.Command("file", artifactFileName)
	cmd.Stderr = &cerr
	cmd.Stdout = &cout

	if err := cmd.Start(); err != nil {
		return "", err
	}

	if err := cmd.Wait(); err != nil {
		err = fmt.Errorf("Error getting data type: %s / stderr: %s", err, cerr.String())
		return "", err
	}

	if typeInfo := strings.Split(strings.TrimSpace(cout.String()), ":"); len(typeInfo) > 1 {
		return strings.TrimSpace(typeInfo[1]), nil
	}

	return "unknown", nil
}
