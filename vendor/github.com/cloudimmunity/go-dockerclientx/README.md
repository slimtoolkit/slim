# go-dockerclientx

[![Drone](https://drone.io/github.com/cloudimmunity/go-dockerclientx/status.png)](https://drone.io/github.com/cloudimmunity/go-dockerclientx/latest)
[![Travis](https://img.shields.io/travis/cloudimmunity/go-dockerclientx.svg?style=flat-square)](https://travis-ci.org/cloudimmunity/go-dockerclientx)
[![GoDoc](https://img.shields.io/badge/api-Godoc-blue.svg?style=flat-square)](https://godoc.org/github.com/cloudimmunity/go-dockerclientx)

## Fork Info

Forked from `github.com/fsouza/go-dockerclient`.

Goals:

* Keep up with the current Docker API.
* Raw API interface (to have a way out in case the library can't keep up with the official Docker API or if you need more control over the calls).
* Easy debugging and tracing.
* Extensive client documentation and samples.
* Docker Remote API documentation (because the official docs are often incomplete)

## Overview

This package presents a client for the Docker remote API. It also provides
support for the extensions in the [Swarm API](https://docs.docker.com/swarm/API/).

This package also provides support for docker's network API, which is a simple
passthrough to the libnetwork remote API.  Note that docker's network API is
only available in docker 1.8 and above, and only enabled in docker if
DOCKER_EXPERIMENTAL is defined during the docker build process.

For more details, check the [remote API documentation](http://docs.docker.com/en/latest/reference/api/docker_remote_api/).

## Vendoring

If you are having issues with Go 1.5 and have `GO15VENDOREXPERIMENT` set with an application that has go-dockerclient vendored,
please update your vendoring of go-dockerclient :) We recently moved the `vendor` directory to `external` so that go-dockerclient
is compatible with this configuration. See [338](https://github.com/fsouza/go-dockerclient/issues/338) and [339](https://github.com/fsouza/go-dockerclient/pull/339)
for details.

## Examples

See EXAMPLES.md.

## Developing

All development commands can be seen in the [Makefile](Makefile).

Commited code must pass:

* [golint](https://github.com/golang/lint)
* [go vet](https://godoc.org/golang.org/x/tools/cmd/vet)
* [gofmt](https://golang.org/cmd/gofmt)
* [go test](https://golang.org/cmd/go/#hdr-Test_packages)

Running `make test` will check all of these. If your editor does not automatically call gofmt, `make fmt` will format all go files in this repository.
