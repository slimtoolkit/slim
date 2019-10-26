# Enhancements Wishlist

This is a list of non-core related enhancements and features. If you want to help feel free to submit a PR! You can also open an issue to track the work and to have a place to talk about it.

## Publish DockerSlim to various package repos

Package Managers:
* Homebrew
* Mac Ports
* Apt/Debian
* ???

## Classic container image optimizations (aka ability to disable minification based on dynamic/static analysis)

* Docker image flattening
* OS specific cleanup commands (optional)

## ARM Support (aarch64, armhf)

Need to enhance the "system" library to support ARM syscalls, so the seccomp feature works on ARM systems.

Also need to make "ptrace" enhancements.

Original issues:
* https://github.com/docker-slim/docker-slim/issues/6
* https://github.com/docker-slim/docker-slim/issues/25

## Native Windows Support

Original issue:
* https://github.com/docker-slim/docker-slim/issues/57

## Provide an Audit Log for the Removed Files

Original issue:
* https://github.com/docker-slim/docker-slim/issues/67

## Dockerizing Local Applications (Linux Only)

Dockerizing local applications and creating minified images for them.

## Minifying Local Applications and Saving to Directory (Linux Only)

Original issue:
https://github.com/docker-slim/docker-slim/issues/60



