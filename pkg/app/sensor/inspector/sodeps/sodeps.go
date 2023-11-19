package sodeps

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/slimtoolkit/slim/pkg/app/sensor/detector/binfile"

	log "github.com/sirupsen/logrus"
)

// Inspector errors
var (
	ErrFilePathNotAbs      = errors.New("file path is not absolute")
	ErrFileNotBin          = errors.New("file is not a binary")
	ErrDepResolverNotFound = errors.New("dependency resolver not found")
)

const (
	resolverExeName = "ldd"
)

func AllExeDependencies(exeFileName string, find bool) ([]string, error) {
	if !strings.HasPrefix(exeFileName, "/") {
		if !find {
			return nil, ErrFilePathNotAbs
		}

		exePath, err := exec.LookPath(exeFileName)
		if err != nil {
			return nil, err
		}

		exeFileName = exePath
	}

	return AllDependencies(exeFileName)
}

const (
	strExitStatus       = "exit status 127"
	strErrorReloc       = "Error relocating"
	strErrorSymNotFound = "symbol not found"
)

func AllDependencies(binFilePath string) ([]string, error) {
	//binFilePath could point to an executable or a shared object
	if !strings.HasPrefix(binFilePath, "/") {
		return nil, ErrFilePathNotAbs
	}

	if _, err := os.Stat(binFilePath); err != nil {
		if os.IsNotExist(err) {
			log.Debugf("sodeps.AllDependencies(%v): missing target - %v", binFilePath, err)
		}

		return nil, err
	}

	if binProps, _ := binfile.Detected(binFilePath); binProps == nil || !binProps.IsBin {
		return nil, ErrFileNotBin
	}

	resolverExePath, err := exec.LookPath(resolverExeName)
	if err != nil {
		log.Debugf("sodeps.AllDependencies(%v): resolver not found - %v", binFilePath, err)
		return nil, ErrDepResolverNotFound
	}

	var cerr bytes.Buffer
	var cout bytes.Buffer
	cmd := exec.Command(resolverExePath, binFilePath)
	cmd.Stderr = &cerr
	cmd.Stdout = &cout

	if err := cmd.Start(); err != nil {
		log.Debugf("sodeps.AllDependencies(%v): exe run error - %v", binFilePath, err)
		return nil, err
	}

	depMap := map[string]struct{}{}
	var deps []string
	deps = append(deps, binFilePath)

	if err := cmd.Wait(); err != nil {
		if strings.Contains(err.Error(), strExitStatus) &&
			strings.Contains(cerr.String(), strErrorReloc) &&
			strings.Contains(cerr.String(), strErrorSymNotFound) {
			elines := strings.Split(cerr.String(), "\n")
			for _, line := range elines {
				clean := strings.TrimSpace(line)
				if len(clean) == 0 {
					continue
				}

				if !strings.Contains(clean, strErrorReloc) {
					log.Debugf("sodeps.AllDependencies(%v): skipping '%s'", binFilePath, line)
					continue
				}

				parts := strings.Split(clean, " ")
				//Error relocating __LIB_NAME__: __SYMBOL_NAME__: symbol not found
				if len(parts) == 7 {
					if strings.HasSuffix(parts[2], ":") {
						dep := strings.Trim(parts[2], ":")
						depMap[dep] = struct{}{}
					}
				} else {
					log.Debugf("sodeps.AllDependencies(%v): skipping '%s'", binFilePath, line)
				}
			}

		} else {
			log.Debugf("sodeps.AllDependencies(%v): exe error result - %v (stderr: '%v')", binFilePath, err, cerr.String())
			return nil, err
		}
	}

	for dep := range depMap {
		deps = append(deps, dep)
	}

	lines := strings.Split(cout.String(), "\n")
	for _, line := range lines {
		clean := strings.TrimSpace(line)
		if len(clean) == 0 {
			continue
		}

		if strings.Contains(clean, "statically linked") {
			log.Debugf("sodeps.AllDependencies(%v): statically linked binary file", binFilePath)
			continue
		}

		parts := strings.Split(clean, " ")

		if len(parts) == 4 {
			if parts[1] != "=>" {
				log.Debugf("sodeps.AllDependencies(%v): unexpected line format - '%v'", binFilePath, clean)
				continue
			}

			if parts[2] == resolverExeName {
				log.Debugf("sodeps.AllDependencies(%v): ignore non-lib dependency - '%v'", binFilePath, clean)
				continue
			}

			deps = append(deps, parts[2])
			continue
		}

		if len(parts) == 2 {
			if strings.HasPrefix(parts[0], "/") {
				//full path (dynamic linker)
				deps = append(deps, parts[0])
				continue
			}

			if strings.HasPrefix(parts[0], "linux-vdso") {
				continue
			}

			if strings.HasPrefix(parts[0], resolverExeName) {
				log.Debugf("sodeps.AllDependencies(%v): ignore resolver dependency - '%v'", binFilePath, clean)
				continue
			}

			log.Debugf("sodeps.AllDependencies(%v): unexpected line - '%v'", binFilePath, clean)
			continue
		}
	}

	var allDeps []string
	for depth := 0; len(deps) > 0; depth++ {
		var fileDeps []string
		fileDeps, deps = resolveDepArtifacts(deps)
		allDeps = append(allDeps, fileDeps...)

		if depth > 5 {
			log.Debugf("sodeps.AllDependencies(%v): link ref too deep - breaking", binFilePath)
			break
		}
	}

	return allDeps, nil
}

func resolveDepArtifacts(names []string) (files, links []string) {
	for _, name := range names {
		if info, err := os.Lstat(name); err == nil {
			files = append(files, name)
			if info.Mode()&os.ModeSymlink != 0 {
				linkRef, err := os.Readlink(name)
				if err != nil {
					log.Debugf("sodeps.resolveDepArtifacts: %v - error reading link (%v)\n", name, err)
					continue
				}

				var absLinkRef string
				if !filepath.IsAbs(linkRef) {
					linkDir := filepath.Dir(name)
					fullLinkRef := filepath.Join(linkDir, linkRef)
					var err error
					absLinkRef, err = filepath.Abs(fullLinkRef)
					if err != nil {
						log.Debugf("sodeps.resolveDepArtifacts: %v - error getting absolute path for symlink ref (1) %v - (%v)\n", name, fullLinkRef, err)
						continue
					}
				} else {
					var err error
					absLinkRef, err = filepath.Abs(linkRef)
					if err != nil {
						log.Debugf("sodeps.resolveDepArtifacts: %v - error getting absolute path for symlink ref (2) %v - (%v)\n", name, linkRef, err)
						continue
					}
				}

				links = append(links, absLinkRef)
			}
		} else {
			if os.IsNotExist(err) {
				log.Debugf("sodeps.resolveDepArtifacts: %v - missing dep (%v)\n", name, err)
			} else {
				log.Debugf("sodeps.resolveDepArtifacts: %v - error checking dep (%v)\n", name, err)
			}
		}
	}

	return files, links
}
