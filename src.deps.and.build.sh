#!/usr/bin/env bash

set -e

source env.sh
pushd _vendor
go get github.com/cloudimmunity/go-dockerclientx
go get github.com/dustin/go-humanize
go get -d bitbucket.org/madmo/fanotify
go get github.com/cloudimmunity/pdiscover
popd
pushd src/slim
gox -osarch="linux/amd64" -output="../../bin/linux/docker-slim"
gox -osarch="darwin/amd64" -output="../../bin/mac/docker-slim"
popd
pushd src/launcher
gox -osarch="linux/amd64" -output="../../bin/linux/alauncher"
popd

#go get -v ./...