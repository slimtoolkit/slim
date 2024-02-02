//go:build linux
// +build linux

package artifact

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/sensor/detector/binfile"
	"github.com/slimtoolkit/slim/pkg/app/sensor/inspector/sodeps"
	"github.com/slimtoolkit/slim/pkg/artifact"
	"github.com/slimtoolkit/slim/pkg/certdiscover"
	"github.com/slimtoolkit/slim/pkg/ipc/command"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/sysidentity"
	"github.com/slimtoolkit/slim/pkg/system"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
)

const (
	pidFileSuffix    = ".pid"
	varRunDir        = "/var/run/"
	fileTypeCmdName  = "file"
	filesArchiveName = "files.tar"
	runArchiveName   = "run.tar"
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
	pyReqsFile           = "requirements.txt"
	pyPoetryProjectFile  = "pyproject.toml"
	pyPipEnvProjectFile  = "Pipfile"
	pyPipEnvLockFile     = "Pipfile.lock"
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

// later: each language pack will register its metadata files
var appMetadataFiles = map[string]struct{}{
	//python:
	pyReqsFile:          {},
	pyPoetryProjectFile: {},
	pyPipEnvProjectFile: {},
	pyPipEnvLockFile:    {},
	//ruby:
	rbGemfile:         {},
	rbGemfileLockFile: {},
	//node:
	nodePackageFile:       {},
	nodePackageLockFile:   {},
	nodeNpmShrinkwrapFile: {},
	nodeYarnLockFile:      {},
	nuxtConfigFile:        {},
	nextConfigFile:        {},
	nextConfigFileAlt:     {},
}

func isAppMetadataFile(filePath string) bool {
	target := filepath.Base(filePath)

	for name := range appMetadataFiles {
		if target == name {
			return true
		}
	}

	return false
}

var binDataReplace = []fsutil.ReplaceInfo{
	{
		PathSuffix: "/node",
		Match:      "node.js/v",
		Replace:    "done,xu/v",
	},
}

var appMetadataFileUpdate = map[string]fsutil.DataUpdaterFn{
	nodePackageFile: nodePackageJSONVerUpdater,
}

func appMetadataFileUpdater(filePath string) error {
	target := filepath.Base(filePath)

	updater, found := appMetadataFileUpdate[target]
	if !found {
		log.Tracef("appMetadataFileUpdater - no updater")
		return nil
	}

	return fsutil.UpdateFileData(filePath, updater, true)
}

func nodePackageJSONVerUpdater(target string, data []byte) ([]byte, error) {
	var info map[string]interface{}

	err := json.Unmarshal(data, &info)
	if err != nil {
		return nil, err
	}

	version, ok := info["version"].(string)
	if !ok {
		log.Tracef("nodePackageJSONVerUpdater - no version field, return as-is")
		return data, nil
	}

	version = fmt.Sprintf("1%s", version)
	log.Tracef("nodePackageJSONVerUpdater(%s) - version='%v'->'%v')\n", target, info["version"], version)
	info["version"] = version

	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("  ", "  ")
	err = enc.Encode(info)
	if err != nil {
		return nil, fmt.Errorf("error encoding updated package data")
	}

	return b.Bytes(), nil
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

// Needed mostly to be able to mock it in the sensor tests.
type Processor interface {
	// Current location of the artifacts folder.
	ArtifactsDir() string

	// Enumerate all files under a given root (used later on to tell the files
	// that were created during probing and the existed files appart).
	GetCurrentPaths(root string, excludes []string) (map[string]struct{}, error)

	// Create the artifacts folder, preserve some files, etc.
	PrepareEnv(cmd *command.StartMonitor) error

	// Dump the creport and the files to the artifacts folder.
	Process(
		cmd *command.StartMonitor,
		mountPoint string,
		peReport *report.PeMonitorReport,
		fanReport *report.FanMonitorReport,
		ptReport *report.PtMonitorReport,
	) error

	// Archives commands.json, creport.json, events.json, sensor.log, etc
	// to a tar ball.
	Archive() error
}

type processor struct {
	seReport         *report.SensorReport
	artifactsDirName string
	// Extra files to put into the artifacts archive before exiting.
	artifactsExtra []string
	origPathMap    map[string]struct{}
}

func NewProcessor(seReport *report.SensorReport, artifactsDirName string, artifactsExtra []string) Processor {
	return &processor{
		seReport:         seReport,
		artifactsDirName: artifactsDirName,
		artifactsExtra:   artifactsExtra,
	}
}

func (a *processor) ArtifactsDir() string {
	return a.artifactsDirName
}

func (a *processor) GetCurrentPaths(root string, excludes []string) (map[string]struct{}, error) {
	logger := log.WithField("op", "processor.GetCurrentPaths")
	logger.Trace("call")
	defer logger.Trace("exit")

	pathMap := map[string]struct{}{}
	err := filepath.Walk(root,
		func(pth string, info os.FileInfo, err error) error {
			if strings.HasPrefix(pth, "/proc/") {
				logger.Debugf("skipping /proc file system objects... - '%s'", pth)
				return filepath.SkipDir
			}

			if strings.HasPrefix(pth, "/sys/") {
				logger.Debugf("skipping /sys file system objects... - '%s'", pth)
				return filepath.SkipDir
			}

			if strings.HasPrefix(pth, "/dev/") {
				logger.Debugf("skipping /dev file system objects... - '%s'", pth)
				return filepath.SkipDir
			}

			// Optimization: Exclude folders early on to prevent slow enumerat
			//               Can help with mounting big folders from the host.
			// TODO: Combine this logic with the similar logic in findSymlinks().
			for _, xpattern := range excludes {
				if match, _ := doublestar.Match(xpattern, pth); match {
					if info.Mode().IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			if err != nil {
				logger.Debugf("skipping %s with error: %v", pth, err)
				return nil
			}

			if !(info.Mode().IsRegular() || (info.Mode()&os.ModeSymlink) != 0) {
				//need symlinks too
				return nil
			}

			pth, err = filepath.Abs(pth)
			if err != nil {
				return nil
			}

			if strings.HasPrefix(pth, "/proc/") ||
				strings.HasPrefix(pth, "/sys/") ||
				strings.HasPrefix(pth, "/dev/") {
				return nil
			}

			pathMap[pth] = struct{}{}
			return nil
		})

	if err != nil {
		return nil, err
	}

	a.origPathMap = pathMap
	return pathMap, nil
}

func (a *processor) PrepareEnv(cmd *command.StartMonitor) error {
	logger := log.WithField("op", "processor.PrepareEnv")
	logger.Trace("call")
	defer logger.Trace("exit")

	dstRootPath := filepath.Join(a.artifactsDirName, app.ArtifactFilesDirName)
	logger.Debugf("prep file artifacts root dir - '%s'", dstRootPath)
	if err := os.MkdirAll(dstRootPath, 0777); err != nil {
		return err
	}

	if cmd != nil && len(cmd.Preserves) > 0 {
		logger.Debugf("preserving paths - %d", len(cmd.Preserves))

		preservedDirPath := filepath.Join(a.artifactsDirName, preservedDirName)
		logger.Debugf("prep preserved artifacts root dir - '%s'", preservedDirPath)
		if err := os.MkdirAll(preservedDirPath, 0777); err != nil {
			return err
		}

		preservePaths := preparePaths(getKeys(cmd.Preserves))
		logger.Debugf("preservePaths(%v): %+v", len(preservePaths), preservePaths)

		newPerms := getRecordsWithPerms(cmd.Preserves)
		logger.Debugf("newPerms(%v): %+v", len(newPerms), newPerms)

		for inPath, isDir := range preservePaths {
			if artifact.IsFilteredPath(inPath) {
				logger.Debugf("skipping filtered path [isDir=%v] %s", isDir, inPath)
				continue
			}

			dstPath := fmt.Sprintf("%s%s", preservedDirPath, inPath)
			logger.Debugf("[isDir=%v] %s", isDir, dstPath)

			if isDir {
				err, errs := fsutil.CopyDir(cmd.KeepPerms, inPath, dstPath, true, true, nil, nil, nil)
				if err != nil {
					logger.Debugf("fsutil.CopyDir(%v,%v) error: %v", inPath, dstPath, err)
				}

				if len(errs) > 0 {
					logger.Debugf("fsutil.CopyDir(%v,%v) copy errors: %+v", inPath, dstPath, errs)
				}
			} else {
				if err := fsutil.CopyFile(cmd.KeepPerms, inPath, dstPath, true); err != nil {
					logger.Debugf("fsutil.CopyFile(%v,%v) error: %v", inPath, dstPath, err)
				}
			}
		}

		for inPath, perms := range newPerms {
			dstPath := fmt.Sprintf("%s%s", preservedDirPath, inPath)
			if fsutil.Exists(dstPath) {
				if err := fsutil.SetAccess(dstPath, perms); err != nil {
					logger.Debugf("fsutil.SetAccess(%v,%v) error: %v", dstPath, perms, err)
				}
			}
		}
	}

	return nil
}

func (a *processor) Process(
	cmd *command.StartMonitor,
	mountPoint string,
	peReport *report.PeMonitorReport,
	fanReport *report.FanMonitorReport,
	ptReport *report.PtMonitorReport,
) error {
	//TODO: when peReport is available filter file events from fanReport
	logger := log.WithField("op", "processor.Process")
	logger.Trace("call")
	defer logger.Trace("exit")

	logger.Debug("processing data...")

	fileCount := 0
	fileList := make([]string, 0, fileCount)
	for _, processFileMap := range fanReport.ProcessFiles {
		fileCount += len(processFileMap)
		for fpath := range processFileMap {
			fileList = append(fileList, fpath)
		}
	}

	logger.Debugf("len(fanReport.ProcessFiles)=%v / fileCount=%v", len(fanReport.ProcessFiles), fileCount)
	allFilesMap := findSymlinks(fileList, mountPoint, cmd.Excludes)
	return saveResults(a.origPathMap, a.artifactsDirName, cmd, allFilesMap, fanReport, ptReport, peReport, a.seReport)
}

func (a *processor) Archive() error {
	toArchive := map[string]struct{}{}
	for _, f := range a.artifactsExtra {
		if fsutil.Exists(f) {
			toArchive[f] = struct{}{}
		}
	}

	artifacts, err := os.ReadDir(a.artifactsDirName)
	if err != nil {
		return err
	}

	// We archive everything in the /opt/_slim/artifacts folder
	// except (potentially large data) `files` and `files.tar` entries.
	// and the monitor data event log
	// (which is used for local debugging or it should be streamed out of band)
	// In particular, this may include:
	//   - creport.json
	//   - events.json
	//   - app_stdout.log
	//   - app_stderr.log
	for _, f := range artifacts {
		if f.Name() != app.ArtifactFilesDirName &&
			f.Name() != filesArchiveName &&
			f.Name() != report.DefaultMonDelFileName {
			toArchive[filepath.Join(a.artifactsDirName, f.Name())] = struct{}{}
		}
	}

	var toArchiveList []string
	for name := range toArchive {
		toArchiveList = append(toArchiveList, name)
	}
	return fsutil.ArchiveFiles(
		filepath.Join(a.artifactsDirName, runArchiveName), toArchiveList, false, "")
}

func saveResults(
	origPathMap map[string]struct{},
	artifactsDirName string,
	cmd *command.StartMonitor,
	fileNames map[string]*report.ArtifactProps,
	fanMonReport *report.FanMonitorReport,
	ptMonReport *report.PtMonitorReport,
	peReport *report.PeMonitorReport,
	seReport *report.SensorReport,
) error {
	log.Debugf("saveResults(%v,...)", len(fileNames))

	artifactStore := newStore(origPathMap,
		artifactsDirName,
		fileNames,
		fanMonReport,
		ptMonReport,
		peReport,
		seReport,
		cmd)

	artifactStore.prepareArtifacts()
	artifactStore.saveArtifacts()
	artifactStore.enumerateArtifacts()
	//artifactStore.archiveArtifacts() //alternative way to xfer artifacts
	return artifactStore.saveReport()
}

// NOTE:
// the 'store' is supposed to only store/save/copy the artifacts we identified,
// but overtime a lot of artifact processing and post-processing logic
// ended up there too (which belongs in the artifact 'processor').
// TODO: refactor 'processor' and 'store' to have the right logic in the right places
type store struct {
	origPathMap   map[string]struct{}
	storeLocation string
	fanMonReport  *report.FanMonitorReport
	ptMonReport   *report.PtMonitorReport
	peMonReport   *report.PeMonitorReport
	seReport      *report.SensorReport
	rawNames      map[string]*report.ArtifactProps
	nameList      []string
	resolve       map[string]struct{}
	linkMap       map[string]*report.ArtifactProps
	fileMap       map[string]*report.ArtifactProps
	saFileMap     map[string]*report.ArtifactProps
	cmd           *command.StartMonitor
	appStacks     map[string]*appStackInfo
}

func newStore(
	origPathMap map[string]struct{},
	storeLocation string,
	rawNames map[string]*report.ArtifactProps,
	fanMonReport *report.FanMonitorReport,
	ptMonReport *report.PtMonitorReport,
	peMonReport *report.PeMonitorReport,
	seReport *report.SensorReport,
	cmd *command.StartMonitor) *store {
	store := &store{
		origPathMap:   origPathMap,
		storeLocation: storeLocation,
		fanMonReport:  fanMonReport,
		ptMonReport:   ptMonReport,
		peMonReport:   peMonReport,
		seReport:      seReport,
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

func (p *store) getArtifactFlags(artifactFileName string) map[string]bool {
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

func (p *store) prepareArtifact(artifactFileName string) {
	srcLinkFileInfo, err := os.Lstat(artifactFileName)
	if err != nil {
		log.Debugf("prepareArtifact - artifact don't exist: %v (%v)", artifactFileName, os.IsNotExist(err))
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

	log.Tracef("prepareArtifact - file mode:%v", srcLinkFileInfo.Mode())
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
			log.Debugf("prepareArtifact - error getting reference for symlink (%v) -> %v", err, artifactFileName)
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
				log.Debugf("prepareArtifact - error getting absolute path for symlink ref (%v) -> %v => %v", err, artifactFileName, fullLinkRef)
			}
		} else {
			absLinkRef, err = filepath.Abs(linkRef)
			if err != nil {
				log.Debugf("prepareArtifact - error getting absolute path for symlink ref 2 (%v) -> %v => %v", err, artifactFileName, linkRef)
			}
		}

		if absLinkRef != "" {
			evalLinkRef, err := filepath.EvalSymlinks(absLinkRef)
			if err != nil {
				log.Debugf("prepareArtifact - error evaluating symlink (%v) -> %v => %v", err, artifactFileName, absLinkRef)
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
		log.Debugf("prepareArtifact - is a directory (shouldn't see it) - %v", artifactFileName)
		props.FileType = report.DirArtifactType
		p.rawNames[artifactFileName] = props
	default:
		log.Debugf("prepareArtifact - other type (shouldn't see it) - %v", artifactFileName)
		p.rawNames[artifactFileName] = props
	}
}

func (p *store) prepareArtifacts() {
	log.Debugf("p.prepareArtifacts() p.rawNames=%v", len(p.rawNames))

	for artifactFileName := range p.rawNames {
		log.Debugf("prepareArtifacts - artifact => %v", artifactFileName)
		p.prepareArtifact(artifactFileName)
	}

	if p.ptMonReport.Enabled {
		log.Debug("prepareArtifacts - ptMonReport.Enabled")
		for artifactFileName, fsaInfo := range p.ptMonReport.FSActivity {
			artifactInfo, found := p.rawNames[artifactFileName]
			if found && artifactInfo != nil {
				artifactInfo.FSActivity = fsaInfo
			} else {
				log.Debugf("prepareArtifacts [%v] - fsa artifact => %v", found, artifactFileName)
				if found && artifactInfo == nil {
					log.Debugf("prepareArtifacts - fsa artifact (found, but no info) => %v", artifactFileName)
				}
				p.prepareArtifact(artifactFileName)
				artifactInfo, found := p.rawNames[artifactFileName]
				if found && artifactInfo != nil {
					artifactInfo.FSActivity = fsaInfo
				} else {
					log.Debugf("[warn] prepareArtifacts - fsa artifact - missing in rawNames => %v", artifactFileName)
				}

				//TMP:
				//fsa might include directories, which we'll need to copy (dir only)
				//but p.prepareArtifact() doesn't do anything with dirs for now
			}
		}
	}

	for artifactFileName := range p.fileMap {
		//TODO: conditionally detect binary files and their deps
		if binProps, _ := binfile.Detected(artifactFileName); binProps == nil || !binProps.IsBin {
			continue
		}

		binArtifacts, err := sodeps.AllDependencies(artifactFileName)
		if err != nil {
			if err == sodeps.ErrDepResolverNotFound {
				log.Debug("prepareArtifacts.binArtifacts[bsa] - no static bin dep resolver")
			} else {
				log.Debugf("prepareArtifacts.binArtifacts[bsa] - %v - error getting bin artifacts => %v\n", artifactFileName, err)
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
				log.Debugf("prepareArtifacts.binArtifacts[bsa] - artifact doesn't exist: %v (%v)", bpath, os.IsNotExist(err))
				continue
			}

			bprops := &report.ArtifactProps{
				FilePath: bpath,
				Mode:     bpathFileInfo.Mode(),
				ModeText: bpathFileInfo.Mode().String(),
				FileSize: bpathFileInfo.Size(),
			}

			bprops.Flags = p.getArtifactFlags(bpath)

			fsType := report.UnknownArtifactTypeName
			switch {
			case bpathFileInfo.Mode().IsRegular():
				fsType = report.FileArtifactTypeName
				p.rawNames[bpath] = bprops
				//use a separate file map, so we can save them last
				//in case we are dealing with intermediate symlinks
				//and to better track what bin deps are not covered by dynamic analysis
				p.saFileMap[bpath] = bprops
			case (bpathFileInfo.Mode() & os.ModeSymlink) != 0:
				fsType = report.SymlinkArtifactTypeName
				p.linkMap[bpath] = bprops
				p.rawNames[bpath] = bprops
			default:
				fsType = report.UnexpectedArtifactTypeName
				log.Debugf("prepareArtifacts.binArtifacts[bsa] - unexpected ft - %s", bpath)
			}

			log.Debugf("prepareArtifacts.binArtifacts[bsa] - bin artifact (%s) fsType=%s [%d]bdep=%s", artifactFileName, fsType, idx, bpath)
		}
	}

	p.resolveLinks()
}

func (p *store) resolveLinks() {
	//note:
	//the links should be resolved in findSymlinks, but
	//the current design needs to be improved to catch all symlinks
	//this is a backup to catch the root level symlinks
	files, err := os.ReadDir("/")
	if err != nil {
		log.Debug("resolveLinks - os.ReadDir error: ", err)
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
			log.Debugf("resolveLinks.files - os.Lstat(%s) error: %v", fpath, err)
			continue
		}

		if fileInfo.Mode()&os.ModeSymlink == 0 {
			log.Debug("resolveLinks.files - skipping non-symlink")
			continue
		}

		linkRef, err := os.Readlink(fpath)
		if err != nil {
			log.Debugf("resolveLinks.files - os.Readlink(%s) error: %v", fpath, err)
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
				log.Debugf("resolveLinks.files - error getting absolute path for symlink ref (1) (%v) -> %v => %v", err, fpath, fullLinkRef)
				continue
			}
		} else {
			var err error
			absLinkRef, err = filepath.Abs(linkRef)
			if err != nil {
				log.Debugf("resolveLinks.files - error getting absolute path for symlink ref (2) (%v) -> %v => %v", err, fpath, linkRef)
				continue
			}
		}

		//todo: skip "/proc/..." references
		evalLinkRef, err := filepath.EvalSymlinks(absLinkRef)
		if err != nil {
			log.Debugf("resolveLinks.files - error evaluating symlink (%v) -> %v => %v", err, fpath, absLinkRef)
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
			log.WithError(err).Debug("preparePaths(): skipping path = ", pathValue)
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

func (p *store) saveWorkdir(excludePatterns []string) {
	if p.cmd.IncludeWorkdir == "" {
		return
	}

	if artifact.IsFilteredPath(p.cmd.IncludeWorkdir) {
		log.Debug("sensor.store.saveWorkdir(): skipping filtered workdir")
		return
	}

	if !fsutil.DirExists(p.cmd.IncludeWorkdir) {
		log.Debugf("sensor.store.saveWorkdir: workdir does not exist %s", p.cmd.IncludeWorkdir)
		return
	}

	dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, p.cmd.IncludeWorkdir)
	if fsutil.Exists(dstPath) {
		log.Debug("sensor.store.saveWorkdir: workdir dst path already exists")
		//it's possible that some of the files in the work dir are already copied
		//the copy logic will improve when we copy the files separately
		//for now just copy the whole workdir
	}

	log.Debugf("sensor.store.saveWorkdir: workdir=%s", p.cmd.IncludeWorkdir)

	err, errs := fsutil.CopyDir(p.cmd.KeepPerms, p.cmd.IncludeWorkdir, dstPath, true, true, excludePatterns, nil, nil)
	if err != nil {
		log.Debugf("sensor.store.saveWorkdir: CopyDir(%v,%v) error: %v", p.cmd.IncludeWorkdir, dstPath, err)
	}

	if len(errs) > 0 {
		log.Debugf("sensor.store.saveWorkdir: CopyDir(%v,%v) copy errors: %+v", p.cmd.IncludeWorkdir, dstPath, errs)
	}

	//todo:
	//copy files separately and
	//apply 'workdir-exclude' patterns in addition to the global excludes (excludePatterns)
	//resolve symlinks
}

/////////////////////////////////////////////////////////

const (
	ziDirOne    = "/usr/lib/zoneinfo"
	ziDirTwo    = "/usr/share/zoneinfo"
	ziDirThree  = "/usr/share/zoneinfo-icu"
	ziEnv       = "TZDIR" //TODO: lookup zoneinfo data path from TZDIR
	ziTimezone  = "/etc/timezone"
	ziLocaltime = "/etc/localtime"
)

var ziDirs = []string{
	ziDirOne,
	ziDirTwo,
	ziDirThree,
}

var ziFiles = []string{
	ziTimezone,
	ziLocaltime,
}

func (p *store) saveZoneInfo() {
	if !p.cmd.IncludeZoneInfo {
		return
	}

	log.Trace("sensor.store.saveZoneInfo")
	for _, fp := range ziFiles {
		if !fsutil.Exists(fp) {
			log.Debugf("sensor.store.saveZoneInfo: no target file '%s' (skipping...)", fp)
			continue
		}

		log.Tracef("sensor.store.saveZoneInfo: copy %s", fp)
		dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, fp)
		if fsutil.Exists(dstPath) {
			log.Debugf("sensor.store.saveZoneInfo: already copied target file '%s' (skipping...)", dstPath)
			continue
		}

		if err := fsutil.CopyFile(p.cmd.KeepPerms, fp, dstPath, true); err != nil {
			log.Debugf("sensor.store.saveZoneInfo: fsutil.CopyFile(%v,%v) error - %v", fp, dstPath, err)
		}
	}

	for _, dp := range ziDirs {
		if !fsutil.DirExists(dp) {
			log.Debugf("sensor.store.saveZoneInfo: no target directory '%s' (skipping...)", dp)
			continue
		}

		log.Tracef("sensor.store.saveZoneInfo: copy dir %s", dp)
		dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, dp)

		err, errs := fsutil.CopyDir(p.cmd.KeepPerms, dp, dstPath, true, true, nil, nil, nil)
		if err != nil {
			log.Debugf("sensor.store.saveZoneInfo: fsutil.CopyDir(%s,%s) error: %v", dp, dstPath, err)
		}

		if len(errs) > 0 {
			log.Debugf("sensor.store.saveZoneInfo: fsutil.CopyDir(%v,%v) copy errors: %+v", dp, dstPath, errs)
		}
	}
}

/////////////////////////////////////////////////////////

const (
	sshUserSSHDir    = ".ssh"
	sshUserSSHDirPat = "/.ssh/"
	sshEtc           = "/etc/ssh"
	sshLibOpenSSH    = "/usr/lib/openssh"
	sshDefaultExeDir = "/usr/bin"

	sshExeName        = "ssh"
	sshAddExeName     = "ssh-add"
	sshAgentExeName   = "ssh-agent"
	sshKeygenExeName  = "ssh-keygen"
	sshKeyscanExeName = "ssh-keyscan"
	sshArgv0ExeName   = "ssh-argv0"
	sshCopyIDExeName  = "ssh-copy-id"
)

var sshConfigDirs = []string{
	sshEtc,
}

var sshBinDirs = []string{
	sshLibOpenSSH,
}

var sshExeNames = []string{
	sshExeName,
	sshAddExeName,
	sshAgentExeName,
	sshKeygenExeName,
	sshKeyscanExeName,
	sshArgv0ExeName,
	sshCopyIDExeName,
}

func homeDirs() []string {
	dirMap := map[string]struct{}{}
	var done bool
	if fsutil.Exists(sysidentity.PasswdFilePath) {
		info, err := sysidentity.ReadPasswdFile(sysidentity.PasswdFilePath)
		if err != nil {
			log.Debugf("sensor.store.homeDirs: error processing passwd: %v", err)
		} else {
			for _, pr := range info.Records {
				if pr.NoLoginShell || pr.Home == "" {
					continue
				}

				dirMap[pr.Home] = struct{}{}
			}

			done = true
		}
	}

	if !done {
		// hacky way to get the home directories for users...
		rootDir := "/root"
		if !fsutil.DirExists(rootDir) {
			dirMap[rootDir] = struct{}{}
		}

		homeBaseDir := "/home"
		hdFiles, err := os.ReadDir(homeBaseDir)
		if err == nil {
			for _, file := range hdFiles {
				fullPath := filepath.Join(homeBaseDir, file.Name())
				if fsutil.IsDir(fullPath) {
					dirMap[fullPath] = struct{}{}
				}
			}
		} else {
			log.Debugf("sensor.store.homeDirs: error enumerating %s: %v", homeBaseDir, err)
		}
	}

	var dirList []string
	for dp := range dirMap {
		dirList = append(dirList, dp)
	}

	return dirList
}

func (ref *store) saveSSHClient() {
	if !ref.cmd.IncludeSSHClient {
		return
	}

	log.Trace("sensor.store.saveSSHClient")
	configDirs := append([]string{}, sshConfigDirs...)

	// copy user config dirs
	for _, dir := range homeDirs() {
		dp := filepath.Join(dir, sshUserSSHDir)
		if !fsutil.DirExists(dp) {
			continue
		}

		configDirs = append(configDirs, dp)
	}

	// copy config dirs
	for _, dp := range configDirs {
		if !fsutil.DirExists(dp) {
			log.Debugf("sensor.store.saveSSHClient: no target directory '%s' (skipping...)", dp)
			continue
		}

		log.Tracef("sensor.store.saveSSHClient: copy dir %s", dp)
		dstPath := fmt.Sprintf("%s/files%s", ref.storeLocation, dp)

		err, errs := fsutil.CopyDir(ref.cmd.KeepPerms, dp, dstPath, true, true, nil, nil, nil)
		if err != nil {
			log.Debugf("sensor.store.saveSSHClient: fsutil.CopyDir(%s,%s) error: %v", dp, dstPath, err)
		}

		if len(errs) > 0 {
			log.Debugf("sensor.store.saveSSHClient: fsutil.CopyDir(%v,%v) copy errors: %+v", dp, dstPath, errs)
		}
	}

	// locate/resolve exes to full bin paths
	allDepsMap := map[string]struct{}{}
	for _, name := range sshExeNames {
		exePath, err := exec.LookPath(name)
		if err != nil {
			log.Debugf("sensor.store.saveSSHClient - checking '%s' exe (not found: %s)", name, err)
			exePath = filepath.Join(sshDefaultExeDir, name)
		}

		if !fsutil.Exists(exePath) {
			log.Debugf("sensor.store.saveSSHClient - exe bin file not found - '%s' (skipping)", exePath)
			continue
		}

		artifacts, err := sodeps.AllDependencies(exePath)
		if err != nil {
			log.Debugf("sensor.store.saveSSHClient - %s - error getting bin artifacts => %v", exePath, err)
			// still add the bin path itself even if we had problems locating its deps
			allDepsMap[exePath] = struct{}{}
			continue
		}

		// artifacts includes exePath
		for _, an := range artifacts {
			allDepsMap[an] = struct{}{}
		}
	}

	// copy bin dirs and identify bin deps
	for _, dp := range sshBinDirs {
		if !fsutil.DirExists(dp) {
			log.Debugf("sensor.store.saveSSHClient: no target directory '%s' (skipping...)", dp)
			continue
		}

		log.Tracef("sensor.store.saveSSHClient: copy dir %s", dp)
		dstPath := fmt.Sprintf("%s/files%s", ref.storeLocation, dp)

		err, errs := fsutil.CopyDir(ref.cmd.KeepPerms, dp, dstPath, true, true, nil, nil, nil)
		if err != nil {
			log.Debugf("sensor.store.saveSSHClient: fsutil.CopyDir(%s,%s) error: %v", dp, dstPath, err)
		}

		if len(errs) > 0 {
			log.Debugf("sensor.store.saveSSHClient: fsutil.CopyDir(%v,%v) copy errors: %+v", dp, dstPath, errs)
		}

		dirFiles := map[string]struct{}{}
		err = filepath.Walk(dp,
			func(p string, info os.FileInfo, err error) error {
				if err != nil {
					log.Debugf("sensor.store.saveSSHClient: [bin dir path - %s] skipping %s with error: %v", dp, p, err)
					return nil
				}

				p, err = filepath.Abs(p)
				if err != nil {
					return nil
				}

				dirFiles[p] = struct{}{}
				return nil
			})

		if err != nil {
			log.Debugf("sensor.store.saveSSHClient: error enumerating %s: %v", dp, err)
		}

		for fp := range dirFiles {
			if !fsutil.Exists(fp) {
				log.Debugf("sensor.store.saveSSHClient - bin dir (%s) file not found - '%s' (skipping)", dp, fp)
				continue
			}

			if binProps, _ := binfile.Detected(fp); binProps != nil && binProps.IsBin {
				binArtifacts, err := sodeps.AllDependencies(fp)
				if err != nil {
					// still add the bin path itself even if we had problems locating its deps
					allDepsMap[fp] = struct{}{}
					continue
				}

				for _, bpath := range binArtifacts {
					bfpaths, err := resloveLink(bpath)
					if err != nil {
						log.Debugf("sensor.store.saveSSHClient: error resolving link - %s (%v)", bpath, err)
						// still add the path...
						allDepsMap[bpath] = struct{}{}
						continue
					}

					for _, bfp := range bfpaths {
						if bfp == "" {
							continue
						}

						if !fsutil.Exists(bfp) {
							continue
						}

						allDepsMap[bfp] = struct{}{}
					}
				}
			} else {
				allDepsMap[fp] = struct{}{}
			}
		}
	}

	// copy bin files and their deps
	log.Tracef("sensor.store.saveSSHClient: - paths.len(%d) = %+v", len(allDepsMap), allDepsMap)
	for fp := range allDepsMap {
		if !fsutil.Exists(fp) {
			continue
		}

		dstPath := fmt.Sprintf("%s/files%s", ref.storeLocation, fp)
		if fsutil.Exists(dstPath) {
			continue
		}

		if err := fsutil.CopyFile(ref.cmd.KeepPerms, fp, dstPath, true); err != nil {
			log.Debugf("sensor.store.saveSSHClient: fsutil.CopyFile(%v,%v) error - %v", fp, dstPath, err)
		}
	}
}

const (
	osLibDir          = "/lib/"
	osUsrLibDir       = "/usr/lib/"
	osUsrLib64Dir     = "/usr/lib64/"
	osLibNssDns       = "/libnss_dns"
	osLibNssResolv    = "/libresolv"
	osLibNssFiles     = "/libnss_files"
	osLibSO           = ".so"
	osLibResolveConf  = "/etc/resolv.conf"
	osLibNsswitchConf = "/etc/nsswitch.conf"
	osLibHostConf     = "/etc/host.conf"
)

var osLibsNetFiles = []string{
	osLibResolveConf,
	osLibNsswitchConf,
	osLibHostConf,
}

func (p *store) saveOSLibsNetwork() {
	if !p.cmd.IncludeOSLibsNet {
		return
	}

	log.Trace("sensor.store.saveOSLibsNetwork")
	for _, fp := range osLibsNetFiles {
		if !fsutil.Exists(fp) {
			continue
		}

		log.Debugf("sensor.store.saveOSLibsNetwork: copy %s", fp)
		dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, fp)
		if fsutil.Exists(dstPath) {
			continue
		}

		if err := fsutil.CopyFile(p.cmd.KeepPerms, fp, dstPath, true); err != nil {
			log.Debugf("sensor.store.saveOSLibsNetwork: fsutil.CopyFile(%v,%v) error - %v", fp, dstPath, err)
		}
	}

	if len(p.origPathMap) == 0 {
		log.Debug("sensor.store.saveOSLibsNetwork: no origPathMap")
		return
	}

	pathMap := map[string]struct{}{}
	for fileName := range p.origPathMap {
		if (strings.Contains(fileName, osLibNssDns) ||
			strings.Contains(fileName, osLibNssResolv) ||
			strings.Contains(fileName, osLibNssFiles)) &&
			(strings.Contains(fileName, osLibDir) ||
				strings.Contains(fileName, osUsrLibDir) ||
				strings.Contains(fileName, osUsrLib64Dir)) &&
			strings.Contains(fileName, osLibSO) {
			log.Debugf("sensor.store.saveOSLibsNetwork: match - %s", fileName)
			pathMap[fileName] = struct{}{}
		}
	}

	allPathMap := map[string]struct{}{}
	for fpath := range pathMap {
		if !fsutil.Exists(fpath) {
			continue
		}

		fpaths, err := resloveLink(fpath)
		if err != nil {
			log.Debugf("sensor.store.saveOSLibsNetwork: error resolving link - %s", fpath)
			continue
		}

		fpaths = append(fpaths, fpath)
		for _, fp := range fpaths {
			if fp == "" {
				continue
			}

			if !fsutil.Exists(fp) {
				continue
			}

			allPathMap[fp] = struct{}{}
			if binProps, _ := binfile.Detected(fp); binProps != nil && binProps.IsBin {
				binArtifacts, err := sodeps.AllDependencies(fp)
				if err != nil {
					if err == sodeps.ErrDepResolverNotFound {
						log.Debug("sensor.store.saveOSLibsNetwork[bsa] - no static bin dep resolver")
					} else {
						log.Debugf("sensor.store.saveOSLibsNetwork[bsa] - %v - error getting bin artifacts => %v\n", fp, err)
					}
					continue
				}

				for _, bpath := range binArtifacts {
					bfpaths, err := resloveLink(bpath)
					if err != nil {
						log.Debugf("sensor.store.saveOSLibsNetwork: error resolving link - %s", bpath)
						continue
					}

					for _, bfp := range bfpaths {
						if bfp == "" {
							continue
						}

						if !fsutil.Exists(bfp) {
							continue
						}
						allPathMap[bfp] = struct{}{}
					}
				}
			}
		}
	}

	log.Debugf("sensor.store.saveOSLibsNetwork: - allPathMap(%v) = %+v", len(allPathMap), allPathMap)
	for fp := range allPathMap {
		if !fsutil.Exists(fp) {
			continue
		}

		dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, fp)
		if fsutil.Exists(dstPath) {
			continue
		}

		if err := fsutil.CopyFile(p.cmd.KeepPerms, fp, dstPath, true); err != nil {
			log.Debugf("sensor.store.saveOSLibsNetwork: fsutil.CopyFile(%v,%v) error - %v", fp, dstPath, err)
		}
	}
}

func resloveLink(fpath string) ([]string, error) {
	finfo, err := os.Lstat(fpath)
	if err != nil {
		return nil, err
	}

	if finfo.Mode()&os.ModeSymlink == 0 {
		return nil, nil
	}

	linkRef, err := os.Readlink(fpath)
	if err != nil {
		return nil, err
	}

	var out []string
	var target string
	if filepath.IsAbs(linkRef) {
		target = linkRef
	} else {
		linkDir := filepath.Dir(fpath)
		fullLinkRef := filepath.Clean(filepath.Join(linkDir, linkRef))
		if fullLinkRef != "." {
			target = fullLinkRef
		}
	}

	if target != "" {
		out = append(out, target)
		if evalLinkRef, err := filepath.EvalSymlinks(target); err == nil {
			if evalLinkRef != target {
				out = append(out, evalLinkRef)
			}
		}
	}

	return out, nil
}

func (p *store) saveCertsData() {
	copyCertFiles := func(list []string) {
		log.Debugf("sensor.store.saveCertsData.copyCertFiles(list=%+v)", list)
		for _, fname := range list {
			if fsutil.Exists(fname) {
				dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, fname)
				if err := fsutil.CopyFile(p.cmd.KeepPerms, fname, dstPath, true); err != nil {
					log.Debugf("sensor.store.saveCertsData.copyCertFiles: fsutil.CopyFile(%v,%v) error - %v", fname, dstPath, err)
				}
			}
		}
	}

	copyDirs := func(list []string, copyLinkTargets bool) {
		log.Debugf("sensor.store.saveCertsData.copyDirs(list=%+v,copyLinkTargets=%v)", list, copyLinkTargets)
		for _, fname := range list {
			if fsutil.Exists(fname) {
				dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, fname)

				if fsutil.IsDir(fname) {
					err, errs := fsutil.CopyDir(p.cmd.KeepPerms, fname, dstPath, true, true, nil, nil, nil)
					if err != nil {
						log.Debugf("sensor.store.saveCertsData.copyDirs: fsutil.CopyDir(%v,%v) error: %v", fname, dstPath, err)
					} else if copyLinkTargets {
						foList, err := os.ReadDir(fname)
						if err == nil {
							log.Debugf("sensor.store.saveCertsData.copyDirs(): dir=%v fcount=%v", fname, len(foList))
							for _, fo := range foList {
								fullPath := filepath.Join(fname, fo.Name())
								log.Debugf("sensor.store.saveCertsData.copyDirs(): dir=%v fullPath=%v", fname, fullPath)
								if fsutil.IsSymlink(fullPath) {
									linkRef, err := os.Readlink(fullPath)
									if err != nil {
										log.Debugf("sensor.store.saveCertsData.copyDirs: os.Readlink(%v) error - %v", fullPath, err)
										continue
									}

									log.Debugf("sensor.store.saveCertsData.copyDirs(): dir=%v fullPath=%v linkRef=%v",
										fname, fullPath, linkRef)
									if strings.Contains(linkRef, "/") {
										targetFilePath := linkTargetToFullPath(fullPath, linkRef)
										if targetFilePath != "" && fsutil.Exists(targetFilePath) {
											log.Debugf("sensor.store.saveCertsData.copyDirs(): dir=%v fullPath=%v linkRef=%v targetFilePath=%v",
												fname, fullPath, linkRef, targetFilePath)
											dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, targetFilePath)
											if err := fsutil.CopyFile(p.cmd.KeepPerms, targetFilePath, dstPath, true); err != nil {
												log.Debugf("sensor.store.saveCertsData.copyDirs: fsutil.CopyFile(%v,%v) error - %v", targetFilePath, dstPath, err)
											}
										} else {
											log.Debugf("sensor.store.saveCertsData.copyDirs: targetFilePath does not exist - %v", targetFilePath)
										}
									}
								}
							}
						} else {
							log.Debugf("sensor.store.saveCertsData.copyDirs: os.ReadDir(%v) error - %v", fname, err)
						}
					}

					if len(errs) > 0 {
						log.Debugf("sensor.store.saveCertsData.copyDirs: fsutil.CopyDir(%v,%v) copy errors: %+v", fname, dstPath, errs)
					}
				} else if fsutil.IsSymlink(fname) {
					if err := fsutil.CopySymlinkFile(p.cmd.KeepPerms, fname, dstPath, true); err != nil {
						log.Debugf("sensor.store.saveCertsData.copyDirs: fsutil.CopySymlinkFile(%v,%v) error - %v", fname, dstPath, err)
					}
				} else {
					log.Debugf("store.saveCertsData.copyDir: unexpected obect type - %s", fname)
				}
			}
		}
	}

	copyAppCertFiles := func(suffix string, dirs []string, subdirPrefix string) {
		//NOTE: dirs end with "/" (need to revisit the formatting to make it consistent)
		log.Debugf("sensor.store.saveCertsData.copyAppCertFiles(suffix=%v,dirs=%+v,subdirPrefix=%v)",
			suffix, dirs, subdirPrefix)
		for _, dirName := range dirs {
			if subdirPrefix != "" {
				foList, err := os.ReadDir(dirName)
				if err != nil {
					log.Debugf("sensor.store.saveCertsData.copyAppCertFiles: os.ReadDir(%v) error - %v", dirName, err)
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
					log.Debugf("sensor.store.saveCertsData.copyAppCertFiles: fsutil.CopyFile(%v,%v) error - %v", srcFilePath, dstPath, err)
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

func (p *store) saveArtifacts() {
	var includePaths map[string]bool
	var includeDirBinsList map[string]bool
	var newPerms map[string]*fsutil.AccessInfo

	syscall.Umask(0)

	excludePatterns := p.cmd.Excludes
	excludePatterns = append(excludePatterns, "/opt/_slim")
	excludePatterns = append(excludePatterns, "/opt/_slim/**")
	if p.cmd.ExcludeVarLockFiles {
		excludePatterns = append(excludePatterns, "/var/lock/**")
		excludePatterns = append(excludePatterns, "/run/lock/**")
	}

	log.Debugf("saveArtifacts - excludePatterns(%v): %+v", len(excludePatterns), excludePatterns)

	includePaths = preparePaths(getKeys(p.cmd.Includes))
	log.Debugf("saveArtifacts - includePaths(%v): %+v", len(includePaths), includePaths)

	if includePaths == nil {
		includePaths = map[string]bool{}
	}

	includeDirBinsList = preparePaths(getKeys(p.cmd.IncludeDirBinsList))
	log.Debugf("saveArtifacts - includeDirBinsList(%d): %+v", len(includeDirBinsList), includeDirBinsList)
	if includeDirBinsList == nil {
		includeDirBinsList = map[string]bool{}
	}

	newPerms = getRecordsWithPerms(p.cmd.Includes)
	log.Debugf("saveArtifacts - newPerms(%v): %+v", len(newPerms), newPerms)

	for pk, pv := range p.cmd.Perms {
		newPerms[pk] = pv
	}
	log.Debugf("saveArtifacts - merged newPerms(%v): %+v", len(newPerms), newPerms)

	//moved to prepareEnv
	//dstRootPath := filepath.Join(p.storeLocation, app.ArtifactFilesDirName)
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
			log.Debugf("saveArtifacts.symlinkWalk: could not convert data - %s\n", linkName)
			return false
		}

		for _, xpattern := range excludePatterns {
			found, err := doublestar.Match(xpattern, linkName)
			if err != nil {
				log.Debugf("saveArtifacts.symlinkWalk - copy links - [%v] excludePatterns Match error - %v\n", linkName, err)
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
			log.Debugf("saveArtifacts.symlinkWalk - dir error (linkName=%s linkDir=%s linkPath=%s) => error=%v", linkName, linkDir, linkPath, err)
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
				log.Debugf("saveArtifacts.symlinkWalk - symlink create error: %v", err)
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
			log.Debugf("saveArtifacts.symlinkFailed - dir error (linkName=%s linkDir=%s linkPath=%s) => error=%v", linkName, linkDir, linkPath, err)
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
				log.Debugf("saveArtifacts.symlinkFailed - symlink create error ==> %v", err)
			}
		}
	}

	//NOTE: need to copy the files after the links are copied
	log.Debugf("saveArtifacts - copy files (%v) and copy additional files checked at runtime...", len(p.fileMap))
	ngxEnsured := false

copyFiles:
	for srcFileName, artifactInfo := range p.fileMap {
		//need to make sure we don't filter out something we need
		if artifact.IsFilteredPath(srcFileName) {
			log.Debugf("saveArtifacts - skipping filtered copy file - %s", srcFileName)
			continue
		}

		for _, xpattern := range excludePatterns {
			found, err := doublestar.Match(xpattern, srcFileName)
			if err != nil {
				log.Debugf("saveArtifacts - copy files - [%v] excludePatterns Match error - %v\n", srcFileName, err)
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

		filePath := fmt.Sprintf("%s/files%s", p.storeLocation, srcFileName)
		log.Debug("saveArtifacts - saving file data => ", filePath)

		if artifactInfo != nil &&
			artifactInfo.FSActivity != nil &&
			artifactInfo.FSActivity.OpsCheckFile > 0 {
			log.Debugf("saveArtifacts - saving 'checked' file => %v", srcFileName)
			//NOTE: later have an option to save 'checked' only files without data
		}

		if p.cmd.ObfuscateMetadata {
			if isAppMetadataFile(srcFileName) {
				log.Tracef("saveArtifacts - isAppMetadataFile - src(%s)->dst(%s)", srcFileName, filePath)
				err := fsutil.CopyAndObfuscateFile(p.cmd.KeepPerms, srcFileName, filePath, true)
				if err != nil {
					log.Debugf("saveArtifacts [%s,%s] - error saving file => %v", srcFileName, filePath, err)
				}

				if err := appMetadataFileUpdater(filePath); err != nil {
					log.Debugf("saveArtifacts [%s,%s] - appMetadataFileUpdater => not updated / err = %v", srcFileName, filePath, err)
				}
			} else {
				err := fsutil.CopyRegularFile(p.cmd.KeepPerms, srcFileName, filePath, true)
				if err != nil {
					log.Debugf("saveArtifacts [%s,%s] - error saving file => %v", srcFileName, filePath, err)
				} else {
					//NOTE: this covers the main file set (doesn't cover the extra includes)
					binProps, err := binfile.Detected(filePath)
					if err == nil && binProps != nil && binProps.IsBin && binProps.IsExe {
						if err := fsutil.AppendToFile(filePath, []byte("KCQ"), true); err != nil {
							log.Debugf("saveArtifacts [%s,%s] - fsutil.AppendToFile error => %v", srcFileName, filePath, err)
						} else {
							log.Tracef("saveArtifacts - binfile.Detected[IsExe]/fsutil.AppendToFile - %s", filePath)

							err := fsutil.ReplaceFileData(filePath, binDataReplace, true)
							if err != nil {
								log.Debugf("saveArtifacts [%s,%s] - fsutil.ReplaceFileData error => %v", srcFileName, filePath, err)
							}
						}
					}
				}
			}
		} else {
			err := fsutil.CopyRegularFile(p.cmd.KeepPerms, srcFileName, filePath, true)
			if err != nil {
				log.Debugf("saveArtifacts - error saving file => %v", err)
			}
		}

		///////////////////
		fileName := srcFileName
		p.detectAppStack(fileName)

		if p.cmd.IncludeAppNuxtDir ||
			p.cmd.IncludeAppNuxtBuildDir ||
			p.cmd.IncludeAppNuxtDistDir ||
			p.cmd.IncludeAppNuxtStaticDir ||
			p.cmd.IncludeAppNuxtNodeModulesDir {
			if isNuxtConfigFile(fileName) {
				nuxtConfig, err := getNuxtConfig(fileName)
				if err != nil {
					log.Debugf("saveArtifacts: failed to get nuxt config: %v", err)
					continue
				}
				if nuxtConfig == nil {
					log.Debugf("saveArtifacts: nuxt config not found: %v", fileName)
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
				log.Debugf("saveArtifacts - error ensuring ruby gem files => %v", err)
			}
		} else if isNodePackageFile(fileName) {
			log.Debug("saveArtifacts - processing node package file ==>", fileName)
			err := nodeEnsurePackageFiles(p.cmd.KeepPerms, fileName, p.storeLocation, "/files")
			if err != nil {
				log.Debugf("saveArtifacts - error ensuring node package files => %v", err)
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
					log.Debugf("saveArtifacts - error getting node package config file => %v", err)
				}
			}

		} else if isNgxArtifact(fileName) && !ngxEnsured {
			log.Debug("saveArtifacts - ensuring ngx artifacts....")
			ngxEnsure(p.storeLocation)
			ngxEnsured = true
		} else {
			err := fixPy3CacheFile(fileName, filePath)
			if err != nil {
				log.Debugf("saveArtifacts - error fixing py3 cache file => %v", err)
			}
		}
		///////////////////
	}

	log.Debugf("saveArtifacts[bsa] - copy files (%v)", len(p.saFileMap))
copyBsaFiles:
	for srcFileName := range p.saFileMap {
		for _, xpattern := range excludePatterns {
			found, err := doublestar.Match(xpattern, srcFileName)
			if err != nil {
				log.Debugf("saveArtifacts[bsa] - copy files - [%v] excludePatterns Match error - %v\n", srcFileName, err)
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
			if p.cmd.ObfuscateMetadata && isAppMetadataFile(srcFileName) {
				err := fsutil.CopyAndObfuscateFile(p.cmd.KeepPerms, srcFileName, dstFilePath, true)
				if err != nil {
					log.Debugf("saveArtifacts[bsa] - error saving file => %v", err)
				} else {
					log.Debugf("saveArtifacts[bsa] - saved file (%s)", dstFilePath)
				}
			} else {
				err := fsutil.CopyRegularFile(p.cmd.KeepPerms, srcFileName, dstFilePath, true)
				if err != nil {
					log.Debugf("saveArtifacts[bsa] - error saving file => %v", err)
				} else {
					log.Debugf("saveArtifacts[bsa] - saved file (%s)", dstFilePath)
				}
			}
		}
	}

	//was conditional: if p.cmd.AppUser != ""
	//NOTE:
	//we may need the user info even if the caller didn't explicitly indicated it
	//makes this conditional again when/if we can fully analyze the target app(s)
	//to understand if it really needs the user info from the system
	copyBasicUserInfo := func() {
		//always copy the '/etc/passwd' file when we have a user
		//later: do it only when AppUser is a name (not UID)
		dstPasswdFilePath := fmt.Sprintf("%s/files%s", p.storeLocation, sysidentity.PasswdFilePath)
		if _, err := os.Stat(sysidentity.PasswdFilePath); err == nil {
			//if err := cpFile(passwdFilePath, passwdFileTargetPath); err != nil {
			if err := fsutil.CopyRegularFile(p.cmd.KeepPerms, sysidentity.PasswdFilePath, dstPasswdFilePath, true); err != nil {
				log.Debugf("sensor: monitor - error copying user info file => %v", err)
			}
		} else {
			if os.IsNotExist(err) {
				log.Debug("sensor: monitor - no user info file")
			} else {
				log.Debug("sensor: monitor - could not save user info file =>", err)
			}
		}
	}

	copyBasicUserInfo()

copyIncludes:
	for inPath, isDir := range includePaths {
		if artifact.IsFilteredPath(inPath) {
			log.Debugf("saveArtifacts - skipping filtered include path [isDir=%v] %s", isDir, inPath)
			continue
		}

		for _, xpattern := range excludePatterns {
			found, err := doublestar.Match(xpattern, inPath)
			if err != nil {
				log.Debugf("saveArtifacts - copy includes - [%v] excludePatterns Match error - %v\n", inPath, err)
				//should only happen when the pattern is malformed
				continue
			}
			if found {
				log.Debugf("saveArtifacts - copy includes - [%v] - excluding (%s) ", inPath, xpattern)
				continue copyIncludes
			}
		}

		dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, inPath)
		if isDir {
			err, errs := fsutil.CopyDir(p.cmd.KeepPerms, inPath, dstPath, true, true, excludePatterns, nil, nil)
			if err != nil {
				log.Debugf("CopyDir(%v,%v) error: %v", inPath, dstPath, err)
			}

			if len(errs) > 0 {
				log.Debugf("CopyDir(%v,%v) copy errors: %+v", inPath, dstPath, errs)
			}
		} else {
			if err := fsutil.CopyFile(p.cmd.KeepPerms, inPath, dstPath, true); err != nil {
				log.Debugf("CopyFile(%v,%v) error: %v", inPath, dstPath, err)
			}
		}
	}

	for _, exePath := range p.cmd.IncludeExes {
		exeArtifacts, err := sodeps.AllExeDependencies(exePath, true)
		if err != nil {
			log.Debugf("saveArtifacts - %v - error getting exe artifacts => %v", exePath, err)
			continue
		}

		log.Debugf("saveArtifacts - include exe [%s]: artifacts (%d):\n%v\n",
			exePath, len(exeArtifacts), strings.Join(exeArtifacts, "\n"))

		for _, apath := range exeArtifacts {
			dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, apath)
			if err := fsutil.CopyFile(p.cmd.KeepPerms, apath, dstPath, true); err != nil {
				log.Debugf("CopyFile(%v,%v) error: %v", apath, dstPath, err)
			}
		}
	}

	binPathMap := map[string]struct{}{}
	for _, binPath := range p.cmd.IncludeBins {
		binPathMap[binPath] = struct{}{}
	}

addExtraBinIncludes:
	for inPath, isDir := range includeDirBinsList {
		if !isDir {
			log.Debugf("saveArtifacts - skipping non-directory in includeDirBinsList - %s", inPath)
			continue
		}

		if artifact.IsFilteredPath(inPath) {
			log.Debugf("saveArtifacts - skipping filtered path in includeDirBinsList - %s", inPath)
			continue
		}

		for _, xpattern := range excludePatterns {
			found, err := doublestar.Match(xpattern, inPath)
			if err != nil {
				log.Debugf("saveArtifacts - includeDirBinsList - [%s] excludePatterns Match error - %v\n", inPath, err)
				//should only happen when the pattern is malformed
				continue
			}
			if found {
				log.Debugf("saveArtifacts - includeDirBinsList - [%s] - excluding (%s) ", inPath, xpattern)
				continue addExtraBinIncludes
			}
		}

		err := filepath.Walk(inPath,
			func(pth string, info os.FileInfo, err error) error {
				if strings.HasPrefix(pth, "/proc/") {
					log.Debugf("skipping /proc file system objects... - '%s'", pth)
					return filepath.SkipDir
				}

				if strings.HasPrefix(pth, "/sys/") {
					log.Debugf("skipping /sys file system objects... - '%s'", pth)
					return filepath.SkipDir
				}

				if strings.HasPrefix(pth, "/dev/") {
					log.Debugf("skipping /dev file system objects... - '%s'", pth)
					return filepath.SkipDir
				}

				// Optimization: Exclude folders early on to prevent slow enumerat
				//               Can help with mounting big folders from the host.
				// TODO: Combine this logic with the similar logic in findSymlinks().
				for _, xpattern := range excludePatterns {
					if match, _ := doublestar.Match(xpattern, pth); match {
						if info.Mode().IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
				}

				if err != nil {
					log.Debugf("skipping %s with error: %v", pth, err)
					return nil
				}

				if !info.Mode().IsRegular() {
					return nil
				}

				pth, err = filepath.Abs(pth)
				if err != nil {
					return nil
				}

				if strings.HasPrefix(pth, "/proc/") ||
					strings.HasPrefix(pth, "/sys/") ||
					strings.HasPrefix(pth, "/dev/") {
					return nil
				}

				if binProps, _ := binfile.Detected(pth); binProps != nil && binProps.IsBin {
					binPathMap[pth] = struct{}{}
				}

				return nil
			})

		if err != nil {
			log.Errorf("saveArtifacts - error enumerating includeDirBinsList dir (%s) - %v", inPath, err)
		}
	}

copyBinIncludes:
	for binPath := range binPathMap {
		if artifact.IsFilteredPath(binPath) {
			log.Debugf("saveArtifacts - skipping filtered include bin - %s", binPath)
			continue
		}

		for _, xpattern := range excludePatterns {
			found, err := doublestar.Match(xpattern, binPath)
			if err != nil {
				log.Debugf("saveArtifacts - copy bin includes - [%v] excludePatterns Match error - %v\n", binPath, err)
				//should only happen when the pattern is malformed
				continue
			}
			if found {
				log.Debugf("saveArtifacts - copy bin includes - [%v] - excluding (%s) ", binPath, xpattern)
				continue copyBinIncludes
			}
		}

		binArtifacts, err := sodeps.AllDependencies(binPath)
		if err != nil {
			log.Debugf("saveArtifacts - %v - error getting bin artifacts => %v", binPath, err)
			continue
		}

		log.Debugf("saveArtifacts - include bin [%s]: artifacts (%d):\n%v",
			binPath, len(binArtifacts), strings.Join(binArtifacts, "\n"))

		for _, bpath := range binArtifacts {
			dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, bpath)
			if err := fsutil.CopyFile(p.cmd.KeepPerms, bpath, dstPath, true); err != nil {
				log.Debugf("CopyFile(%v,%v) error: %v", bpath, dstPath, err)
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
					log.Debugf("CopyFile(%v,%v) error: %v", spath, dstPath, err)
				}
			}
		} else {
			log.Debugf("saveArtifacts - error getting shell artifacts => %v", err)
		}

	}

	p.saveWorkdir(excludePatterns)

	p.saveSSHClient()
	p.saveOSLibsNetwork()
	p.saveCertsData()
	p.saveZoneInfo()

	if fsutil.DirExists("/tmp") {
		tdTargetPath := fmt.Sprintf("%s/files/tmp", p.storeLocation)
		if !fsutil.DirExists(tdTargetPath) {
			if err := os.MkdirAll(tdTargetPath, os.ModeSticky|os.ModeDir|0777); err != nil {
				log.Debugf("saveArtifacts - error creating tmp directory => %v", err)
			}
		} else {
			if err := os.Chmod(tdTargetPath, os.ModeSticky|os.ModeDir|0777); err != nil {
				log.Debugf("saveArtifacts - error setting tmp directory permission ==> %v", err)
			}
		}
	}

	if fsutil.DirExists("/run") {
		tdTargetPath := fmt.Sprintf("%s/files/run", p.storeLocation)
		if !fsutil.DirExists(tdTargetPath) {
			//should use perms from source
			if err := os.MkdirAll(tdTargetPath, 0755); err != nil {
				log.Debugf("saveArtifacts - error creating run directory => %v", err)
			}
		}
	}

	for extraDir := range extraDirs {
		tdTargetPath := fmt.Sprintf("%s/files%s", p.storeLocation, extraDir)
		if fsutil.DirExists(extraDir) && !fsutil.DirExists(tdTargetPath) {
			if err := fsutil.CopyDirOnly(p.cmd.KeepPerms, extraDir, tdTargetPath); err != nil {
				log.Debugf("CopyDirOnly(%v,%v) error: %v", extraDir, tdTargetPath, err)
			}
		}
	}

	for inPath, perms := range newPerms {
		dstPath := fmt.Sprintf("%s/files%s", p.storeLocation, inPath)
		if fsutil.Exists(dstPath) {
			if err := fsutil.SetAccess(dstPath, perms); err != nil {
				log.Debugf("SetPerms(%v,%v) error: %v", dstPath, perms, err)
			}
		}
	}

	if len(p.cmd.Preserves) > 0 {
		log.Debugf("saveArtifacts: restoring preserved paths - %d", len(p.cmd.Preserves))

		preservedDirPath := filepath.Join(p.storeLocation, preservedDirName)
		if fsutil.Exists(preservedDirPath) {
			filesDirPath := filepath.Join(p.storeLocation, app.ArtifactFilesDirName)
			preservePaths := preparePaths(getKeys(p.cmd.Preserves))
			for inPath, isDir := range preservePaths {
				if artifact.IsFilteredPath(inPath) {
					log.Debugf("saveArtifacts: skipping filtered preserved path [isDir=%v] %s", isDir, inPath)
					continue
				}

				srcPath := fmt.Sprintf("%s%s", preservedDirPath, inPath)
				dstPath := fmt.Sprintf("%s%s", filesDirPath, inPath)

				if isDir {
					err, errs := fsutil.CopyDir(p.cmd.KeepPerms, srcPath, dstPath, true, true, nil, nil, nil)
					if err != nil {
						log.Debugf("saveArtifacts.CopyDir(%v,%v) error: %v", srcPath, dstPath, err)
					}

					if len(errs) > 0 {
						log.Debugf("saveArtifacts.CopyDir(%v,%v) copy errors: %+v", srcPath, dstPath, errs)
					}
				} else {
					if err := fsutil.CopyFile(p.cmd.KeepPerms, srcPath, dstPath, true); err != nil {
						log.Debugf("saveArtifacts.CopyFile(%v,%v) error: %v", srcPath, dstPath, err)
					}
				}
			}
		} else {
			log.Debug("saveArtifacts(): preserved root path doesnt exist")
		}
	}
}

func (p *store) detectAppStack(fileName string) {
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

func (p *store) archiveArtifacts() error {
	logger := log.WithField("op", "store.archiveArtifacts")
	logger.Trace("call")
	defer logger.Trace("exit")

	src := filepath.Join(p.storeLocation, app.ArtifactFilesDirName)
	dst := filepath.Join(p.storeLocation, filesArchiveName)
	logger.Debugf("src='%s' dst='%s'", src, dst)

	trimPrefix := fmt.Sprintf("%s/", src)
	return fsutil.ArchiveDir(dst, src, trimPrefix, "")
}

// Go over all saved artifacts and update the name list to make
// sure all the files & folders are reflected in the final report.
// Hopefully, just a temporary workaround until a proper refactoring.
func (p *store) enumerateArtifacts() {
	logger := log.WithField("op", "store.enumerateArtifacts")
	logger.Trace("call")
	defer logger.Trace("exit")

	knownFiles := list2map(p.nameList)
	artifactFilesDir := filepath.Join(p.storeLocation, app.ArtifactFilesDirName)

	var curpath string
	dirqueue := []string{artifactFilesDir}
	for len(dirqueue) > 0 {
		curpath, dirqueue = dirqueue[0], dirqueue[1:]

		entries, err := os.ReadDir(curpath)
		if err != nil {
			logger.WithError(err).Debugf("os.ReadDir(%s)", curpath)
			// Keep processing though since it might have been a partial result.
		}

		// Leaf element - empty dir.
		if len(entries) == 0 {
			// Trim /opt/_slim/artifacts/files prefix from the dirpath.
			curpath = strings.TrimPrefix(curpath, artifactFilesDir)

			if knownFiles[curpath] {
				continue
			}

			if props, err := artifactProps(curpath); err == nil {
				p.nameList = append(p.nameList, curpath)
				p.rawNames[curpath] = props
				knownFiles[curpath] = true
			} else {
				logger.WithError(err).
					WithField("path", curpath).
					Debugf("artifactProps(%s): failed computing dir artifact props", curpath)
			}
			continue
		}

		for _, child := range entries {
			childpath := filepath.Join(curpath, child.Name())
			if child.IsDir() {
				dirqueue = append(dirqueue, childpath)
				continue
			}

			// Trim /opt/_slim/artifacts/files prefix from the filepath.
			childpath = strings.TrimPrefix(childpath, artifactFilesDir)

			// Leaf element - regular file or symlink.
			if knownFiles[childpath] {
				continue
			}

			if props, err := artifactProps(childpath); err == nil {
				p.nameList = append(p.nameList, childpath)
				p.rawNames[childpath] = props
				knownFiles[childpath] = true
			} else {
				logger.WithError(err).
					WithField("path", childpath).
					Debugf("artifactProps(%s): failed computing artifact props", childpath)
			}
		}
	}
}

func (p *store) saveReport() error {
	logger := log.WithField("op", "store.saveReport")
	logger.Trace("call")
	defer logger.Trace("exit")

	creport := report.ContainerReport{
		Sensor: p.seReport,
		Monitors: report.MonitorReports{
			Pt:  p.ptMonReport,
			Fan: p.fanMonReport,
		},
	}

	if p.cmd != nil {
		creport.StartCommand = &report.StartCommandReport{
			AppName:       p.cmd.AppName,
			AppArgs:       p.cmd.AppArgs,
			AppUser:       p.cmd.AppUser,
			AppEntrypoint: p.cmd.AppEntrypoint,
			AppCmd:        p.cmd.AppCmd,
		}
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
		rawNameRecord, found := p.rawNames[fname]
		if found {
			creport.Image.Files = append(creport.Image.Files, rawNameRecord)
		} else {
			logger.Debugf("nameList file name (%s) not found in rawNames map", fname)
		}
	}

	_, err := os.Stat(p.storeLocation)
	if os.IsNotExist(err) {
		os.MkdirAll(p.storeLocation, 0777)
		if _, err := os.Stat(p.storeLocation); err != nil {
			return err
		}
	}

	reportFilePath := filepath.Join(p.storeLocation, report.DefaultContainerReportFileName)
	logger.Debugf("saving report to '%s'", reportFilePath)

	var reportData bytes.Buffer
	encoder := json.NewEncoder(&reportData)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(creport); err != nil {
		return err
	}

	return os.WriteFile(reportFilePath, reportData.Bytes(), 0644)
}

func getFileHash(artifactFileName string) (string, error) {
	fileData, err := os.ReadFile(artifactFileName)
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
			log.Debugf("sensor: monitor - fixPy3CacheFile - error copying file => %v", dstPyFilePath)
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
	foList, err := os.ReadDir(extBasePath)
	if err != nil {
		return err
	}

	for _, fo := range foList {
		if fo.IsDir() {
			platform := fo.Name()

			extPlatformPath := filepath.Join(extBasePath, platform)
			foVerList, err := os.ReadDir(extPlatformPath)
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
							log.Debugf("sensor: monitor - rbEnsureGemFiles - error copying file => %v", extBuildFlagFilePathDst)
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

	dat, err := os.ReadFile(path)
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
		log.Debugf("sensor: getNodePackageFileData(%s) - error loading data => %v", filePath, err)
		return nil, err
	}

	return &result, nil
}

func nodeEnsurePackageFiles(keepPerms bool, src, storeLocation, prefix string) error {
	if strings.HasSuffix(src, nodeNPMNodeGypPackage) {
		//for now only ensure that we have node-gyp for npm
		//npm requires it to be there even though it won't use it
		//'check if exists' condition (not picked up by the FAN monitor, but picked up by the PT monitor)
		nodeGypFilePath := path.Join(filepath.Dir(src), nodeNPMNodeGypFile)
		if _, err := os.Stat(nodeGypFilePath); err == nil {
			nodeGypFilePathDst := fmt.Sprintf("%s%s%s", storeLocation, prefix, nodeGypFilePath)
			if err := fsutil.CopyRegularFile(keepPerms, nodeGypFilePath, nodeGypFilePathDst, true); err != nil {
				log.Debugf("sensor: nodeEnsurePackageFiles - error copying %s => %v", nodeGypFilePath, err)
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
					log.Debugf("ngxEnsure - MkdirAll(%v) error: %v", dstPath, err)
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
					log.Debugf("ngxEnsure -  MkdirAll(%v) error: %v", dstPath, err)
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
					log.Debugf("ngxEnsure -  MkdirAll(%v) error: %v", dstPath, err)
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

func shellDependencies() ([]string, error) {
	var allDeps []string
	for _, name := range artifact.ShellNames {
		shellPath, err := exec.LookPath(name)
		if err != nil {
			log.Debugf("shellDependencies - checking '%s' shell (not found: %s)", name, err)
			continue
		}

		exeArtifacts, err := sodeps.AllExeDependencies(shellPath, true)
		if err != nil {
			log.Debugf("shellDependencies - %v - error getting shell artifacts => %v", shellPath, err)
			return nil, err
		}

		allDeps = append(allDeps, exeArtifacts...)
		break
	}

	if len(allDeps) == 0 {
		log.Debug("shellDependencies - no shell found")
		return nil, nil
	}

	for _, name := range artifact.ShellCommands {
		cmdPath, err := exec.LookPath(name)
		if err != nil {
			log.Debugf("shellDependencies - checking '%s' cmd (not found: %s)", name, err)
			continue
		}

		cmdArtifacts, err := sodeps.AllExeDependencies(cmdPath, true)
		if err != nil {
			log.Debugf("shellDependencies - %v - error getting cmd artifacts => %v", cmdPath, err)
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

func findSymlinks(files []string, mountPoint string, excludes []string) map[string]*report.ArtifactProps {
	log.Debugf("findSymlinks(%v,%v)", len(files), mountPoint)

	result := map[string]*report.ArtifactProps{}
	symlinks := map[string]string{}

	checkPathSymlinks := func(symlinkFileName string) {
		if _, ok := result[symlinkFileName]; ok {
			log.Tracef("findSymlinks.checkPathSymlinks - symlink already in files -> %v", symlinkFileName)
			return
		}

		linkRef, err := os.Readlink(symlinkFileName)
		if err != nil {
			log.Debugf("findSymlinks.checkPathSymlinks - error getting reference for symlink (%v) -> %v", err, symlinkFileName)
			return
		}

		var absLinkRef string
		if !filepath.IsAbs(linkRef) {
			linkDir := filepath.Dir(symlinkFileName)
			log.Tracef("findSymlinks.checkPathSymlinks - relative linkRef %v -> %v +/+ %v", symlinkFileName, linkDir, linkRef)
			fullLinkRef := filepath.Join(linkDir, linkRef)
			var err error
			absLinkRef, err = filepath.Abs(fullLinkRef)
			if err != nil {
				log.Debugf("findSymlinks.checkPathSymlinks - error getting absolute path for symlink ref (1) (%v) -> %v => %v", err, symlinkFileName, fullLinkRef)
				return
			}
		} else {
			var err error
			absLinkRef, err = filepath.Abs(linkRef)
			if err != nil {
				log.Debugf("findSymlinks.checkPathSymlinks - error getting absolute path for symlink ref (2) (%v) -> %v => %v", err, symlinkFileName, linkRef)
				return
			}
		}

		//todo: skip "/proc/..." references
		evalLinkRef, err := filepath.EvalSymlinks(absLinkRef)
		if err != nil {
			log.Debugf("findSymlinks.checkPathSymlinks - error evaluating symlink (%v) -> %v => %v", err, symlinkFileName, absLinkRef)
		}

		//detecting intermediate dir symlinks
		symlinkPrefix := fmt.Sprintf("%s/", symlinkFileName)
		absPrefix := fmt.Sprintf("%s/", absLinkRef)
		evalPrefix := fmt.Sprintf("%s/", evalLinkRef)

		//TODO:
		//have an option not to resolve intermediate dir symlinks
		//it'll result in file duplication, but the symlinks
		//resolution logic will be less complicated and faster
		for _, fname := range files {
			added := false
			if strings.HasPrefix(fname, symlinkPrefix) {
				result[symlinkFileName] = nil
				log.Tracef("findSymlinks.checkPathSymlinks - added path symlink to files (0) -> %v", symlinkFileName)
				added = true
			}

			if strings.HasPrefix(fname, absPrefix) {
				result[symlinkFileName] = nil
				log.Tracef("findSymlinks.checkPathSymlinks - added path symlink to files (1) -> %v", symlinkFileName)
				added = true
			}

			if evalLinkRef != "" &&
				absPrefix != evalPrefix &&
				strings.HasPrefix(fname, evalPrefix) {
				result[symlinkFileName] = nil
				log.Tracef("findSymlinks.checkPathSymlinks - added path symlink to files (2) -> %v", symlinkFileName)
				added = true
			}

			if added {
				return
			}
		}

		symlinks[symlinkFileName] = linkRef
	}

	inodes, devices := filesToInodesNative(files)
	log.Debugf("findSymlinks - len(inodes)=%v len(devices)=%v", len(inodes), len(devices))

	inodeToFiles := map[uint64][]string{}

	//native filepath.Walk is a bit slow (compared to the "find" command)
	//but it's fast enough for now
	filepath.Walk(mountPoint,
		func(fullName string, fileInfo os.FileInfo, err error) error {
			if strings.HasPrefix(fullName, "/proc/") {
				log.Debugf("findSymlinks: skipping /proc file system objects...")
				return filepath.SkipDir
			}

			if strings.HasPrefix(fullName, "/sys/") {
				log.Debugf("findSymlinks: skipping /sys file system objects...")
				return filepath.SkipDir
			}

			if strings.HasPrefix(fullName, "/dev/") {
				log.Debugf("findSymlinks: skipping /dev file system objects...")
				return filepath.SkipDir
			}

			// Optimization: Avoid walking excluded folders. Supposed to help with
			//               mounting big folders from the host (they should be explicitly
			//               excluded).
			// TODO: Combine this logic with the similar logic in GetCurrentPaths().
			for _, xpattern := range excludes {
				if match, _ := doublestar.Match(xpattern, fullName); match {
					if fileInfo.Mode().IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			if err != nil {
				log.Debugf("findSymlinks: error accessing %q: %v\n", fullName, err)
				//just ignore the error and keep going
				return nil
			}

			if fileInfo.Sys() == nil {
				log.Debugf("findSymlinks: fileInfo.Sys() is nil (ignoring)")
				return nil
			}

			sysStatInfo, ok := fileInfo.Sys().(*syscall.Stat_t)
			if !ok {
				return fmt.Errorf("findSymlinks - could not convert fileInfo to Stat_t for %s", fullName)
			}

			if _, ok := devices[uint64(sysStatInfo.Dev)]; !ok {
				log.Debugf("findSymlinks: ignoring %v (by device id - %v)", fullName, sysStatInfo.Dev)
				//NOTE:
				//don't return filepath.SkipDir for everything
				//because we might still need other files in the dir
				//return filepath.SkipDir
				//example: "/etc/hostname" Docker mounts from another device
				//NOTE:
				//can move the checks for /dev, /sys and /proc here too
				return nil
			}

			if fileInfo.Mode()&os.ModeSymlink != 0 {
				checkPathSymlinks(fullName)

				if info, err := getFileSysStats(fullName); err == nil {

					if _, ok := inodes[info.Ino]; ok {
						//not using the inode for the link (using the target inode instead)
						inodeToFiles[info.Ino] = append(inodeToFiles[info.Ino], fullName)
					} else {
						//log.Debugf("findSymlinks - don't care about this symlink (%s)",fullName)
					}

				} else {
					log.Infof("findSymlinks - could not get target stats info for file (%v) -> %v", err, fullName)
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

	log.Debugf("findSymlinks - len(inodeToFiles)=%v", len(inodeToFiles))

	for inodeID := range inodes {
		v := inodeToFiles[inodeID]
		for _, f := range v {
			//result[f] = inodeID
			result[f] = nil
		}
	}

	//NOTE/TODO:
	//Might need multiple passes until no new symlinks are added to result
	//(with the current approach)
	//Should REDESIGN to use a reverse/target radix and a radix-based result
	for symlinkFileName, linkRef := range symlinks {
		var absLinkRef string
		if !filepath.IsAbs(linkRef) {
			linkDir := filepath.Dir(symlinkFileName)
			log.Debugf("findSymlinks.walkSymlinks - relative linkRef %v -> %v +/+ %v", symlinkFileName, linkDir, linkRef)
			fullLinkRef := filepath.Join(linkDir, linkRef)
			var err error
			absLinkRef, err = filepath.Abs(fullLinkRef)
			if err != nil {
				log.Debugf("findSymlinks.walkSymlinks - error getting absolute path for symlink ref (1) (%v) -> %v => %v", err, symlinkFileName, fullLinkRef)
				break
			}
		} else {
			var err error
			absLinkRef, err = filepath.Abs(linkRef)
			if err != nil {
				log.Debugf("findSymlinks.walkSymlinks - error getting absolute path for symlink ref (2) (%v) -> %v => %v", err, symlinkFileName, linkRef)
				break
			}
		}

		//todo: skip "/proc/..." references
		evalLinkRef, err := filepath.EvalSymlinks(absLinkRef)
		if err != nil {
			log.Debugf("findSymlinks.walkSymlinks - error evaluating symlink (%v) -> %v => %v", err, symlinkFileName, absLinkRef)
		}

		//detecting intermediate dir symlinks
		symlinkPrefix := fmt.Sprintf("%s/", symlinkFileName)
		absPrefix := fmt.Sprintf("%s/", absLinkRef)
		evalPrefix := fmt.Sprintf("%s/", evalLinkRef)

		for fname := range result {
			added := false
			if strings.HasPrefix(fname, symlinkPrefix) {
				result[symlinkFileName] = nil
				log.Debugf("findSymlinks.walkSymlinks - added path symlink to files (0) -> %v", symlinkFileName)
				added = true
			}

			if strings.HasPrefix(fname, absPrefix) {
				result[symlinkFileName] = nil
				log.Debugf("findSymlinks.walkSymlinks - added path symlink to files (1) -> %v", symlinkFileName)
				added = true
			}

			if evalLinkRef != "" &&
				absPrefix != evalPrefix &&
				strings.HasPrefix(fname, evalPrefix) {
				result[symlinkFileName] = nil
				log.Debugf("findSymlinks.walkSymlinks - added path symlink to files (2) -> %v", symlinkFileName)
				added = true
			}

			if added {
				break
			}
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
