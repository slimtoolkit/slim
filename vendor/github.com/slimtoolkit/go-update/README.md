# go-update: Build self-updating Go programs [![godoc reference](https://godoc.org/github.com/docker-slim/go-update?status.png)](https://godoc.org/github.com/docker-slim/go-update)

## Note

The forked version will fix a few minor bugs and add a few new features (using updates for installs when the executables are't there yet or updating with a file path instead of a reader interface)

## Original Package Info

Package update provides functionality to implement secure, self-updating Go programs (or other single-file targets)
A program can update itself by replacing its executable file with a new version.

It provides the flexibility to implement different updating user experiences
like auto-updating, or manual user-initiated updates. It also boasts
advanced features like binary patching and code signing verification.

Example of updating from a URL:

```go
import (
    "fmt"
    "net/http"

    "github.com/docker-slim/go-update"
)

func doUpdate(url string) error {
    resp, err := http.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    err := update.Apply(resp.Body, update.Options{})
    if err != nil {
        // error handling
    }
    return err
}
```

### Features

- Cross platform support (Windows too!)
- Binary patch application
- Checksum verification
- Code signing verification
- Support for updating arbitrary files

### API Compatibility Promises
The master branch of `go-update` is *not* guaranteed to have a stable API over time. For any production application, you should vendor
your dependency on `go-update` with a tool like git submodules, [gb](http://getgb.io/) or [govendor](https://github.com/kardianos/govendor).

The `go-update` package makes the following promises about API compatibility:
1. A list of all API-breaking changes will be documented in this README.
1. `go-update` will strive for as few API-breaking changes as possible.

### License
Apache
