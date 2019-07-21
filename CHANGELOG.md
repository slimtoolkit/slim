## 1.25.2 (7/21/2019)

### New Features

* Enhanced build command reports with additional container image metadata (using the global `--report` flag)
* Ability to update the minified image Dockerfile instructions (using the --new-cmd, --new-entrypoint, --new-expose, --new-workdir, --new-env and --image-overrides flags)
* Dockerfile volume support

### Improvements

* HTTP probes by default (you will have to disable HTTP probes if you don't need them)
* Various UX enhancements to provide better CLI feedback and to avoid generating minified images that might not work

### Bug Fixes

* TTY bug fix caused by an external dependency (used to track update download progress)

## 1.25.0 (4/23/2019)

### New Features

* Experimental ARM32 support
* Easy way to keep a shell in your image (just pass `--include-shell` to the `build` command)
* Easy way to include additional executables (`--include-exe` flag) and binary objects (`--include-bin` flag), which will also include their binary dependencies, so you don't have to explicitly include them all yourself
* `update` command - now you can update `docker-slim` from `docker-slim`!
* Current version checks to know if the installed release is out of date

### Improvements

* Improvements to handle complex `--entrypoint` and `--cmd` parameters

## Previous Releases

* Better Mac OS X support - when you install `docker-slim` to /usr/local/bin or other special/non-shared directories docker-slim will detect it and use the /temp directory to save its artifacts and to mount its sensor
* HTTP Probing enhancements and new flags to control the probing process
* Better Nginx support
* Support for non-default users
* Improved symlink handling
* Better failure monitoring and reporting
* The `--include-path-file` option to make it easier to load extra files you want to keep in your image
* CentOS support
* Enhancements for ruby applications with extensions
* Save the docker-slim command results in a JSON file using the `--report` flag
* Better support for applications with dynamic libraries (e.g., python compiled with `--enable-shared`)
* Additional network related Docker parameters
* Extended version information
* Alpine image support
* Ability to override ENV variables analyzing target image
* Docker 1.12 support
* User selected location to store DockerSlim state (global `--state-path` parameter).
* Auto-generated seccomp profiles for Docker 1.10.
* Python 3 support
* Docker connect options
* HTTP probe commands
* Include extra directories and files in minified images
