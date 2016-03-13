package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cloudimmunity/docker-slim/messages"
	"github.com/cloudimmunity/docker-slim/report"
	"github.com/cloudimmunity/docker-slim/utils"

	log "github.com/Sirupsen/logrus"
)

const (
	pycExt                 = ".pyc"
	pyoExt                 = ".pyo"
	pycacheDir             = "/__pycache__/"
	pycache                = "__pycache__"
	defaultReportName      = "creport.json"
	defaultArtifactDirName = "/opt/dockerslim/artifacts"
)

func saveResults(fanMonReport *report.FanMonitorReport,
	fileNames map[string]*report.ArtifactProps,
	ptMonReport *report.PtMonitorReport,
	peReport *report.PeMonitorReport,
	cmd *messages.StartMonitor) {
	artifactDirName := defaultArtifactDirName

	artifactStore := newArtifactStore(artifactDirName, fanMonReport, fileNames, ptMonReport, peReport, cmd)
	artifactStore.prepareArtifacts()
	artifactStore.saveArtifacts()
	artifactStore.saveReport()
}

type artifactStore struct {
	storeLocation string
	fanMonReport  *report.FanMonitorReport
	ptMonReport   *report.PtMonitorReport
	peMonReport   *report.PeMonitorReport
	rawNames      map[string]*report.ArtifactProps
	nameList      []string
	resolve       map[string]struct{}
	linkMap       map[string]*report.ArtifactProps
	fileMap       map[string]*report.ArtifactProps
	cmd           *messages.StartMonitor
}

func newArtifactStore(storeLocation string,
	fanMonReport *report.FanMonitorReport,
	rawNames map[string]*report.ArtifactProps,
	ptMonReport *report.PtMonitorReport,
	peMonReport *report.PeMonitorReport,
	cmd *messages.StartMonitor) *artifactStore {
	store := &artifactStore{
		storeLocation: storeLocation,
		fanMonReport:  fanMonReport,
		ptMonReport:   ptMonReport,
		peMonReport:   peMonReport,
		rawNames:      rawNames,
		nameList:      make([]string, 0, len(rawNames)),
		resolve:       map[string]struct{}{},
		linkMap:       map[string]*report.ArtifactProps{},
		fileMap:       map[string]*report.ArtifactProps{},
		cmd:           cmd,
	}

	return store
}

func (p *artifactStore) getArtifactFlags(artifactFileName string) map[string]bool {
	flags := map[string]bool{}
	for _, processFileMap := range p.fanMonReport.ProcessFiles {
		if finfo, ok := processFileMap[artifactFileName]; ok {
			if finfo.ReadCount > 0 {
				flags["R"] = true
			}

			if finfo.WriteCount > 0 {
				flags["W"] = true
			}

			if finfo.ExeCount > 0 {
				flags["X"] = true
			}
		}
	}

	if len(flags) < 1 {
		return nil
	}

	return flags
}

func (p *artifactStore) prepareArtifact(artifactFileName string) {
	srcLinkFileInfo, err := os.Lstat(artifactFileName)
	if err != nil {
		log.Warnf("prepareArtifact - artifact don't exist: %v (%v)\n", artifactFileName, os.IsNotExist(err))
		return
	}

	p.nameList = append(p.nameList, artifactFileName)

	props := &report.ArtifactProps{
		FilePath: artifactFileName,
		Mode:     srcLinkFileInfo.Mode(),
		ModeText: srcLinkFileInfo.Mode().String(),
		FileSize: srcLinkFileInfo.Size(),
	}

	props.Flags = p.getArtifactFlags(artifactFileName)

	log.Debugf("prepareArtifact - file mode:%v\n", srcLinkFileInfo.Mode())
	switch {
	case srcLinkFileInfo.Mode().IsRegular():
		props.FileType = report.FileArtifactType
		props.Sha1Hash, _ = getFileHash(artifactFileName)
		props.DataType, _ = getDataType(artifactFileName)
		p.fileMap[artifactFileName] = props
		p.rawNames[artifactFileName] = props
	case (srcLinkFileInfo.Mode() & os.ModeSymlink) != 0:
		linkRef, err := os.Readlink(artifactFileName)
		if err != nil {
			log.Warnf("prepareArtifact - error getting reference for symlink: %v\n", artifactFileName)
			return
		}

		props.FileType = report.SymlinkArtifactType
		props.LinkRef = linkRef

		if _, ok := p.rawNames[linkRef]; !ok {
			p.resolve[linkRef] = struct{}{}
		}

		p.linkMap[artifactFileName] = props
		p.rawNames[artifactFileName] = props

	case srcLinkFileInfo.Mode().IsDir():
		log.Warnf("prepareArtifact - is a directory (shouldn't see it)")
		props.FileType = report.DirArtifactType
	default:
		log.Warn("prepareArtifact - other type (shouldn't see it)")
	}
}

func (p *artifactStore) prepareArtifacts() {
	for artifactFileName := range p.rawNames {
		log.Debugf("prepareArtifacts - artifact => %v\n", artifactFileName)
		p.prepareArtifact(artifactFileName)
	}

	p.resolveLinks()
}

func (p *artifactStore) resolveLinks() {
	for name := range p.resolve {
		_ = name
		log.Debug("resolveLinks - resolving: ", name)
		//TODO
	}
}

func (p *artifactStore) saveArtifacts() {
	var excludePaths map[string]bool
	var includePaths map[string]bool

	preparePaths := func(pathList []string) map[string]bool {
		if len(pathList) < 1 {
			return nil
		}

		paths := map[string]bool{}
		for _, pathValue := range pathList {
			pathInfo, err := os.Stat(pathValue)
			if err != nil {
				log.Debug("saveArtifacts.preparePaths(): skipping path = ", pathValue)
				continue
			}

			if pathInfo.IsDir() {
				paths[pathValue] = true
			} else {
				paths[pathValue] = false
			}
		}

		return paths
	}

	includePaths = preparePaths(p.cmd.Includes)
	excludePaths = preparePaths(p.cmd.Excludes)
	log.Debugf("includePaths: %+v\n", includePaths)
	log.Debugf("excludePaths: %+v\n", excludePaths)

	//TODO: use exludePaths to filter discovered files
	for fileName := range p.fileMap {
		filePath := fmt.Sprintf("%s/files%s", p.storeLocation, fileName)
		log.Debug("saveArtifacts - saving file data => ", filePath)
		err := cpFile(fileName, filePath)
		if err != nil {
			log.Warn("saveArtifacts - error saving file => ", err)
		}
	}

	//TODO: use exludePaths to filter discovered links
	for linkName, linkProps := range p.linkMap {
		linkPath := fmt.Sprintf("%s/files%s", p.storeLocation, linkName)
		linkDir := utils.FileDir(linkPath)
		err := os.MkdirAll(linkDir, 0777)
		if err != nil {
			log.Warn("saveArtifacts - dir error => ", err)
			continue
		}
		err = os.Symlink(linkProps.LinkRef, linkPath)
		if err != nil {
			log.Warn("saveArtifacts - symlink create error ==> ", err)
		}
	}

	for fileName := range p.fileMap {
		filePath := fmt.Sprintf("%s/files%s", p.storeLocation, fileName)

		err := fixPy3CacheFile(fileName, filePath)
		if err != nil {
			log.Warn("saveArtifacts - error fixing py3 cache file => ", err)
		}
	}

	//TODO: use exludePaths to filter included paths
	for inPath, isDir := range includePaths {
		dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, inPath)
		if isDir {
			err, errs := utils.CopyDir(inPath, dstPath, true, true, nil, nil, nil)
			if err != nil {
				log.Warnf("CopyDir(%v,%v) error: %v\n", inPath, dstPath, err)
			}

			if len(errs) > 0 {
				log.Warnf("CopyDir(%v,%v) copy errors: %+v\n", inPath, dstPath, errs)
			}
		} else {
			if err := utils.CopyFile(inPath, dstPath, true); err != nil {
				log.Warnf("CopyFile(%v,%v) error: %v\n", inPath, dstPath, err)
			}
		}
	}
}

func (p *artifactStore) saveReport() {
	sort.Strings(p.nameList)

	creport := report.ContainerReport{
		Monitors: report.MonitorReports{
			Pt:  p.ptMonReport,
			Fan: p.fanMonReport,
		},
	}

	for _, fname := range p.nameList {
		creport.Image.Files = append(creport.Image.Files, p.rawNames[fname])
	}

	artifactDirName := defaultArtifactDirName
	reportName := defaultReportName

	_, err := os.Stat(artifactDirName)
	if os.IsNotExist(err) {
		os.MkdirAll(artifactDirName, 0777)
		_, err = os.Stat(artifactDirName)
		utils.FailOn(err)
	}

	reportFilePath := filepath.Join(artifactDirName, reportName)
	log.Debug("sensor: monitor - saving report to ", reportFilePath)

	reportData, err := json.MarshalIndent(creport, "", "  ")
	utils.FailOn(err)

	err = ioutil.WriteFile(reportFilePath, reportData, 0644)
	utils.FailOn(err)
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

func cpFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		log.Warnln("sensor: monitor - cp - error opening source file =>", src)
		return err
	}
	defer s.Close()

	dstDir := utils.FileDir(dst)
	err = os.MkdirAll(dstDir, 0777)
	if err != nil {
		log.Warnln("sensor: monitor - dir error =>", err)
	}

	d, err := os.Create(dst)
	if err != nil {
		log.Warnln("sensor: monitor - cp - error opening dst file =>", dst)
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

func py3FileNameFromCache(p string) string {
	ext := path.Ext(p)

	if !(((ext == pycExt) || (ext == pycExt)) && strings.Contains(p, pycacheDir)) {
		return ""
	}

	pathParts := strings.Split(p, "/")

	if !((len(pathParts) > 1) && (pycache == pathParts[len(pathParts)-2])) {
		return ""
	}

	pycFileName := path.Base(p)

	nameParts := strings.Split(pycFileName, ".")
	if !(len(nameParts) > 2) {
		return ""
	}

	var pyFileName string
	if len(nameParts) == 3 {
		pyFileName = fmt.Sprintf("%v.py", nameParts[0])
	} else {
		pyFileName = fmt.Sprintf("%v.py", strings.Join(nameParts[0:len(nameParts)-2], "."))
	}

	return path.Join(path.Dir(path.Dir(p)), pyFileName)
}

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
	}

	return nil
}

func fixPy3CacheFile(src, dst string) error {
	dstPyFilePath := py3FileNameFromCache(dst)
	if dstPyFilePath == "" {
		return nil
	}

	srcPyFilePath := py3FileNameFromCache(src)
	if srcPyFilePath == "" {
		return nil
	}

	if _, err := os.Stat(dstPyFilePath); err != nil && os.IsNotExist(err) {
		if err := cpFile(srcPyFilePath, dstPyFilePath); err != nil {
			log.Warnln("sensor: monitor - fixPy3CacheFile - error copying file =>", dstPyFilePath)
			return err
		}
	}

	return nil
}
