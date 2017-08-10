# Enhancements Wishlist

This is a list of non-core related enhancements and features. If you want to help feel free to submit a PR! You can also open an issue to track the work and to have a place to talk about it.

## Publish DockerSlim to various package repos

Package Managers:
* Homebrew
* Mac Ports
* Apt
* ???

## Classic container image optimizations (aka ability to disable minification based on dynamic/static analysis)

* Docker image flattening
* OS specific cleanup commands (optional)

## Replace Nanomsg/mongos with Libchan or Something Else

Nanomsg is "officially" dead. Need to replace it. It's also not reliable enough (right now missed event notifications from the sensor are the main reason for most cases when DockerSlim "hangs"). Either way, the sequential request processing model (imposed by mongos) isn't a great fit for the DockerSlim communication interface.

## ARM Support (aarch64, armhf)

Need to enhance the "system" library to support ARM syscalls, so the seccomp feature works on ARM systems.

Also need to make "ptrace" enhancements.

Original issues:
* https://github.com/docker-slim/docker-slim/issues/6
* https://github.com/docker-slim/docker-slim/issues/25




