// +build linux

package app

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"syscall"

	"github.com/docker-slim/docker-slim/pkg/app/sensor/inspectors/sodeps"
	"github.com/docker-slim/docker-slim/pkg/certdiscover"
	"github.com/docker-slim/docker-slim/pkg/ipc/command"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/system"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"

	"github.com/bmatcuk/doublestar/v3"
	log "github.com/sirupsen/logrus"
)

const (
	pidFileSuffix          = ".pid"
	varRunDir              = "/var/run/"
	ngxBinName             = "/nginx"
	ngxSubDir              = "/nginx/"
	ngxCommonTemp          = "/var/lib/nginx"
	ngxLogTemp             = "/var/log/nginx"
	ngxCacheTemp           = "/var/cache/nginx"
	rbGemSpecExt           = ".gemspec"
	rbGemsSubDir           = "/gems/"
	rbDefaultSpecSubDir    = "/specifications/default/"
	rbSpecSubDir           = "/specifications/"
	rgExtSibDir            = "extensions"
	rbGemBuildFlag         = "gem.build_complete"
	pycExt                 = ".pyc"
	pyoExt                 = ".pyo"
	pycacheDir             = "/__pycache__/"
	pycache                = "__pycache__"
	nodePackageFile        = "package.json"
	nodeNPMNodeGypPackage  = "/npm/node_modules/node-gyp/package.json"
	nodeNPMNodeGypFile     = "bin/node-gyp.js"
	fileTypeCmdName        = "file"
	defaultReportName      = "creport.json"
	defaultArtifactDirName = "/opt/dockerslim/artifacts"
	filesDirName           = "files"
	filesArchiveName       = "files.tar"
	preservedDirName       = "preserved"
)

var fileTypeCmd string

func init() {
	findFileTypeCmd()
}

func findFileTypeCmd() {
	fileTypeCmd, err := exec.LookPath(fileTypeCmdName)
	if err != nil {
		log.Debugf("findFileTypeCmd - cmd not found: %v", err)
		return
	}

	log.Debugf("findFileTypeCmd - cmd found: %v", fileTypeCmd)
}

func prepareEnv(storeLocation string, cmd *command.StartMonitor) {
	log.Debug("sensor.app.prepareEnv()")

	dstRootPath := filepath.Join(storeLocation, filesDirName)
	log.Debugf("sensor.app.prepareEnv - prep file artifacts root dir - '%s'", dstRootPath)
	err := os.MkdirAll(dstRootPath, 0777)
	errutil.FailOn(err)

	if cmd != nil && len(cmd.Preserves) > 0 {
		log.Debugf("sensor.app.prepareEnv(): preserving paths - %d", len(cmd.Preserves))

		preservedDirPath := filepath.Join(storeLocation, preservedDirName)
		log.Debugf("sensor.app.prepareEnv - prep preserved artifacts root dir - '%s'", preservedDirPath)
		err = os.MkdirAll(preservedDirPath, 0777)
		errutil.FailOn(err)

		preservePaths := preparePaths(getKeys(cmd.Preserves))
		log.Debugf("sensor.app.prepareEnv - preservePaths(%v): %+v", len(preservePaths), preservePaths)

		newPerms := getRecordsWithPerms(cmd.Preserves)
		log.Debugf("sensor.app.prepareEnv - newPerms(%v): %+v", len(newPerms), newPerms)

		for inPath, isDir := range preservePaths {
			dstPath := fmt.Sprintf("%s%s", preservedDirPath, inPath)
			log.Debugf("sensor.app.prepareEnv(): [isDir=%v] %s", isDir, dstPath)

			if isDir {
				err, errs := fsutil.CopyDir(cmd.KeepPerms, inPath, dstPath, true, true, nil, nil, nil)
				if err != nil {
					log.Warnf("sensor.app.prepareEnv.CopyDir(%v,%v) error: %v", inPath, dstPath, err)
				}

				if len(errs) > 0 {
					log.Warnf("sensor.app.prepareEnv.CopyDir(%v,%v) copy errors: %+v", inPath, dstPath, errs)
				}
			} else {
				if err := fsutil.CopyFile(cmd.KeepPerms, inPath, dstPath, true); err != nil {
					log.Warnf("sensor.app.prepareEnv.CopyFile(%v,%v) error: %v", inPath, dstPath, err)
				}
			}
		}

		for inPath, perms := range newPerms {
			dstPath := fmt.Sprintf("%s%s", preservedDirPath, inPath)
			if fsutil.Exists(dstPath) {
				if err := fsutil.SetAccess(dstPath, perms); err != nil {
					log.Warnf("sensor.app.prepareEnv.SetPerms(%v,%v) error: %v", dstPath, perms, err)
				}
			}
		}
	}
}

func saveResults(fanMonReport *report.FanMonitorReport,
	fileNames map[string]*report.ArtifactProps,
	ptMonReport *report.PtMonitorReport,
	peReport *report.PeMonitorReport,
	cmd *command.StartMonitor) {
	log.Debugf("saveResults(%v,...)", len(fileNames))

	artifactDirName := defaultArtifactDirName

	artifactStore := newArtifactStore(artifactDirName, fanMonReport, fileNames, ptMonReport, peReport, cmd)
	artifactStore.prepareArtifacts()
	artifactStore.saveArtifacts()
	//artifactStore.archiveArtifacts() //alternative way to xfer artifacts
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
	cmd           *command.StartMonitor
}

func newArtifactStore(storeLocation string,
	fanMonReport *report.FanMonitorReport,
	rawNames map[string]*report.ArtifactProps,
	ptMonReport *report.PtMonitorReport,
	peMonReport *report.PeMonitorReport,
	cmd *command.StartMonitor) *artifactStore {
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
		log.Warnf("prepareArtifact - artifact don't exist: %v (%v)", artifactFileName, os.IsNotExist(err))
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

	log.Debugf("prepareArtifact - file mode:%v", srcLinkFileInfo.Mode())
	switch {
	case srcLinkFileInfo.Mode().IsRegular():
		props.FileType = report.FileArtifactType
		props.Sha1Hash, _ = getFileHash(artifactFileName)

		if fileTypeCmd != "" {
			props.DataType, _ = getDataType(artifactFileName)
		}

		p.fileMap[artifactFileName] = props
		p.rawNames[artifactFileName] = props
	case (srcLinkFileInfo.Mode() & os.ModeSymlink) != 0:
		linkRef, err := os.Readlink(artifactFileName)
		if err != nil {
			log.Warnf("prepareArtifact - error getting reference for symlink (%v) -> %v", err, artifactFileName)
			return
		}

		props.FileType = report.SymlinkArtifactType
		props.LinkRef = linkRef
		//props.LinkRefAbs, err := filepath.Abs(linkRef)
		//if err != nil {
		//	log.Warnf("prepareArtifact - error getting absolute path for symlink reference (%v) -> %v => %v",
		//		err, artifactFileName, linkRef)
		//}

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
	log.Debugf("p.prepareArtifacts() p.rawNames=%v", len(p.rawNames))

	for artifactFileName := range p.rawNames {
		log.Debugf("prepareArtifacts - artifact => %v", artifactFileName)
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

func preparePaths(pathList []string) map[string]bool {
	if len(pathList) < 1 {
		return nil
	}

	paths := map[string]bool{}
	for _, pathValue := range pathList {
		pathInfo, err := os.Stat(pathValue)
		if err != nil {
			log.Debug("preparePaths(): skipping path = ", pathValue)
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

func getKeys(m map[string]*fsutil.AccessInfo) []string {
	if len(m) == 0 {
		return nil
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	return keys
}

func getRecordsWithPerms(m map[string]*fsutil.AccessInfo) map[string]*fsutil.AccessInfo {
	perms := map[string]*fsutil.AccessInfo{}
	for k, v := range m {
		if v != nil {
			perms[k] = v
		}
	}

	return perms
}

func (p *artifactStore) saveCertsData() {
	copyCertBundles := func(list []string) {
		for _, fname := range list {
			if fsutil.Exists(fname) {
				dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, fname)
				if err := fsutil.CopyFile(p.cmd.KeepPerms, fname, dstPath, true); err != nil {
					log.Warnf("cert file bundle: fsutil.CopyFile(%v,%v) error - %v", fname, dstPath, err)
				}
			}
		}
	}

	copyDirs := func(list []string) {
		for _, fname := range list {
			if fsutil.Exists(fname) {
				dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, fname)

				if fsutil.IsDir(fname) {
					err, errs := fsutil.CopyDir(p.cmd.KeepPerms, fname, dstPath, true, true, nil, nil, nil)
					if err != nil {
						log.Warnf("cert dir: fsutil.CopyDir(%v,%v) error: %v", fname, dstPath, err)
					}

					if len(errs) > 0 {
						log.Warnf("cert dir: fsutil.CopyDir(%v,%v) copy errors: %+v", fname, dstPath, errs)
					}
				} else if fsutil.IsSymlink(fname) {
					if err := fsutil.CopySymlinkFile(p.cmd.KeepPerms, fname, dstPath, true); err != nil {
						log.Warnf("cert dir link: fsutil.CopySymlinkFile(%v,%v) error - %v", fname, dstPath, err)
					}
				}
			}
		}
	}

	if p.cmd.IncludeCertAll {
		copyCertBundles(certdiscover.CertFileList())
		copyCertBundles(certdiscover.CACertFileList())
	}

	if !p.cmd.IncludeCertAll && p.cmd.IncludeCertBundles {
		copyCertBundles(certdiscover.CertFileList())
		copyCertBundles(certdiscover.CACertFileList())
	}

	if p.cmd.IncludeCertDirs {
		copyDirs(certdiscover.CertDirList())
		copyDirs(certdiscover.CACertDirList())
	}

	if p.cmd.IncludeCertPKAll {
	}

	if p.cmd.IncludeCertPKDirs {
	}
}

func (p *artifactStore) saveArtifacts() {
	var includePaths map[string]bool
	var newPerms map[string]*fsutil.AccessInfo

	syscall.Umask(0)

	excludePatterns := p.cmd.Excludes
	excludePatterns = append(excludePatterns, "/opt/dockerslim")
	excludePatterns = append(excludePatterns, "/opt/dockerslim/**")
	log.Debugf("saveArtifacts - excludePatterns(%v): %+v", len(excludePatterns), excludePatterns)

	includePaths = preparePaths(getKeys(p.cmd.Includes))
	log.Debugf("saveArtifacts - includePaths(%v): %+v", len(includePaths), includePaths)

	newPerms = getRecordsWithPerms(p.cmd.Includes)
	log.Debugf("saveArtifacts - newPerms(%v): %+v", len(newPerms), newPerms)

	for pk, pv := range p.cmd.Perms {
		newPerms[pk] = pv
	}
	log.Debugf("saveArtifacts - merged newPerms(%v): %+v", len(newPerms), newPerms)

	//moved to prepareEnv
	//dstRootPath := filepath.Join(p.storeLocation, filesDirName)
	//log.Debugf("saveArtifacts - prep file artifacts root dir - '%s'", dstRootPath)
	//err := os.MkdirAll(dstRootPath, 0777)
	//errutil.FailOn(err)

	extraDirs := map[string]struct{}{}

	log.Debugf("saveArtifacts - copy files (%v)", len(p.fileMap))
copyFiles:
	for srcFileName := range p.fileMap {
		for _, xpattern := range excludePatterns {
			found, err := doublestar.Match(xpattern, srcFileName)
			if err != nil {
				log.Warnf("saveArtifacts - copy files - [%v] excludePatterns Match error - %v\n", srcFileName, err)
				//should only happen when the pattern is malformed
				continue
			}
			if found {
				log.Debugf("saveArtifacts - copy files - [%v] - excluding (%s) ", srcFileName, xpattern)
				continue copyFiles
			}
		}

		//filter out pid files (todo: have a flag to enable/disable these capabilities)
		if isKnownPidFilePath(srcFileName) {
			log.Debugf("saveArtifacts - copy files - skipping known pid file (%v)", srcFileName)
			extraDirs[fsutil.FileDir(srcFileName)] = struct{}{}
			continue
		}

		if hasPidFileSuffix(srcFileName) {
			log.Debugf("saveArtifacts - copy files - skipping a pid file (%v)", srcFileName)
			extraDirs[fsutil.FileDir(srcFileName)] = struct{}{}
			continue
		}

		dstFilePath := fmt.Sprintf("%s/files%s", p.storeLocation, srcFileName)
		log.Debug("saveArtifacts - saving file data => ", dstFilePath)
		//err := cpFile(fileName, filePath)
		err := fsutil.CopyRegularFile(p.cmd.KeepPerms, srcFileName, dstFilePath, true)
		if err != nil {
			log.Warn("saveArtifacts - error saving file => ", err)
		}
	}

	log.Debugf("saveArtifacts - copy links (%v)", len(p.linkMap))
copyLinks:
	for linkName, linkProps := range p.linkMap {
		for _, xpattern := range excludePatterns {
			found, err := doublestar.Match(xpattern, linkName)
			if err != nil {
				log.Warnf("saveArtifacts - copy links - [%v] excludePatterns Match error - %v\n", linkName, err)
				//should only happen when the pattern is malformed
				continue
			}
			if found {
				log.Debugf("saveArtifacts - copy links - [%v] - excluding (%s) ", linkName, xpattern)
				continue copyLinks
			}
		}

		//TODO: review
		linkPath := fmt.Sprintf("%s/files%s", p.storeLocation, linkName)
		linkDir := fsutil.FileDir(linkPath)
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

	log.Debug("saveArtifacts - copy additional files checked at runtime....")
	ngxEnsured := false
	for fileName := range p.fileMap {
		filePath := fmt.Sprintf("%s/files%s", p.storeLocation, fileName)

		if isRbGemSpecFile(fileName) {
			log.Debug("saveArtifacts - processing ruby gem spec ==>", fileName)
			err := rbEnsureGemFiles(fileName, p.storeLocation, "/files")
			if err != nil {
				log.Warn("saveArtifacts - error ensuring ruby gem files => ", err)
			}
		} else if isNodePackageFile(fileName) {
			log.Debug("saveArtifacts - processing node package file ==>", fileName)
			err := nodeEnsurePackageFiles(p.cmd.KeepPerms, fileName, p.storeLocation, "/files")
			if err != nil {
				log.Warn("saveArtifacts - error ensuring node package files => ", err)
			}
		} else if isNgxArtifact(fileName) && !ngxEnsured {
			log.Debug("saveArtifacts - ensuring ngx artifacts....")
			ngxEnsure(p.storeLocation)
			ngxEnsured = true
		} else {
			err := fixPy3CacheFile(fileName, filePath)
			if err != nil {
				log.Warn("saveArtifacts - error fixing py3 cache file => ", err)
			}
		}
	}

	if p.cmd.AppUser != "" {
		//always copy the '/etc/passwd' file when we have a user
		//later: do it only when AppUser is a name (not UID)
		passwdFilePath := "/etc/passwd"
		passwdFileTargetPath := fmt.Sprintf("%s/files%s", p.storeLocation, passwdFilePath)
		if _, err := os.Stat(passwdFilePath); err == nil {
			//if err := cpFile(passwdFilePath, passwdFileTargetPath); err != nil {
			if err := fsutil.CopyRegularFile(p.cmd.KeepPerms, passwdFilePath, passwdFileTargetPath, true); err != nil {
				log.Warn("sensor: monitor - error copying user info file =>", err)
			}
		} else {
			if os.IsNotExist(err) {
				log.Debug("sensor: monitor - no user info file")
			} else {
				log.Debug("sensor: monitor - could not save user info file =>", err)
			}
		}
	}

copyIncludes:
	for inPath, isDir := range includePaths {
		dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, inPath)
		if isDir {
			err, errs := fsutil.CopyDir(p.cmd.KeepPerms, inPath, dstPath, true, true, excludePatterns, nil, nil)
			if err != nil {
				log.Warnf("CopyDir(%v,%v) error: %v", inPath, dstPath, err)
			}

			if len(errs) > 0 {
				log.Warnf("CopyDir(%v,%v) copy errors: %+v", inPath, dstPath, errs)
			}
		} else {
			for _, xpattern := range excludePatterns {
				found, err := doublestar.Match(xpattern, inPath)
				if err != nil {
					log.Warnf("saveArtifacts - copy includes - [%v] excludePatterns Match error - %v\n", inPath, err)
					//should only happen when the pattern is malformed
					continue
				}
				if found {
					log.Debugf("saveArtifacts - copy includes - [%v] - excluding (%s) ", inPath, xpattern)
					continue copyIncludes
				}
			}

			if err := fsutil.CopyFile(p.cmd.KeepPerms, inPath, dstPath, true); err != nil {
				log.Warnf("CopyFile(%v,%v) error: %v", inPath, dstPath, err)
			}
		}
	}

	for _, exePath := range p.cmd.IncludeExes {
		exeArtifacts, err := sodeps.AllExeDependencies(exePath, true)
		if err != nil {
			log.Warnf("saveArtifacts - %v - error getting exe artifacts => %v\n", exePath, err)
			continue
		}

		log.Debugf("saveArtifacts - include exe [%s]: artifacts (%d):\n%v\n",
			exePath, len(exeArtifacts), strings.Join(exeArtifacts, "\n"))

		for _, apath := range exeArtifacts {
			dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, apath)
			if err := fsutil.CopyFile(p.cmd.KeepPerms, apath, dstPath, true); err != nil {
				log.Warnf("CopyFile(%v,%v) error: %v", apath, dstPath, err)
			}
		}
	}

	for _, binPath := range p.cmd.IncludeBins {
		binArtifacts, err := sodeps.AllDependencies(binPath)
		if err != nil {
			log.Warnf("saveArtifacts - %v - error getting bin artifacts => %v\n", binPath, err)
			continue
		}

		log.Debugf("saveArtifacts - include bin [%s]: artifacts (%d):\n%v\n",
			binPath, len(binArtifacts), strings.Join(binArtifacts, "\n"))

		for _, bpath := range binArtifacts {
			dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, bpath)
			if err := fsutil.CopyFile(p.cmd.KeepPerms, bpath, dstPath, true); err != nil {
				log.Warnf("CopyFile(%v,%v) error: %v", bpath, dstPath, err)
			}
		}
	}

	if p.cmd.IncludeShell {
		shellArtifacts, err := shellDependencies()
		if err == nil {
			log.Debugf("saveArtifacts - include shell: artifacts (%d):\n%v\n",
				len(shellArtifacts), strings.Join(shellArtifacts, "\n"))

			for _, spath := range shellArtifacts {
				dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, spath)
				if err := fsutil.CopyFile(p.cmd.KeepPerms, spath, dstPath, true); err != nil {
					log.Warnf("CopyFile(%v,%v) error: %v", spath, dstPath, err)
				}
			}
		} else {
			log.Warnf("saveArtifacts - error getting shell artifacts => %v", err)
		}

	}

	p.saveCertsData()

	if fsutil.DirExists("/tmp") {
		tdTargetPath := fmt.Sprintf("%s/files/tmp", p.storeLocation)
		if !fsutil.DirExists(tdTargetPath) {
			if err := os.MkdirAll(tdTargetPath, os.ModeSticky|os.ModeDir|0777); err != nil {
				log.Warn("saveArtifacts - error creating tmp directory => ", err)
			}
		} else {
			if err := os.Chmod(tdTargetPath, os.ModeSticky|os.ModeDir|0777); err != nil {
				log.Warn("saveArtifacts - error setting tmp directory permission ==> ", err)
			}
		}
	}

	if fsutil.DirExists("/run") {
		tdTargetPath := fmt.Sprintf("%s/files/run", p.storeLocation)
		if !fsutil.DirExists(tdTargetPath) {
			if err := os.MkdirAll(tdTargetPath, 0755); err != nil {
				log.Warn("saveArtifacts - error creating run directory => ", err)
			}
		}
	}

	for extraDir := range extraDirs {
		tdTargetPath := fmt.Sprintf("%s/files%s", p.storeLocation, extraDir)
		if fsutil.DirExists(extraDir) && !fsutil.DirExists(tdTargetPath) {
			if err := fsutil.CopyDirOnly(p.cmd.KeepPerms, extraDir, tdTargetPath); err != nil {
				log.Warnf("CopyDirOnly(%v,%v) error: %v", extraDir, tdTargetPath, err)
			}
		}
	}

	for inPath, perms := range newPerms {
		dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, inPath)
		if fsutil.Exists(dstPath) {
			if err := fsutil.SetAccess(dstPath, perms); err != nil {
				log.Warnf("SetPerms(%v,%v) error: %v", dstPath, perms, err)
			}
		}
	}

	if len(p.cmd.Preserves) > 0 {
		log.Debug("saveArtifacts: restoring preserved paths - %d", len(p.cmd.Preserves))

		preservedDirPath := filepath.Join(p.storeLocation, preservedDirName)
		filesDirPath := filepath.Join(p.storeLocation, filesDirName)
		if fsutil.Exists(preservedDirPath) {
			preservePaths := preparePaths(getKeys(p.cmd.Preserves))
			for inPath, isDir := range preservePaths {
				srcPath := fmt.Sprintf("%s%s", preservedDirPath, inPath)
				dstPath := fmt.Sprintf("%s%s", filesDirPath, inPath)

				if isDir {
					err, errs := fsutil.CopyDir(p.cmd.KeepPerms, srcPath, dstPath, true, true, nil, nil, nil)
					if err != nil {
						log.Warnf("saveArtifacts.CopyDir(%v,%v) error: %v", srcPath, dstPath, err)
					}

					if len(errs) > 0 {
						log.Warnf("saveArtifacts.CopyDir(%v,%v) copy errors: %+v", srcPath, dstPath, errs)
					}
				} else {
					if err := fsutil.CopyFile(p.cmd.KeepPerms, srcPath, dstPath, true); err != nil {
						log.Warnf("saveArtifacts.CopyFile(%v,%v) error: %v", srcPath, dstPath, err)
					}
				}
			}
		} else {
			log.Debug("saveArtifacts(): preserved root path doesnt exist")
		}
	}
}

func (p *artifactStore) archiveArtifacts() {
	src := filepath.Join(p.storeLocation, filesDirName)
	dst := filepath.Join(p.storeLocation, filesArchiveName)
	log.Debugf("artifactStore.archiveArtifacts: src='%s' dst='%s'", src, dst)

	trimPrefix := fmt.Sprintf("%s/", src)
	err := fsutil.ArchiveDir(dst, src, trimPrefix, "")
	errutil.FailOn(err)
}

func (p *artifactStore) saveReport() {
	sort.Strings(p.nameList)

	creport := report.ContainerReport{
		Monitors: report.MonitorReports{
			Pt:  p.ptMonReport,
			Fan: p.fanMonReport,
		},
	}

	sinfo := system.GetSystemInfo()
	creport.System = report.SystemReport{
		Type:    sinfo.Sysname,
		Release: sinfo.Release,
		Distro: report.DistroInfo{
			Name:        sinfo.Distro.Name,
			Version:     sinfo.Distro.Version,
			DisplayName: sinfo.Distro.DisplayName,
		},
	}

	for _, fname := range p.nameList {
		creport.Image.Files = append(creport.Image.Files, p.rawNames[fname])
	}

	reportName := defaultReportName

	_, err := os.Stat(p.storeLocation)
	if os.IsNotExist(err) {
		os.MkdirAll(p.storeLocation, 0777)
		_, err = os.Stat(p.storeLocation)
		errutil.FailOn(err)
	}

	reportFilePath := filepath.Join(p.storeLocation, reportName)
	log.Debugf("sensor: monitor - saving report to '%s'", reportFilePath)

	reportData, err := json.MarshalIndent(creport, "", "  ")
	errutil.FailOn(err)

	err = ioutil.WriteFile(reportFilePath, reportData, 0644)
	errutil.FailOn(err)
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

	cmd := exec.Command(fileTypeCmd, artifactFileName)
	cmd.Stderr = &cerr
	cmd.Stdout = &cout

	if err := cmd.Start(); err != nil {
		return "", err
	}

	if err := cmd.Wait(); err != nil {
		err = fmt.Errorf("getDataType - error getting data type: %s / stderr: %s", err, cerr.String())
		return "", err
	}

	if typeInfo := strings.Split(strings.TrimSpace(cout.String()), ":"); len(typeInfo) > 1 {
		return strings.TrimSpace(typeInfo[1]), nil
	}

	return "unknown", nil
}

/*


func cpFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		log.Warnln("sensor: monitor - cp - error opening source file =>", src)
		return err
	}
	defer s.Close()

	dstDir := fsutil.FileDir(dst)
	err = os.MkdirAll(dstDir, 0777)
	if err != nil {
		log.Warnln("sensor: monitor - dir error =>", err)
	}

	d, err := os.Create(dst)
	if err != nil {
		log.Warnln("sensor: monitor - cp - error opening dst file =>", dst)
		return err
	}

	//todo: copy owner info...

	srcFileInfo, err := s.Stat()
	if err == nil {
		if err := d.Chmod(srcFileInfo.Mode()); err != nil {
			log.Warnln("sensor: cpFile - unable to set mode =>", dst)
		}
	}

	if _, err := io.Copy(d, s); err != nil {
		d.Close()
		return err
	}

	if err := d.Close(); err != nil {
		return err
	}

	sysStat, ok := srcFileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		log.Warnln("sensor: cpFile - unable to get Stat_t =>", src)
		return nil
	}

	//note: cpFile() is only for regular files
	if srcFileInfo.Mode()&os.ModeSymlink != 0 {
		log.Warnln("sensor: cpFile - source is a symlink =>", src)
		return nil
	}

	//note: need to do the same for symlinks too
	if err := fsutil.UpdateFileTimes(dst, sysStat.Atim, sysStat.Mtim); err != nil {
		log.Warnln("sensor: cpFile - UpdateFileTimes error =>", dst)
		return err
	}

	return nil
}
*/

func py3FileNameFromCache(p string) string {
	ext := path.Ext(p)

	if !(((ext == pycExt) || (ext == pyoExt)) && strings.Contains(p, pycacheDir)) {
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
		//if err := cpFile(srcPyFilePath, dstPyFilePath); err != nil {
		if err := fsutil.CopyRegularFile(true, srcPyFilePath, dstPyFilePath, true); err != nil {
			log.Warnln("sensor: monitor - fixPy3CacheFile - error copying file =>", dstPyFilePath)
			return err
		}
	}

	return nil
}

func rbEnsureGemFiles(src, storeLocation, prefix string) error {
	if strings.Contains(src, rbDefaultSpecSubDir) {
		return nil
	}

	dir, file := path.Split(src)
	base := strings.TrimSuffix(dir, rbSpecSubDir)
	gemName := strings.TrimSuffix(file, rbGemSpecExt)

	extBasePath := filepath.Join(base, rgExtSibDir)
	foList, err := ioutil.ReadDir(extBasePath)
	if err != nil {
		return err
	}

	for _, fo := range foList {
		if fo.IsDir() {
			platform := fo.Name()

			extPlatformPath := filepath.Join(extBasePath, platform)
			foVerList, err := ioutil.ReadDir(extPlatformPath)
			if err != nil {
				return err
			}

			for _, foVer := range foVerList {
				if foVer.IsDir() {
					rversion := foVer.Name()

					extBuildFlagFilePath := filepath.Join(base, rgExtSibDir, platform, rversion, gemName, rbGemBuildFlag)

					if _, err := os.Stat(extBuildFlagFilePath); err != nil && os.IsNotExist(err) {
						log.Debug("sensor: monitor - rbEnsureGemFiles - no native extensions for gem =>", gemName)
						continue
					}

					extBuildFlagFilePathDst := fmt.Sprintf("%s%s%s", storeLocation, prefix, extBuildFlagFilePath)

					if _, err := os.Stat(extBuildFlagFilePathDst); err != nil && os.IsNotExist(err) {
						//if err := cpFile(extBuildFlagFilePath, extBuildFlagFilePathDst); err != nil {
						if err := fsutil.CopyRegularFile(true, extBuildFlagFilePath, extBuildFlagFilePathDst, true); err != nil {
							log.Warnln("sensor: monitor - rbEnsureGemFiles - error copying file =>", extBuildFlagFilePathDst)
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

func isRbGemSpecFile(filePath string) bool {
	ext := path.Ext(filePath)

	if ext == rbGemSpecExt && strings.Contains(filePath, rbSpecSubDir) {
		return true
	}

	return false
}

func isNodePackageFile(filePath string) bool {
	fileName := filepath.Base(filePath)

	if fileName == nodePackageFile {
		return true
	}

	//TODO: read the file and verify that it's a real package file
	return false
}

func nodeEnsurePackageFiles(keepPerms bool, src, storeLocation, prefix string) error {
	if strings.HasSuffix(src, nodeNPMNodeGypPackage) {
		//for now only ensure that we have node-gyp for npm
		//npm requires it to be there even though it won't use it
		//'check if exists' condition (not picked up by the current FS monitor)
		nodeGypFilePath := path.Join(filepath.Dir(src), nodeNPMNodeGypFile)
		if _, err := os.Stat(nodeGypFilePath); err == nil {
			nodeGypFilePathDst := fmt.Sprintf("%s%s%s", storeLocation, prefix, nodeGypFilePath)
			if err := fsutil.CopyRegularFile(keepPerms, nodeGypFilePath, nodeGypFilePathDst, true); err != nil {
				log.Warnf("sensor: nodeEnsurePackageFiles - error copying %s => %v", nodeGypFilePath, err)
			}
		}
	}

	//NOTE: can also read the dependencies and confirm/ensure that we copied everything we need
	return nil
}

var pidFilePathSuffixes = []string{
	"/var/run/nginx.pid",
	"/run/nginx.pid",
	"/tmp/nginx.pid",
	"/tmp/pids/server.pid",
}

func isKnownPidFilePath(filePath string) bool {
	for _, suffix := range pidFilePathSuffixes {
		if strings.HasSuffix(filePath, suffix) {
			return true
		}
	}

	return false
}

func hasPidFileSuffix(filePath string) bool {
	if strings.HasSuffix(filePath, pidFileSuffix) {
		return true
	}

	return false
}

func isNgxArtifact(filePath string) bool {
	if strings.Contains(filePath, ngxSubDir) || strings.HasSuffix(filePath, ngxBinName) {
		return true
	}

	return false
}

func ngxEnsure(prefix string) {
	//ensure common temp paths (note: full implementation needs mkdir syscall info)
	if info, err := os.Stat(ngxCommonTemp); err == nil {
		if info.IsDir() {
			dstPath := fmt.Sprintf("%s/files%s", prefix, ngxCommonTemp)
			if !fsutil.DirExists(dstPath) {
				err := os.MkdirAll(dstPath, 0777)
				//err, errs := fsutil.CopyDir(true, ngxCommonTemp, dstPath, true, true, nil, nil, nil)
				if err != nil {
					log.Warnf("ngxEnsure - MkdirAll(%v) error: %v", dstPath, err)
				}
				//if len(errs) > 0 {
				//	log.Warnf("ngxEnsure - CopyDir copy error: %+v", errs)
				//}
			}
		} else {
			log.Debugf("ngxEnsure - %v should be a directory", ngxCommonTemp)
		}
	} else {
		if !os.IsNotExist(err) {
			log.Debugf("ngxEnsure - error checking %v => %v", ngxCommonTemp, err)
		}
	}

	if info, err := os.Stat(ngxLogTemp); err == nil {
		if info.IsDir() {
			dstPath := fmt.Sprintf("%s/files%s", prefix, ngxLogTemp)
			if !fsutil.DirExists(dstPath) {
				err := os.MkdirAll(dstPath, 0777)
				if err != nil {
					log.Warnf("ngxEnsure -  MkdirAll(%v) error: %v", dstPath, err)
				}
			}
		} else {
			log.Debugf("ngxEnsure - %v should be a directory", ngxLogTemp)
		}
	} else {
		if !os.IsNotExist(err) {
			log.Debugf("ngxEnsure - error checking %v => %v", ngxLogTemp, err)
		}
	}

	if info, err := os.Stat(ngxCacheTemp); err == nil {
		if info.IsDir() {
			dstPath := fmt.Sprintf("%s/files%s", prefix, ngxCacheTemp)
			if !fsutil.DirExists(dstPath) {
				err := os.MkdirAll(dstPath, 0777)
				if err != nil {
					log.Warnf("ngxEnsure -  MkdirAll(%v) error: %v", dstPath, err)
				}
			}
		} else {
			log.Debugf("ngxEnsure - %v should be a directory", ngxCacheTemp)
		}
	} else {
		if !os.IsNotExist(err) {
			log.Debugf("ngxEnsure - error checking %v => %v", ngxCacheTemp, err)
		}
	}
}

var shellNames = []string{
	"bash",
	"sh",
}

var shellCommands = []string{
	"ls",
	"pwd",
	"cd",
	"ps",
	"head",
	"tail",
	"cat",
	"more",
	"find",
	"grep",
	"awk",
	"env",
}

func shellDependencies() ([]string, error) {
	var allDeps []string
	for _, name := range shellNames {
		shellPath, err := exec.LookPath(name)
		if err != nil {
			log.Debugf("shellDependencies - checking '%s' shell (not found: %s)", name, err)
			continue
		}

		exeArtifacts, err := sodeps.AllExeDependencies(shellPath, true)
		if err != nil {
			log.Warnf("shellDependencies - %v - error getting shell artifacts => %v", shellPath, err)
			return nil, err
		}

		allDeps = append(allDeps, exeArtifacts...)
		break
	}

	if len(allDeps) == 0 {
		log.Warnf("shellDependencies - no shell found")
		return nil, nil
	}

	for _, name := range shellCommands {
		cmdPath, err := exec.LookPath(name)
		if err != nil {
			log.Debugf("shellDependencies - checking '%s' cmd (not found: %s)", name, err)
			continue
		}

		cmdArtifacts, err := sodeps.AllExeDependencies(cmdPath, true)
		if err != nil {
			log.Warnf("shellDependencies - %v - error getting cmd artifacts => %v", cmdPath, err)
			return nil, err
		}

		allDeps = append(allDeps, cmdArtifacts...)
	}

	return allDeps, nil
}
