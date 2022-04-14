package manager

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/config"
	"github.com/fvbommel/sortorder"
	"github.com/spf13/cobra"
	exec "golang.org/x/sys/execabs"
)

// ReexecEnvvar is the name of an ennvar which is set to the command
// used to originally invoke the docker CLI when executing a
// plugin. Assuming $PATH and $CWD remain unchanged this should allow
// the plugin to re-execute the original CLI.
const ReexecEnvvar = "DOCKER_CLI_PLUGIN_ORIGINAL_CLI_COMMAND"

// errPluginNotFound is the error returned when a plugin could not be found.
type errPluginNotFound string

func (e errPluginNotFound) NotFound() {}

func (e errPluginNotFound) Error() string {
	return "Error: No such CLI plugin: " + string(e)
}

type notFound interface{ NotFound() }

// IsNotFound is true if the given error is due to a plugin not being found.
func IsNotFound(err error) bool {
	if e, ok := err.(*pluginError); ok {
		err = e.Cause()
	}
	_, ok := err.(notFound)
	return ok
}

func getPluginDirs(dockerCli command.Cli) ([]string, error) {
	var pluginDirs []string

	if cfg := dockerCli.ConfigFile(); cfg != nil {
		pluginDirs = append(pluginDirs, cfg.CLIPluginsExtraDirs...)
	}
	pluginDir, err := config.Path("cli-plugins")
	if err != nil {
		return nil, err
	}

	pluginDirs = append(pluginDirs, pluginDir)
	pluginDirs = append(pluginDirs, defaultSystemPluginDirs...)
	return pluginDirs, nil
}

func addPluginCandidatesFromDir(res map[string][]string, d string) error {
	dentries, err := os.ReadDir(d)
	if err != nil {
		return err
	}
	for _, dentry := range dentries {
		switch dentry.Type() & os.ModeType {
		case 0, os.ModeSymlink:
			// Regular file or symlink, keep going
		default:
			// Something else, ignore.
			continue
		}
		name := dentry.Name()
		if !strings.HasPrefix(name, NamePrefix) {
			continue
		}
		name = strings.TrimPrefix(name, NamePrefix)
		var err error
		if name, err = trimExeSuffix(name); err != nil {
			continue
		}
		res[name] = append(res[name], filepath.Join(d, dentry.Name()))
	}
	return nil
}

// listPluginCandidates returns a map from plugin name to the list of (unvalidated) Candidates. The list is in descending order of priority.
func listPluginCandidates(dirs []string) (map[string][]string, error) {
	result := make(map[string][]string)
	for _, d := range dirs {
		// Silently ignore any directories which we cannot
		// Stat (e.g. due to permissions or anything else) or
		// which is not a directory.
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
			continue
		}
		if err := addPluginCandidatesFromDir(result, d); err != nil {
			// Silently ignore paths which don't exist.
			if os.IsNotExist(err) {
				continue
			}
			return nil, err // Or return partial result?
		}
	}
	return result, nil
}

// GetPlugin returns a plugin on the system by its name
func GetPlugin(name string, dockerCli command.Cli, rootcmd *cobra.Command) (*Plugin, error) {
	pluginDirs, err := getPluginDirs(dockerCli)
	if err != nil {
		return nil, err
	}

	candidates, err := listPluginCandidates(pluginDirs)
	if err != nil {
		return nil, err
	}

	if paths, ok := candidates[name]; ok {
		if len(paths) == 0 {
			return nil, errPluginNotFound(name)
		}
		c := &candidate{paths[0]}
		p, err := newPlugin(c, rootcmd)
		if err != nil {
			return nil, err
		}
		if !IsNotFound(p.Err) {
			p.ShadowedPaths = paths[1:]
		}
		return &p, nil
	}

	return nil, errPluginNotFound(name)
}

// ListPlugins produces a list of the plugins available on the system
func ListPlugins(dockerCli command.Cli, rootcmd *cobra.Command) ([]Plugin, error) {
	pluginDirs, err := getPluginDirs(dockerCli)
	if err != nil {
		return nil, err
	}

	candidates, err := listPluginCandidates(pluginDirs)
	if err != nil {
		return nil, err
	}

	var plugins []Plugin
	for _, paths := range candidates {
		if len(paths) == 0 {
			continue
		}
		c := &candidate{paths[0]}
		p, err := newPlugin(c, rootcmd)
		if err != nil {
			return nil, err
		}
		if !IsNotFound(p.Err) {
			p.ShadowedPaths = paths[1:]
			plugins = append(plugins, p)
		}
	}

	sort.Slice(plugins, func(i, j int) bool {
		return sortorder.NaturalLess(plugins[i].Name, plugins[j].Name)
	})

	return plugins, nil
}

// PluginRunCommand returns an "os/exec".Cmd which when .Run() will execute the named plugin.
// The rootcmd argument is referenced to determine the set of builtin commands in order to detect conficts.
// The error returned satisfies the IsNotFound() predicate if no plugin was found or if the first candidate plugin was invalid somehow.
func PluginRunCommand(dockerCli command.Cli, name string, rootcmd *cobra.Command) (*exec.Cmd, error) {
	// This uses the full original args, not the args which may
	// have been provided by cobra to our caller. This is because
	// they lack e.g. global options which we must propagate here.
	args := os.Args[1:]
	if !pluginNameRe.MatchString(name) {
		// We treat this as "not found" so that callers will
		// fallback to their "invalid" command path.
		return nil, errPluginNotFound(name)
	}
	exename := addExeSuffix(NamePrefix + name)
	pluginDirs, err := getPluginDirs(dockerCli)
	if err != nil {
		return nil, err
	}

	for _, d := range pluginDirs {
		path := filepath.Join(d, exename)

		// We stat here rather than letting the exec tell us
		// ENOENT because the latter does not distinguish a
		// file not existing from its dynamic loader or one of
		// its libraries not existing.
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		c := &candidate{path: path}
		plugin, err := newPlugin(c, rootcmd)
		if err != nil {
			return nil, err
		}
		if plugin.Err != nil {
			// TODO: why are we not returning plugin.Err?
			return nil, errPluginNotFound(name)
		}
		cmd := exec.Command(plugin.Path, args...)
		// Using dockerCli.{In,Out,Err}() here results in a hang until something is input.
		// See: - https://github.com/golang/go/issues/10338
		//      - https://github.com/golang/go/commit/d000e8742a173aa0659584aa01b7ba2834ba28ab
		// os.Stdin is a *os.File which avoids this behaviour. We don't need the functionality
		// of the wrappers here anyway.
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, ReexecEnvvar+"="+os.Args[0])

		return cmd, nil
	}
	return nil, errPluginNotFound(name)
}
