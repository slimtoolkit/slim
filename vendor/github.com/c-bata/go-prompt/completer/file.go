package completer

import (
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/c-bata/go-prompt"
)

var (
	FilePathCompletionSeparator = string([]byte{' ', os.PathSeparator})
)

// FilePathCompleter is a completer for your local file system.
// Please caution that you need to set OptionCompletionWordSeparator(completer.FilePathCompletionSeparator)
// when you use this completer.
type FilePathCompleter struct {
	Filter        func(fi os.FileInfo) bool
	IgnoreCase    bool
	fileListCache map[string][]prompt.Suggest
}

func cleanFilePath(path string) (dir, base string, err error) {
	if path == "" {
		return ".", "", nil
	}

	var endsWithSeparator bool
	if len(path) >= 1 && path[len(path)-1] == os.PathSeparator {
		endsWithSeparator = true
	}

	if runtime.GOOS != "windows" && len(path) >= 2 && path[0:2] == "~/" {
		me, err := user.Current()
		if err != nil {
			return "", "", err
		}
		path = filepath.Join(me.HomeDir, path[1:])
	}
	path = filepath.Clean(os.ExpandEnv(path))
	dir = filepath.Dir(path)
	base = filepath.Base(path)

	if endsWithSeparator {
		dir = path + string(os.PathSeparator) // Append slash(in POSIX) if path ends with slash.
		base = ""                             // Set empty string if path ends with separator.
	}
	return dir, base, nil
}

// Complete returns suggestions from your local file system.
func (c *FilePathCompleter) Complete(d prompt.Document) []prompt.Suggest {
	if c.fileListCache == nil {
		c.fileListCache = make(map[string][]prompt.Suggest, 4)
	}

	path := d.GetWordBeforeCursor()
	dir, base, err := cleanFilePath(path)
	if err != nil {
		log.Print("[ERROR] completer: cannot get current user " + err.Error())
		return nil
	}

	if cached, ok := c.fileListCache[dir]; ok {
		return prompt.FilterHasPrefix(cached, base, c.IgnoreCase)
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil && os.IsNotExist(err) {
		return nil
	} else if err != nil {
		log.Print("[ERROR] completer: cannot read directory items " + err.Error())
		return nil
	}

	suggests := make([]prompt.Suggest, 0, len(files))
	for _, f := range files {
		if c.Filter != nil && !c.Filter(f) {
			continue
		}
		suggests = append(suggests, prompt.Suggest{Text: f.Name()})
	}
	c.fileListCache[dir] = suggests
	return prompt.FilterHasPrefix(suggests, base, c.IgnoreCase)
}
