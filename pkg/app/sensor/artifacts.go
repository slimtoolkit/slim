//go:build linux
// +build linux

package sensor

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

	"github.com/armon/go-radix"
	"github.com/bmatcuk/doublestar/v3"
	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/app/sensor/detectors/binfile"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/inspectors/sodeps"
	"github.com/docker-slim/docker-slim/pkg/certdiscover"
	"github.com/docker-slim/docker-slim/pkg/ipc/command"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/system"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
)

const (
	pidFileSuffix    = ".pid"
	varRunDir        = "/var/run/"
	fileTypeCmdName  = "file"
	filesDirName     = "files"
	filesArchiveName = "files.tar"
	preservedDirName = "preserved"
)

//TODO: extract these app, framework and language specific login into separate packages

// Nginx related consts
const (
	ngxBinName    = "/nginx"
	ngxSubDir     = "/nginx/"
	ngxCommonTemp = "/var/lib/nginx"
	ngxLogTemp    = "/var/log/nginx"
	ngxCacheTemp  = "/var/cache/nginx"
)

// Ruby related consts
const (
	rbBinName           = "/ruby"
	rbIrbBinName        = "/irb"
	rbGemBinName        = "/gem"
	rbBundleBinName     = "/bundle"
	rbRbenvBinName      = "/rbenv"
	rbSrcFileExt        = ".rb"
	rbGemSpecExt        = ".gemspec"
	rbGemsSubDir        = "/gems/"
	rbGemfile           = "Gemfile"
	rbGemfileLockFile   = "Gemfile.lock"
	rbDefaultSpecSubDir = "/specifications/default/"
	rbSpecSubDir        = "/specifications/"
	rgExtSibDir         = "extensions"
	rbGemBuildFlag      = "gem.build_complete"
)

// Python related consts
const (
	pyBinName            = "/python"
	py2BinName           = "/python2"
	py3BinName           = "/python3"
	pyPipBinName         = "/pip"
	pyPip2BinName        = "/pip2"
	pyPip3BinName        = "/pip3"
	pyPoetryBinName      = "/poetry"
	pyCondaBinName       = "/conda"
	pyPipEnvBinName      = "/pipenv"
	pyEasyInstallBinName = "/easy_install"
	pyPipxBinName        = "/pipx"
	pyVirtEnvBinName     = "/virtualenv"
	pySrcFileExt         = ".py"
	pycExt               = ".pyc"
	pyoExt               = ".pyo"
	pycacheDir           = "/__pycache__/"
	pycache              = "__pycache__"
	pyReqsFile           = "/requirements.txt"
	pyPoetryProjectFile  = "/pyproject.toml"
	pyPipEnvProjectFile  = "/Pipfile"
	pyPipEnvLockFile     = "/Pipfile.lock"
	pyDistPkgDir         = "/dist-packages/"
	pySitePkgDir         = "/site-packages/"
)

// Node.js related consts
const (
	nodeBinName           = "/node"
	nodeNpmBinName        = "/npm"
	nodeYarnBinName       = "/yarn"
	nodePnpmBinName       = "/pnpm"
	nodeRushBinName       = "/rush"
	nodeLernaBinName      = "/lerna"
	nodeSrcFileExt        = ".js"
	nodePackageFile       = "package.json"
	nodePackageLockFile   = "package-lock.json"
	nodeNpmShrinkwrapFile = "npm-shrinkwrap.json"
	nodeYarnLockFile      = "yarn.lock"
	nodePackageDirPath    = "/node_modules/"
	nodePackageDirName    = "node_modules"
	nodeNPMNodeGypPackage = "/npm/node_modules/node-gyp/package.json"
	nodeNPMNodeGypFile    = "bin/node-gyp.js"
)

// nuxt.js related consts
const (
	nuxtConfigFile      = "nuxt.config.js"
	nuxtBuildDirKey     = "buildDir"
	nuxtSrcDirKey       = "srcDir" //defaults to rootDir
	nuxtDistDirKey      = "dir"    //in 'generate'
	nuxtDefaultDistDir  = "dist"
	nuxtDefaultBuildDir = ".nuxt"
	nuxtStaticDir       = "static"
)

// next.js related consts
const (
	nextConfigFile                = "next.config.js"
	nextConfigFileAlt             = "next.config.mjs"
	nextDefaultBuildDir           = ".next"
	nextDefaultBuildStandaloneDir = ".next/standalone"
	nextDefaultBuildStaticDir     = ".next/static"
	nextStaticDir                 = "public"
	nextDefaultStaticSpaDir       = "out"
	nextDefaultStaticSpaDirPath   = "/out/_next/"
)

type NodePackageConfigSimple struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Dependencies map[string]string `json:"dependencies"`
}

type appStackInfo struct {
	language    string //will be reusing language consts from certdiscover (todo: replace it later)
	codeFiles   uint
	packageDirs map[string]struct{}
}

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

func saveResults(
	artifactDirName string,
	cmd *command.StartMonitor,
	fileNames map[string]*report.ArtifactProps,
	fanMonReport *report.FanMonitorReport,
	ptMonReport *report.PtMonitorReport,
	peReport *report.PeMonitorReport,
) {
	log.Debugf("saveResults(%v,...)", len(fileNames))

	artifactStore := newArtifactStore(artifactDirName, fileNames, fanMonReport, ptMonReport, peReport, cmd)
	artifactStore.prepareArtifacts()
	artifactStore.saveArtifacts()
	artifactStore.enumerateArtifacts()
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
	saFileMap     map[string]*report.ArtifactProps
	cmd           *command.StartMonitor
	appStacks     map[string]*appStackInfo
}

func newArtifactStore(
	storeLocation string,
	rawNames map[string]*report.ArtifactProps,
	fanMonReport *report.FanMonitorReport,
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
		saFileMap:     map[string]*report.ArtifactProps{},
		cmd:           cmd,
		appStacks:     map[string]*appStackInfo{},
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

		//build absolute and evaluated symlink target paths
		var absLinkRef string
		if !filepath.IsAbs(linkRef) {
			linkDir := filepath.Dir(artifactFileName)
			fullLinkRef := filepath.Join(linkDir, linkRef)
			absLinkRef, err = filepath.Abs(fullLinkRef)
			if err != nil {
				log.Warnf("prepareArtifact - error getting absolute path for symlink ref (%v) -> %v => %v", err, artifactFileName, fullLinkRef)
			}
		} else {
			absLinkRef, err = filepath.Abs(linkRef)
			if err != nil {
				log.Warnf("prepareArtifact - error getting absolute path for symlink ref 2 (%v) -> %v => %v", err, artifactFileName, linkRef)
			}
		}

		if absLinkRef != "" {
			evalLinkRef, err := filepath.EvalSymlinks(absLinkRef)
			if err != nil {
				log.Warnf("prepareArtifact - error evaluating symlink (%v) -> %v => %v", err, artifactFileName, absLinkRef)
			} else {
				if evalLinkRef != absLinkRef {
					if _, ok := p.rawNames[evalLinkRef]; !ok {
						p.resolve[evalLinkRef] = struct{}{}
					}
				}
			}

			if _, ok := p.rawNames[absLinkRef]; !ok {
				p.resolve[absLinkRef] = struct{}{}
			}
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

	if p.ptMonReport.Enabled {
		log.Debug("prepareArtifacts - ptMonReport.Enabled")
		for artifactFileName, fsaInfo := range p.ptMonReport.FSActivity {
			artifactInfo, found := p.rawNames[artifactFileName]
			if found {
				artifactInfo.FSActivity = fsaInfo
			} else {
				log.Debugf("prepareArtifacts - fsa artifact => %v", artifactFileName)
				p.prepareArtifact(artifactFileName)
				artifactInfo, found := p.rawNames[artifactFileName]
				if found {
					artifactInfo.FSActivity = fsaInfo
				} else {
					log.Errorf("prepareArtifacts - fsa artifact - missing in rawNames => %v", artifactFileName)
				}

				//TMP:
				//fsa might include directories, which we'll need to copy (dir only)
				//but p.prepareArtifact() doesn't do anything with dirs for now
			}
		}
	}

	for artifactFileName := range p.fileMap {
		//TODO: conditionally detect binary files and their deps
		if isBin, _ := binfile.Detected(artifactFileName); !isBin {
			continue
		}

		binArtifacts, err := sodeps.AllDependencies(artifactFileName)
		if err != nil {
			if err == sodeps.ErrDepResolverNotFound {
				log.Debug("prepareArtifacts.binArtifacts[bsa] - no static bin dep resolver")
			} else {
				log.Warnf("prepareArtifacts.binArtifacts[bsa] - %v - error getting bin artifacts => %v\n", artifactFileName, err)
			}
			continue
		}

		for idx, bpath := range binArtifacts {
			if artifactFileName == bpath {
				continue
			}

			_, found := p.rawNames[bpath]
			if found {
				log.Debugf("prepareArtifacts.binArtifacts[bsa] - known file path (%s)", bpath)
				continue
			}

			bpathFileInfo, err := os.Lstat(bpath)
			if err != nil {
				log.Warnf("prepareArtifacts.binArtifacts[bsa] - artifact doesn't exist: %v (%v)", bpath, os.IsNotExist(err))
				continue
			}

			bprops := &report.ArtifactProps{
				FilePath: bpath,
				Mode:     bpathFileInfo.Mode(),
				ModeText: bpathFileInfo.Mode().String(),
				FileSize: bpathFileInfo.Size(),
			}

			bprops.Flags = p.getArtifactFlags(bpath)

			fsType := "unknown"
			switch {
			case bpathFileInfo.Mode().IsRegular():
				fsType = "file"
				p.rawNames[bpath] = bprops
				//use a separate file map, so we can save them last
				//in case we are dealing with intermediate symlinks
				//and to better track what bin deps are not covered by dynamic analysis
				p.saFileMap[bpath] = bprops
			case (bpathFileInfo.Mode() & os.ModeSymlink) != 0:
				fsType = "symlink"
				p.linkMap[bpath] = bprops
				p.rawNames[bpath] = bprops
			default:
				fsType = "unexpected"
				log.Warnf("prepareArtifacts.binArtifacts[bsa] - unexpected ft - %s", bpath)
			}

			log.Debugf("prepareArtifacts.binArtifacts[bsa] - bin artifact (%s) fsType=%s [%d]bdep=%s", artifactFileName, fsType, idx, bpath)
		}
	}

	p.resolveLinks()
}

func (p *artifactStore) resolveLinks() {
	//note:
	//the links should be resolved in findSymlinks, but
	//the current design needs to be improved to catch all symlinks
	//this is a backup to catch the root level symlinks
	files, err := ioutil.ReadDir("/")
	if err != nil {
		log.Debug("resolveLinks - ioutil.ReadDir error: ", err)
		return
	}

	for _, file := range files {
		fpath := fmt.Sprintf("/%s", file.Name())
		log.Debugf("resolveLinks.files - fpath='%s'", fpath)

		if fpath == "/proc" ||
			fpath == "/sys" ||
			fpath == "/dev" {
			continue
		}

		fileInfo, err := os.Lstat(fpath)
		if err != nil {
			log.Debugf("resolveLinks.files - os.Lstat(%s) error: ", fpath, err)
			continue
		}

		if fileInfo.Mode()&os.ModeSymlink == 0 {
			log.Debug("resolveLinks.files - skipping non-symlink")
			continue
		}

		linkRef, err := os.Readlink(fpath)
		if err != nil {
			log.Debugf("resolveLinks.files - os.Readlink(%s) error: ", fpath, err)
			continue
		}

		var absLinkRef string
		if !filepath.IsAbs(linkRef) {
			linkDir := filepath.Dir(fpath)
			log.Debugf("resolveLinks.files - relative linkRef %v -> %v +/+ %v", fpath, linkDir, linkRef)
			fullLinkRef := filepath.Join(linkDir, linkRef)
			var err error
			absLinkRef, err = filepath.Abs(fullLinkRef)
			if err != nil {
				log.Warnf("resolveLinks.files - error getting absolute path for symlink ref (1) (%v) -> %v => %v", err, fpath, fullLinkRef)
				continue
			}
		} else {
			var err error
			absLinkRef, err = filepath.Abs(linkRef)
			if err != nil {
				log.Warnf("resolveLinks.files - error getting absolute path for symlink ref (2) (%v) -> %v => %v", err, fpath, linkRef)
				continue
			}
		}

		//todo: skip "/proc/..." references
		evalLinkRef, err := filepath.EvalSymlinks(absLinkRef)
		if err != nil {
			log.Warnf("resolveLinks.files - error evaluating symlink (%v) -> %v => %v", err, fpath, absLinkRef)
		}

		//detecting intermediate dir symlinks
		symlinkPrefix := fmt.Sprintf("%s/", fpath)
		absPrefix := fmt.Sprintf("%s/", absLinkRef)
		evalPrefix := fmt.Sprintf("%s/", evalLinkRef)
		for rawName := range p.rawNames {
			if strings.HasPrefix(rawName, symlinkPrefix) {
				if _, found := p.rawNames[fpath]; found {
					log.Debugf("resolveLinks.files - rawNames - known symlink: name=%s target=%s", fpath, symlinkPrefix)
				} else {
					p.rawNames[fpath] = nil
					log.Debugf("resolveLinks.files - added path symlink to p.rawNames (0) -> %v", fpath)
					p.prepareArtifact(fpath)
				}
				break
			}

			if strings.HasPrefix(rawName, absPrefix) {
				if _, found := p.rawNames[fpath]; found {
					log.Debugf("resolveLinks.files - rawNames - known symlink: name=%s target=%s", fpath, absPrefix)
				} else {
					p.rawNames[fpath] = nil
					log.Debugf("resolveLinks.files - added path symlink to p.rawNames (1) -> %v", fpath)
					p.prepareArtifact(fpath)
				}
				break
			}

			if evalLinkRef != "" &&
				absPrefix != evalPrefix &&
				strings.HasPrefix(rawName, evalPrefix) {
				if _, found := p.rawNames[fpath]; found {
					log.Debugf("resolveLinks.files - rawNames - known symlink: name=%s target=%s", fpath, evalPrefix)
				} else {
					p.rawNames[fpath] = nil
					log.Debugf("resolveLinks.files - added path symlink to p.rawNames (2) -> %v", fpath)
					p.prepareArtifact(fpath)
				}
				break
			}
		}
	}

	//note: resolve these extra symlinks after the root level symlinks
	for name := range p.resolve {
		log.Debug("resolveLinks - resolving: ", name)
		p.prepareArtifact(name)
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

// copied from dockerimage.go
func linkTargetToFullPath(fullPath, target string) string {
	if filepath.IsAbs(target) {
		return target
	}

	if target == "." {
		return ""
	}

	d := filepath.Dir(fullPath)

	return filepath.Clean(filepath.Join(d, target))
}

func (p *artifactStore) saveCertsData() {
	copyCertFiles := func(list []string) {
		log.Debugf("sensor.artifactStore.saveCertsData.copyCertFiles(list=%+v)", list)
		for _, fname := range list {
			if fsutil.Exists(fname) {
				dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, fname)
				if err := fsutil.CopyFile(p.cmd.KeepPerms, fname, dstPath, true); err != nil {
					log.Warnf("sensor.artifactStore.saveCertsData.copyCertFiles: fsutil.CopyFile(%v,%v) error - %v", fname, dstPath, err)
				}
			}
		}
	}

	copyDirs := func(list []string, copyLinkTargets bool) {
		log.Debugf("sensor.artifactStore.saveCertsData.copyDirs(list=%+v,copyLinkTargets=%v)", list, copyLinkTargets)
		for _, fname := range list {
			if fsutil.Exists(fname) {
				dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, fname)

				if fsutil.IsDir(fname) {
					err, errs := fsutil.CopyDir(p.cmd.KeepPerms, fname, dstPath, true, true, nil, nil, nil)
					if err != nil {
						log.Warnf("sensor.artifactStore.saveCertsData.copyDirs: fsutil.CopyDir(%v,%v) error: %v", fname, dstPath, err)
					} else if copyLinkTargets {
						foList, err := ioutil.ReadDir(fname)
						if err == nil {
							log.Debugf("sensor.artifactStore.saveCertsData.copyDirs(): dir=%v fcount=%v", fname, len(foList))
							for _, fo := range foList {
								fullPath := filepath.Join(fname, fo.Name())
								log.Debugf("sensor.artifactStore.saveCertsData.copyDirs(): dir=%v fullPath=%v", fname, fullPath)
								if fsutil.IsSymlink(fullPath) {
									linkRef, err := os.Readlink(fullPath)
									if err != nil {
										log.Warnf("sensor.artifactStore.saveCertsData.copyDirs: os.Readlink(%v) error - %v", fullPath, err)
										continue
									}

									log.Debugf("sensor.artifactStore.saveCertsData.copyDirs(): dir=%v fullPath=%v linkRef=%v",
										fname, fullPath, linkRef)
									if strings.Contains(linkRef, "/") {
										targetFilePath := linkTargetToFullPath(fullPath, linkRef)
										if targetFilePath != "" && fsutil.Exists(targetFilePath) {
											log.Debugf("sensor.artifactStore.saveCertsData.copyDirs(): dir=%v fullPath=%v linkRef=%v targetFilePath=%v",
												fname, fullPath, linkRef, targetFilePath)
											dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, targetFilePath)
											if err := fsutil.CopyFile(p.cmd.KeepPerms, targetFilePath, dstPath, true); err != nil {
												log.Warnf("sensor.artifactStore.saveCertsData.copyDirs: fsutil.CopyFile(%v,%v) error - %v", targetFilePath, dstPath, err)
											}
										} else {
											log.Warnf("sensor.artifactStore.saveCertsData.copyDirs: targetFilePath does not exist - %v", targetFilePath)
										}
									}
								}
							}
						} else {
							log.Warnf("sensor.artifactStore.saveCertsData.copyDirs: os.ReadDir(%v) error - %v", fname, err)
						}
					}

					if len(errs) > 0 {
						log.Warnf("sensor.artifactStore.saveCertsData.copyDirs: fsutil.CopyDir(%v,%v) copy errors: %+v", fname, dstPath, errs)
					}
				} else if fsutil.IsSymlink(fname) {
					if err := fsutil.CopySymlinkFile(p.cmd.KeepPerms, fname, dstPath, true); err != nil {
						log.Warnf("sensor.artifactStore.saveCertsData.copyDirs: fsutil.CopySymlinkFile(%v,%v) error - %v", fname, dstPath, err)
					}
				} else {
					log.Warnf("artifactStore.saveCertsData.copyDir: unexpected obect type - %s", fname)
				}
			}
		}
	}

	copyAppCertFiles := func(suffix string, dirs []string, subdirPrefix string) {
		//NOTE: dirs end with "/" (need to revisit the formatting to make it consistent)
		log.Debugf("sensor.artifactStore.saveCertsData.copyAppCertFiles(suffix=%v,dirs=%+v,subdirPrefix=%v)",
			suffix, dirs, subdirPrefix)
		for _, dirName := range dirs {
			if subdirPrefix != "" {
				foList, err := ioutil.ReadDir(dirName)
				if err != nil {
					log.Warnf("sensor.artifactStore.saveCertsData.copyAppCertFiles: os.ReadDir(%v) error - %v", dirName, err)
					continue
				}

				for _, fo := range foList {
					if strings.HasPrefix(fo.Name(), subdirPrefix) {
						dirName = fmt.Sprintf("%s%s/", dirName, fo.Name())
						break
					}
				}
			}

			srcFilePath := fmt.Sprintf("%s%s", dirName, suffix)
			if fsutil.Exists(srcFilePath) {
				dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, srcFilePath)
				if err := fsutil.CopyFile(p.cmd.KeepPerms, srcFilePath, dstPath, true); err != nil {
					log.Warnf("sensor.artifactStore.saveCertsData.copyAppCertFiles: fsutil.CopyFile(%v,%v) error - %v", srcFilePath, dstPath, err)
				}
			}
		}
	}

	setToList := func(in map[string]struct{}) []string {
		var out []string
		for k := range in {
			out = append(out, k)
		}

		return out
	}

	if p.cmd.IncludeCertAll {
		copyCertFiles(certdiscover.CertFileList())
		copyCertFiles(certdiscover.CACertFileList())
		//TODO:
		//need to 'walk' these directories detecting cert files
		//and only copying those files instead of copying all files
		copyDirs(certdiscover.CertDirList(), true)
		copyDirs(certdiscover.CACertDirList(), true)
		//shouldn't copy the extra dirs explicitly here
		//the actual cert files should be copied through links above
		copyDirs(certdiscover.CertExtraDirList(), false)

		for _, appStack := range p.appStacks {
			switch appStack.language {
			case certdiscover.LanguagePython:
				copyAppCertFiles(certdiscover.AppCertPathSuffixPython, setToList(appStack.packageDirs), "")
			case certdiscover.LanguageNode:
				copyAppCertFiles(certdiscover.AppCertPathSuffixNode, setToList(appStack.packageDirs), "")
			case certdiscover.LanguageRuby:
				//ruby needs the versioned package name too <prefix>certifi-zzzzz/<suffix>
				copyAppCertFiles(certdiscover.AppCertPathSuffixRuby,
					setToList(appStack.packageDirs),
					certdiscover.AppCertPackageName)
				//case certdiscover.LanguageJava:
			}
		}
	}

	if !p.cmd.IncludeCertAll && p.cmd.IncludeCertBundles {
		copyCertFiles(certdiscover.CertFileList())
		copyCertFiles(certdiscover.CACertFileList())

		for _, appStack := range p.appStacks {
			switch appStack.language {
			case certdiscover.LanguagePython:
				copyAppCertFiles(certdiscover.AppCertPathSuffixPython, setToList(appStack.packageDirs), "")
			case certdiscover.LanguageNode:
				copyAppCertFiles(certdiscover.AppCertPathSuffixNode, setToList(appStack.packageDirs), "")
			case certdiscover.LanguageRuby:
				//ruby needs the versioned package name too <prefix>certifi-zzzzz/<suffix>
				copyAppCertFiles(certdiscover.AppCertPathSuffixRuby,
					setToList(appStack.packageDirs),
					certdiscover.AppCertPackageName)
				//case certdiscover.LanguageJava:
			}
		}
	}

	if !p.cmd.IncludeCertAll && p.cmd.IncludeCertDirs {
		copyDirs(certdiscover.CertDirList(), true)
		copyDirs(certdiscover.CACertDirList(), true)
		copyDirs(certdiscover.CertExtraDirList(), false)
	}

	if p.cmd.IncludeCertPKAll {
		copyCertFiles(certdiscover.CACertPKFileList())
		//TODO:
		//need to 'walk' these directories detecting cert PK files
		//and only copying those files instead of copying all files
		copyDirs(certdiscover.CertPKDirList(), true)
		copyDirs(certdiscover.CACertPKDirList(), true)
	}

	if !p.cmd.IncludeCertPKAll && p.cmd.IncludeCertPKDirs {
		copyDirs(certdiscover.CertPKDirList(), true)
		copyDirs(certdiscover.CACertPKDirList(), true)
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

	if includePaths == nil {
		includePaths = map[string]bool{}
	}

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
	symlinkFailed := map[string]*report.ArtifactProps{}

	log.Debugf("saveArtifacts - copy links (%v)", len(p.linkMap))
	//copyLinks:
	//NOTE: MUST copy the links FIRST, so the dir symlinks get created before their files are copied
	symlinkMap := radix.New()
	for linkName, linkProps := range p.linkMap {
		symlinkMap.Insert(linkName, linkProps)
	}

	symlinkWalk := func(linkName string, val interface{}) bool {
		linkProps, ok := val.(*report.ArtifactProps)
		if !ok {
			log.Warnf("saveArtifacts.symlinkWalk: could not convert data - %s\n", linkName)
			return false
		}

		for _, xpattern := range excludePatterns {
			found, err := doublestar.Match(xpattern, linkName)
			if err != nil {
				log.Warnf("saveArtifacts.symlinkWalk - copy links - [%v] excludePatterns Match error - %v\n", linkName, err)
				//should only happen when the pattern is malformed
				return false
			}
			if found {
				log.Debugf("saveArtifacts.symlinkWalk - copy links - [%v] - excluding (%s) ", linkName, xpattern)
				return false
			}
		}

		//TODO: review
		linkPath := fmt.Sprintf("%s/files%s", p.storeLocation, linkName)
		linkDir := fsutil.FileDir(linkPath)
		//NOTE:
		//The symlink target dir might not exist, which means
		//the dir create calls that start with the current symlink prefix will fail.
		//We'll save the failed links to try again
		//later when the symlink target is already created.
		//Another option is to create the symlink targets,
		//but it might be tricky if the target is a symlink (potentially to another symlink, etc)

		//log.Debugf("saveArtifacts.symlinkWalk - saving symlink - create subdir: linkName=%s linkDir=%s linkPath=%s", linkName, linkDir, linkPath)
		err := os.MkdirAll(linkDir, 0777)
		if err != nil {
			log.Warnf("saveArtifacts.symlinkWalk - dir error (linkName=%s linkDir=%s linkPath=%s) => error=%v", linkName, linkDir, linkPath, err)
			//save it and try again later
			symlinkFailed[linkName] = linkProps
			return false
		}

		if linkProps != nil &&
			linkProps.FSActivity != nil &&
			linkProps.FSActivity.OpsCheckFile > 0 {
			log.Debug("saveArtifacts.symlinkWalk - saving 'checked' symlink => ", linkName)
		}

		//log.Debugf("saveArtifacts.symlinkWalk - saving symlink: name=%s target=%s", linkName, linkProps.LinkRef)
		err = os.Symlink(linkProps.LinkRef, linkPath)
		if err != nil {
			if os.IsExist(err) {
				log.Debug("saveArtifacts.symlinkWalk - symlink already exists")
			} else {
				log.Warn("saveArtifacts.symlinkWalk - symlink create error ==> ", err)
			}
		}

		return false
	}

	symlinkMap.Walk(symlinkWalk)

	for linkName, linkProps := range symlinkFailed {
		linkPath := fmt.Sprintf("%s/files%s", p.storeLocation, linkName)
		linkDir := fsutil.FileDir(linkPath)

		//log.Debugf("saveArtifacts.symlinkFailed - saving symlink - create subdir: linkName=%s linkDir=%s linkPath=%s", linkName, linkDir, linkPath)
		err := os.MkdirAll(linkDir, 0777)
		if err != nil {
			log.Warnf("saveArtifacts.symlinkFailed - dir error (linkName=%s linkDir=%s linkPath=%s) => error=%v", linkName, linkDir, linkPath, err)
			continue
		}

		if linkProps != nil &&
			linkProps.FSActivity != nil &&
			linkProps.FSActivity.OpsCheckFile > 0 {
			log.Debug("saveArtifacts.symlinkFailed - saving 'checked' symlink => ", linkName)
		}

		//log.Debugf("saveArtifacts.symlinkFailed - saving symlink: name=%s target=%s", linkName, linkProps.LinkRef)

		err = os.Symlink(linkProps.LinkRef, linkPath)
		if err != nil {
			if os.IsExist(err) {
				log.Debug("saveArtifacts.symlinkFailed - symlink already exists")
			} else {
				log.Warn("saveArtifacts.symlinkFailed - symlink create error ==> ", err)
			}
		}
	}

	log.Debugf("saveArtifacts - copy files (%v)", len(p.fileMap))
copyFiles:
	for srcFileName, artifactInfo := range p.fileMap {
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

		if artifactInfo != nil &&
			artifactInfo.FSActivity != nil &&
			artifactInfo.FSActivity.OpsCheckFile > 0 {
			log.Debug("saveArtifacts - saving 'checked' file => ", srcFileName)
			//NOTE: later have an option to save 'checked' only files without data
		}

		err := fsutil.CopyRegularFile(p.cmd.KeepPerms, srcFileName, dstFilePath, true)
		if err != nil {
			log.Warn("saveArtifacts - error saving file => ", err)
		}
	}

	//NOTE: need to copy the files after the links are copied
	log.Debug("saveArtifacts - copy additional files checked at runtime....")
	ngxEnsured := false
	for fileName := range p.fileMap {
		filePath := fmt.Sprintf("%s/files%s", p.storeLocation, fileName)
		p.detectAppStack(fileName)

		if p.cmd.IncludeAppNuxtDir ||
			p.cmd.IncludeAppNuxtBuildDir ||
			p.cmd.IncludeAppNuxtDistDir ||
			p.cmd.IncludeAppNuxtStaticDir ||
			p.cmd.IncludeAppNuxtNodeModulesDir {
			if isNuxtConfigFile(fileName) {
				nuxtConfig, err := getNuxtConfig(fileName)
				if err != nil {
					log.Warn("saveArtifacts: failed to get nuxt config: %v", err)
					continue
				}
				if nuxtConfig == nil {
					log.Warn("saveArtifacts: nuxt config not found: ", fileName)
					continue
				}

				//note:
				//Nuxt config file is usually in the app directory, but not always
				//cust app path is defined with the "srcDir" field in the Nuxt config file
				nuxtAppDir := filepath.Dir(fileName)
				nuxtAppDirPrefix := fmt.Sprintf("%s/", nuxtAppDir)
				if p.cmd.IncludeAppNuxtDir {
					includePaths[nuxtAppDir] = true
					log.Tracef("saveArtifacts[nuxt] - including app dir - %s", nuxtAppDir)
				}

				if p.cmd.IncludeAppNuxtStaticDir {
					srcPath := filepath.Join(nuxtAppDir, nuxtStaticDir)
					if fsutil.DirExists(srcPath) {
						if p.cmd.IncludeAppNuxtDir && strings.HasPrefix(srcPath, nuxtAppDirPrefix) {
							log.Debugf("saveArtifacts[nuxt] - static dir is already included (%s)", srcPath)
						} else {
							includePaths[srcPath] = true
							log.Tracef("saveArtifacts[nuxt] - including static dir - %s", srcPath)
						}
					} else {
						log.Debugf("saveArtifacts[nuxt] - static dir does not exists (%s)", srcPath)
					}
				}

				if p.cmd.IncludeAppNuxtBuildDir && nuxtConfig.Build != "" {
					basePath := nuxtAppDir
					if strings.HasPrefix(nuxtConfig.Build, "/") {
						basePath = ""
					}

					srcPath := filepath.Join(basePath, nuxtConfig.Build)
					if fsutil.DirExists(srcPath) {
						if p.cmd.IncludeAppNuxtDir && strings.HasPrefix(srcPath, nuxtAppDirPrefix) {
							log.Debugf("saveArtifacts[nuxt] - build dir is already included (%s)", srcPath)
						} else {
							includePaths[srcPath] = true
							log.Tracef("saveArtifacts[nuxt] - including build dir - %s", srcPath)
						}
					} else {
						log.Debugf("saveArtifacts[nuxt] - build dir does not exists (%s)", srcPath)
					}
				}

				if p.cmd.IncludeAppNuxtDistDir && nuxtConfig.Dist != "" {
					basePath := nuxtAppDir
					if strings.HasPrefix(nuxtConfig.Dist, "/") {
						basePath = ""
					}

					srcPath := filepath.Join(basePath, nuxtConfig.Dist)
					if fsutil.DirExists(srcPath) {
						if p.cmd.IncludeAppNuxtDir && strings.HasPrefix(srcPath, nuxtAppDirPrefix) {
							log.Debugf("saveArtifacts[nuxt] - dist dir is already included (%s)", srcPath)
						} else {
							includePaths[srcPath] = true
							log.Tracef("saveArtifacts[nuxt] - including dist dir - %s", srcPath)
						}
					} else {
						log.Debugf("saveArtifacts[nuxt] - dist dir does not exists (%s)", srcPath)
					}
				}

				if p.cmd.IncludeAppNuxtNodeModulesDir {
					srcPath := filepath.Join(nuxtAppDir, nodePackageDirName)
					if fsutil.DirExists(srcPath) {
						if p.cmd.IncludeAppNuxtDir && strings.HasPrefix(srcPath, nuxtAppDirPrefix) {
							log.Debugf("saveArtifacts[nuxt] - node_modules dir is already included (%s)", srcPath)
						} else {
							includePaths[srcPath] = true
							log.Tracef("saveArtifacts[nuxt] - including node_modules dir - %s", srcPath)
						}
					} else {
						log.Debugf("saveArtifacts[nuxt] - node_modules dir does not exists (%s)", srcPath)
					}
				}

				continue
			}
		}

		if p.cmd.IncludeAppNextDir ||
			p.cmd.IncludeAppNextBuildDir ||
			p.cmd.IncludeAppNextDistDir ||
			p.cmd.IncludeAppNextStaticDir ||
			p.cmd.IncludeAppNextNodeModulesDir {
			if isNextConfigFile(fileName) {
				nextAppDir := filepath.Dir(fileName)
				nextAppDirPrefix := fmt.Sprintf("%s/", nextAppDir)
				if p.cmd.IncludeAppNextDir {
					includePaths[nextAppDir] = true
					log.Tracef("saveArtifacts[next] - including app dir - %s", nextAppDir)
				}

				if p.cmd.IncludeAppNextStaticDir {
					srcPath := filepath.Join(nextAppDir, nextStaticDir)
					if fsutil.DirExists(srcPath) {
						if p.cmd.IncludeAppNextDir && strings.HasPrefix(srcPath, nextAppDirPrefix) {
							log.Debugf("saveArtifacts[next] - static public dir is already included (%s)", srcPath)
						} else {
							includePaths[srcPath] = true
							log.Tracef("saveArtifacts[next] - including static public dir - %s", srcPath)
						}
					} else {
						log.Debugf("saveArtifacts[next] - static public dir does not exists (%s)", srcPath)
					}
				}

				if p.cmd.IncludeAppNextBuildDir {
					srcPath := filepath.Join(nextAppDir, nextDefaultBuildDir)
					if fsutil.DirExists(srcPath) {
						if p.cmd.IncludeAppNextDir && strings.HasPrefix(srcPath, nextAppDirPrefix) {
							log.Debugf("saveArtifacts[next] - build dir is already included (%s)", srcPath)
						} else {
							includePaths[srcPath] = true
							log.Tracef("saveArtifacts[next] - including build dir - %s", srcPath)
						}
					} else {
						log.Debugf("saveArtifacts[next] - build dir does not exists (%s)", srcPath)
					}
				}

				if p.cmd.IncludeAppNextDistDir {
					srcPath := filepath.Join(nextAppDir, nextDefaultStaticSpaDir)
					if fsutil.DirExists(srcPath) {
						if p.cmd.IncludeAppNextDir && strings.HasPrefix(srcPath, nextAppDirPrefix) {
							log.Debugf("saveArtifacts[next] - dist dir is already included (%s)", srcPath)
						} else {
							includePaths[srcPath] = true
							log.Tracef("saveArtifacts[next] - including dist dir - %s", srcPath)
						}
					} else {
						log.Debugf("saveArtifacts[next] - dist dir does not exists (%s)", srcPath)
					}
				}

				if p.cmd.IncludeAppNextNodeModulesDir {
					srcPath := filepath.Join(nextAppDir, nodePackageDirName)
					if fsutil.DirExists(srcPath) {
						if p.cmd.IncludeAppNextDir && strings.HasPrefix(srcPath, nextAppDirPrefix) {
							log.Debugf("saveArtifacts[next] - node_modules dir is already included (%s)", srcPath)
						} else {
							includePaths[srcPath] = true
							log.Tracef("saveArtifacts[next] - including node_modules dir - %s", srcPath)
						}
					} else {
						log.Debugf("saveArtifacts[next] - node_modules dir does not exists (%s)", srcPath)
					}
				}

				continue
			}
		}

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

			if len(p.cmd.IncludeNodePackages) > 0 {
				nodePackageInfo, err := getNodePackageFileData(fileName)
				if err == nil && nodePackageInfo != nil {
					for _, pkgName := range p.cmd.IncludeNodePackages {
						//note: use a better match lookup and include package version match later (":" as separator)
						if pkgName != "" && pkgName == nodePackageInfo.Name {
							nodeAppDir := filepath.Dir(fileName)
							includePaths[nodeAppDir] = true
							log.Tracef("saveArtifacts[node] - including app(%s) dir - %s", nodePackageInfo.Name, nodeAppDir)
							break
						}
					}
				} else {
					log.Warn("saveArtifacts - error getting node package config file => ", err)
				}
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

	log.Debugf("saveArtifacts[bsa] - copy files (%v)", len(p.saFileMap))
copyBsaFiles:
	for srcFileName := range p.saFileMap {
		for _, xpattern := range excludePatterns {
			found, err := doublestar.Match(xpattern, srcFileName)
			if err != nil {
				log.Warnf("saveArtifacts[bsa] - copy files - [%v] excludePatterns Match error - %v\n", srcFileName, err)
				//should only happen when the pattern is malformed
				continue
			}
			if found {
				log.Debugf("saveArtifacts[bsa] - copy files - [%v] - excluding (%s) ", srcFileName, xpattern)
				continue copyBsaFiles
			}
		}

		dstFilePath := fmt.Sprintf("%s/files%s", p.storeLocation, srcFileName)
		log.Debug("saveArtifacts[bsa] - saving file data => ", dstFilePath)
		if fsutil.Exists(dstFilePath) {
			//we might already have the target file
			//when we have intermediate symlinks in the path
			log.Debugf("saveArtifacts[bsa] - target file already exists (%s)", dstFilePath)
		} else {
			err := fsutil.CopyRegularFile(p.cmd.KeepPerms, srcFileName, dstFilePath, true)
			if err != nil {
				log.Warn("saveArtifacts[bsa] - error saving file => ", err)
			} else {
				log.Debugf("saveArtifacts[bsa] - saved file (%s)", dstFilePath)
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
		if fsutil.Exists(preservedDirPath) {
			filesDirPath := filepath.Join(p.storeLocation, filesDirName)
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

func (p *artifactStore) detectAppStack(fileName string) {
	isPython := detectPythonCodeFile(fileName)
	if isPython {
		appStack, ok := p.appStacks[certdiscover.LanguagePython]
		if !ok {
			appStack = &appStackInfo{
				language:    certdiscover.LanguagePython,
				packageDirs: map[string]struct{}{},
			}

			p.appStacks[certdiscover.LanguagePython] = appStack
		}

		appStack.codeFiles++
	}

	pyPkgDir := detectPythonPkgDir(fileName)
	if pyPkgDir != "" {
		appStack, ok := p.appStacks[certdiscover.LanguagePython]
		if !ok {
			appStack = &appStackInfo{
				language:    certdiscover.LanguagePython,
				packageDirs: map[string]struct{}{},
			}

			p.appStacks[certdiscover.LanguagePython] = appStack
		}

		appStack.packageDirs[pyPkgDir] = struct{}{}
	}

	if isPython || pyPkgDir != "" {
		return
	}

	isRuby := detectRubyCodeFile(fileName)
	if isRuby {
		appStack, ok := p.appStacks[certdiscover.LanguageRuby]
		if !ok {
			appStack = &appStackInfo{
				language:    certdiscover.LanguageRuby,
				packageDirs: map[string]struct{}{},
			}

			p.appStacks[certdiscover.LanguageRuby] = appStack
		}

		appStack.codeFiles++
	}

	rbPkgDir := detectRubyPkgDir(fileName)
	if rbPkgDir != "" {
		appStack, ok := p.appStacks[certdiscover.LanguageRuby]
		if !ok {
			appStack = &appStackInfo{
				language:    certdiscover.LanguageRuby,
				packageDirs: map[string]struct{}{},
			}

			p.appStacks[certdiscover.LanguageRuby] = appStack
		}

		appStack.packageDirs[rbPkgDir] = struct{}{}
	}

	if isRuby || rbPkgDir != "" {
		return
	}

	isNode := detectNodeCodeFile(fileName)
	if isNode {
		appStack, ok := p.appStacks[certdiscover.LanguageNode]
		if !ok {
			appStack = &appStackInfo{
				language:    certdiscover.LanguageNode,
				packageDirs: map[string]struct{}{},
			}

			p.appStacks[certdiscover.LanguageNode] = appStack
		}

		appStack.codeFiles++
	}

	nodePkgDir := detectNodePkgDir(fileName)
	if nodePkgDir != "" {
		appStack, ok := p.appStacks[certdiscover.LanguageNode]
		if !ok {
			appStack = &appStackInfo{
				language:    certdiscover.LanguageNode,
				packageDirs: map[string]struct{}{},
			}

			p.appStacks[certdiscover.LanguageNode] = appStack
		}

		appStack.packageDirs[nodePkgDir] = struct{}{}
	}
}

func isFileExt(filePath, match string) bool {
	fileExt := filepath.Ext(filePath)
	return fileExt == match
}

func getPathElementPrefix(filePath, match string) string {
	if !strings.Contains(filePath, match) {
		return ""
	}

	parts := strings.Split(filePath, match)
	if len(parts) > 0 {
		return parts[0]
	}

	return ""
}

func getPathElementPrefixLast(filePath, match string) string {
	if !strings.Contains(filePath, match) {
		return ""
	}

	if idx := strings.LastIndex(filePath, match); idx != -1 {
		return filePath[0:idx]
	}

	return ""
}

func detectPythonCodeFile(fileName string) bool {
	return isFileExt(fileName, pySrcFileExt)
}

func detectPythonPkgDir(fileName string) string {
	dpPrefix := getPathElementPrefix(fileName, pyDistPkgDir)
	if dpPrefix != "" {
		return fmt.Sprintf("%s%s", dpPrefix, pyDistPkgDir)
	}

	spPrefix := getPathElementPrefix(fileName, pySitePkgDir)
	if spPrefix != "" {
		return fmt.Sprintf("%s%s", spPrefix, pySitePkgDir)
	}

	return ""
}

func detectRubyCodeFile(fileName string) bool {
	return isFileExt(fileName, rbSrcFileExt)
}

func detectRubyPkgDir(fileName string) string {
	prefix := getPathElementPrefixLast(fileName, rbGemsSubDir)
	if prefix != "" {
		return fmt.Sprintf("%s%s", prefix, rbGemsSubDir)
	}

	return ""
}

func detectNodeCodeFile(fileName string) bool {
	return isFileExt(fileName, nodeSrcFileExt)
}

func detectNodePkgDir(fileName string) string {
	prefix := getPathElementPrefix(fileName, nodePackageDirPath)
	if prefix != "" {
		return fmt.Sprintf("%s%s", prefix, nodePackageDirPath)
	}

	return ""
}

func (p *artifactStore) archiveArtifacts() {
	src := filepath.Join(p.storeLocation, filesDirName)
	dst := filepath.Join(p.storeLocation, filesArchiveName)
	log.Debugf("artifactStore.archiveArtifacts: src='%s' dst='%s'", src, dst)

	trimPrefix := fmt.Sprintf("%s/", src)
	err := fsutil.ArchiveDir(dst, src, trimPrefix, "")
	errutil.FailOn(err)
}

// Go over all saved artifacts and update the name list to make
// sure all the files & folders are reflected in the final report.
// Hopefully, just a temporary workaround until a proper refactoring.
func (p *artifactStore) enumerateArtifacts() {
	knownFiles := list2map(p.nameList)

	var curpath string
	dirqueue := []string{filepath.Join(p.storeLocation, filesDirName)}

	for len(dirqueue) > 0 {
		curpath, dirqueue = dirqueue[0], dirqueue[1:]

		entries, err := os.ReadDir(curpath)
		if err != nil {
			log.WithError(err).Warn("artifactStore.enumerateArtifacts: readdir error")
			// Keep processing though since it might have been a partial result.
		}

		// Leaf element - empty dir.
		if len(entries) == 0 {
			if knownFiles[curpath] {
				continue
			}

			if props, err := artifactProps(curpath); err == nil {
				p.nameList = append(p.nameList, curpath)
				p.rawNames[curpath] = props
				knownFiles[curpath] = true
			} else {
				log.
					WithError(err).
					WithField("path", curpath).
					Warn("artifactStore.enumerateArtifacts: failed computing dir artifact props")
			}
			continue
		}

		for _, child := range entries {
			childpath := filepath.Join(curpath, child.Name())
			if child.IsDir() {
				dirqueue = append(dirqueue, childpath)
				continue
			}

			// Leaf element - regular file or symlink.
			if knownFiles[childpath] {
				continue
			}

			if props, err := artifactProps(childpath); err == nil {
				p.nameList = append(p.nameList, childpath)
				p.rawNames[childpath] = props
				knownFiles[childpath] = true
			} else {
				log.
					WithError(err).
					WithField("paht", childpath).
					Warn("artifactStore.enumerateArtifacts: failed computing artifact props")
			}
		}
	}
}

func (p *artifactStore) saveReport() {
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

	sort.Strings(p.nameList)
	for _, fname := range p.nameList {
		creport.Image.Files = append(creport.Image.Files, p.rawNames[fname])
	}

	reportName := report.DefaultContainerReportFileName

	_, err := os.Stat(p.storeLocation)
	if os.IsNotExist(err) {
		os.MkdirAll(p.storeLocation, 0777)
		_, err = os.Stat(p.storeLocation)
		errutil.FailOn(err)
	}

	reportFilePath := filepath.Join(p.storeLocation, reportName)
	log.Debugf("sensor: monitor - saving report to '%s'", reportFilePath)

	var reportData bytes.Buffer
	encoder := json.NewEncoder(&reportData)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(creport)
	errutil.FailOn(err)

	err = ioutil.WriteFile(reportFilePath, reportData.Bytes(), 0644)
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

type nuxtDirs struct {
	Build string
	Dist  string
}

func getNuxtConfig(path string) (*nuxtDirs, error) {
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		log.Debugf("sensor: monitor - getNuxtConfig - err stat => %s - %s", path, err.Error())
		return nil, fmt.Errorf("sensor: artifact - getNuxtConfig - error getting file => %s", path)
	}

	dat, err := ioutil.ReadFile(path)
	if err != nil {
		log.Debugf("sensor: monitor - getNuxtConfig - err reading file => %s - %s", path, err.Error())
		return nil, fmt.Errorf("sensor: artifact - getNuxtConfig - error reading file => %s", path)
	}

	log.Tracef("sensor: monitor - getNuxtConfig(%s) - %s", path, string(dat))

	nuxt := nuxtDirs{
		Build: nuxtDefaultBuildDir,
		Dist:  fmt.Sprintf("%s/%s", nuxtDefaultBuildDir, nuxtDefaultDistDir),
	}

	/*
		todo: need more test apps to verify this part of the code
		vm := otto.New()
		vm.Run(dat)

		if value, err := vm.Get(nuxtBuildDirKey); err == nil {
			if v, err := value.ToString(); err == nil {
				nuxt.Build = v
			} else {
				log.Debugf("saveArtifacts - using build default => %s", err.Error())
			}
		} else {
			log.Debug("saveArtifacts - error reading nuxt.config.js file => ", err.Error())
			return nil, fmt.Errorf("sensor: artifact - getNuxtConfig - error getting buildDir => %s", path)
		}

		if value, err := vm.Get(nuxtDistDirKey); err == nil {
			if v, err := value.ToString(); err == nil {
				nuxt.Dist = fmt.Sprintf("%s/%s", nuxt.Build, v)
			} else {
				log.Debugf("saveArtifacts - using dist default => %s", err.Error())
			}
		} else {
			log.Debug("saveArtifacts - reading nuxt.config.js file => ", err.Error())
			return nil, fmt.Errorf("sensor: artifact - getNuxtConfig - error getting distDir => %s", path)
		}
	*/

	return &nuxt, nil
}

func isNuxtConfigFile(filePath string) bool {
	fileName := filepath.Base(filePath)
	if fileName == nuxtConfigFile {
		return true
	}

	//TODO: read the file and verify that it's a real nuxt config file
	return false
}

/////

func isNextConfigFile(filePath string) bool {
	fileName := filepath.Base(filePath)
	if fileName == nextConfigFile || fileName == nextConfigFileAlt {
		return true
	}

	//TODO: read the file and verify that it's a real next config file
	return false
}

/////

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

func getNodePackageFileData(filePath string) (*NodePackageConfigSimple, error) {
	fileName := filepath.Base(filePath)
	if fileName != nodePackageFile {
		return nil, nil
	}

	var result NodePackageConfigSimple
	err := fsutil.LoadStructFromFile(filePath, &result)
	if err != nil {
		log.Warnf("sensor: getNodePackageFileData(%s) - error loading data => %v", filePath, err)
		return nil, err
	}

	return &result, nil
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

// TODO: Merge it with prepareArtifact().
func artifactProps(filename string) (*report.ArtifactProps, error) {
	fileInfo, err := os.Lstat(filename)
	if err != nil {
		return nil, err
	}

	fileType := report.UnknownArtifactType
	switch true {
	case fileInfo.Mode().IsRegular():
		fileType = report.FileArtifactType
	case (fileInfo.Mode() & os.ModeSymlink) != 0:
		fileType = report.SymlinkArtifactType
	case fileInfo.IsDir():
		fileType = report.DirArtifactType
	}

	return &report.ArtifactProps{
		FileType: fileType,
		FilePath: filename,
		Mode:     fileInfo.Mode(),
		ModeText: fileInfo.Mode().String(),
		FileSize: fileInfo.Size(),
	}, nil
}

func list2map(l []string) map[string]bool {
	m := map[string]bool{}
	for _, v := range l {
		m[v] = true
	}
	return m
}
