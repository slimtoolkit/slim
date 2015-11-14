package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	log "github.com/Sirupsen/logrus"
)

type processInfo struct {
	Pid       int32  `json:"pid"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Cmd       string `json:"cmd"`
	Cwd       string `json:"cwd"`
	Root      string `json:"root"`
	ParentPid int32  `json:"ppid"`
}

type fileInfo struct {
	EventCount   uint32 `json:"event_count"`
	FirstEventID uint32 `json:"first_eid"`
	Name         string `json:"-"`
	ReadCount    uint32 `json:"reads,omitempty"`
	WriteCount   uint32 `json:"writes,omitempty"`
	ExeCount     uint32 `json:"execs,omitempty"`
}

type artifactType int

const (
	dirArtifactType     = 1
	fileArtifactType    = 2
	symlinkArtifactType = 3
	unknownArtifactType = 99
)

var artifactTypeNames = map[artifactType]string{
	dirArtifactType:     "Dir",
	fileArtifactType:    "File",
	symlinkArtifactType: "Symlink",
	unknownArtifactType: "Unknown",
}

func (t artifactType) String() string {
	return artifactTypeNames[t]
}

type artifactProps struct {
	FileType artifactType    `json:"-"`
	FilePath string          `json:"file_path"`
	Mode     os.FileMode     `json:"-"`
	ModeText string          `json:"mode"`
	LinkRef  string          `json:"link_ref,omitempty"`
	Flags    map[string]bool `json:"flags,omitempty"`
	DataType string          `json:"data_type,omitempty"`
	FileSize int64           `json:"file_size"`
	Sha1Hash string          `json:"sha1_hash,omitempty"`
	AppType  string          `json:"app_type,omitempty"`
}

type artifactStore struct {
	storeLocation string
	fanMonReport  *fanMonitorReport
	ptMonReport   *ptMonitorReport
	rawNames      map[string]*artifactProps
	nameList      []string
	resolve       map[string]struct{}
	linkMap       map[string]*artifactProps
	fileMap       map[string]*artifactProps
}

type imageReport struct {
	Files []*artifactProps `json:"files"`
}

type monitorReports struct {
	Fan *fanMonitorReport `json:"fan"`
	Pt  *ptMonitorReport  `json:"pt"`
}

type containerReport struct {
	Monitors monitorReports `json:"monitors"`
	Image    imageReport    `json:"image"`
}

func (p *artifactProps) MarshalJSON() ([]byte, error) {
	type artifactPropsType artifactProps
	return json.Marshal(&struct {
		FileTypeStr string `json:"file_type"`
		*artifactPropsType
	}{
		FileTypeStr:       p.FileType.String(),
		artifactPropsType: (*artifactPropsType)(p),
	})
}

func newArtifactStore(storeLocation string,
	fanMonReport *fanMonitorReport,
	rawNames map[string]*artifactProps,
	ptMonReport *ptMonitorReport) *artifactStore {
	store := &artifactStore{
		storeLocation: storeLocation,
		fanMonReport:  fanMonReport,
		ptMonReport:   ptMonReport,
		rawNames:      rawNames,
		nameList:      make([]string, 0, len(rawNames)),
		resolve:       map[string]struct{}{},
		linkMap:       map[string]*artifactProps{},
		fileMap:       map[string]*artifactProps{},
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

	props := &artifactProps{
		FilePath: artifactFileName,
		Mode:     srcLinkFileInfo.Mode(),
		ModeText: srcLinkFileInfo.Mode().String(),
		FileSize: srcLinkFileInfo.Size(),
	}

	props.Flags = p.getArtifactFlags(artifactFileName)

	log.Debugf("prepareArtifact - file mode:%v\n", srcLinkFileInfo.Mode())
	switch {
	case srcLinkFileInfo.Mode().IsRegular():
		props.FileType = fileArtifactType
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

		props.FileType = symlinkArtifactType
		props.LinkRef = linkRef

		if _, ok := p.rawNames[linkRef]; !ok {
			p.resolve[linkRef] = struct{}{}
		}

		p.linkMap[artifactFileName] = props
		p.rawNames[artifactFileName] = props

	case srcLinkFileInfo.Mode().IsDir():
		log.Warnf("prepareArtifact - is a directory (shouldn't see it)")
		props.FileType = dirArtifactType
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
		log.Debugln("resolveLinks - resolving:", name)
		//TODO
	}
}

func (p *artifactStore) saveArtifacts() {
	for fileName := range p.fileMap {
		filePath := fmt.Sprintf("%s/files%s", p.storeLocation, fileName)
		log.Debugln("saveArtifacts - saving file data =>", filePath)
		err := cpFile(fileName, filePath)
		if err != nil {
			log.Warnln("saveArtifacts - error saving file =>", err)
		}
	}

	for linkName, linkProps := range p.linkMap {
		linkPath := fmt.Sprintf("%s/files%s", p.storeLocation, linkName)
		linkDir := fileDir(linkPath)
		err := os.MkdirAll(linkDir, 0777)
		if err != nil {
			log.Warnln("saveArtifacts - dir error =>", err)
			continue
		}
		err = os.Symlink(linkProps.LinkRef, linkPath)
		if err != nil {
			log.Warnln("saveArtifacts - symlink create error ==>", err)
		}
	}
}

func (p *artifactStore) saveReport() {
	sort.Strings(p.nameList)

	report := containerReport{
		Monitors: monitorReports{
			Pt:  p.ptMonReport,
			Fan: p.fanMonReport,
		},
	}

	for _, fname := range p.nameList {
		report.Image.Files = append(report.Image.Files, p.rawNames[fname])
	}

	artifactDirName := "/opt/dockerslim/artifacts"
	reportName := "creport.json"

	_, err := os.Stat(artifactDirName)
	if os.IsNotExist(err) {
		os.MkdirAll(artifactDirName, 0777)
		_, err = os.Stat(artifactDirName)
		failOnError(err)
	}

	reportFilePath := filepath.Join(artifactDirName, reportName)
	log.Debugln("launcher: monitor - saving report to", reportFilePath)

	reportData, err := json.MarshalIndent(report, "", "  ")
	failOnError(err)

	err = ioutil.WriteFile(reportFilePath, reportData, 0644)
	failOnError(err)
}

func saveResults(fanMonReport *fanMonitorReport, fileNames map[string]*artifactProps, ptMonReport *ptMonitorReport) {
	artifactDirName := "/opt/dockerslim/artifacts"

	artifactStore := newArtifactStore(artifactDirName, fanMonReport, fileNames, ptMonReport)
	artifactStore.prepareArtifacts()
	artifactStore.saveArtifacts()
	artifactStore.saveReport()
}
