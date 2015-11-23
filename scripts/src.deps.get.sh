#!/usr/bin/env bash

set -e

source env.sh
pushd $BDIR
mkdir _vendor
pushd $BDIR/_vendor
mkdir -p src/github.com/cloudimmunity
ln -sf $BDIR src/github.com/cloudimmunity/docker-slim
go get github.com/cloudimmunity/go-dockerclientx
go get github.com/cloudimmunity/pdiscover
go get github.com/cloudimmunity/system
go get github.com/franela/goreq
go get github.com/gdamore/mangos
go get github.com/dustin/go-humanize
go get -d bitbucket.org/madmo/fanotify
go get github.com/codegangsta/cli
go get github.com/Sirupsen/logrus
popd
popd
