#!/usr/bin/env bash

set -e

source env.sh
pushd _vendor
go get github.com/franela/goreq
go get github.com/gdamore/mangos
go get github.com/cloudimmunity/go-dockerclientx
go get github.com/dustin/go-humanize
go get -d bitbucket.org/madmo/fanotify
go get github.com/cloudimmunity/pdiscover
go get github.com/codegangsta/cli
go get github.com/Sirupsen/logrus
popd
#go get -v ./...