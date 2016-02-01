#!/usr/bin/env bash

set -e

source env.sh
pushd $BDIR/_vendor
mkdir -p src/github.com/cloudimmunity
ln -sf $BDIR src/github.com/cloudimmunity/docker-slim
go get github.com/cloudimmunity/go-dockerclientx
go get github.com/cloudimmunity/pdiscover
go get github.com/cloudimmunity/system
go get github.com/codegangsta/cli
go get github.com/Sirupsen/logrus
go get github.com/franela/goreq
go get github.com/go-mangos/mangos
go get github.com/dustin/go-humanize
go get -d bitbucket.org/madmo/fanotify
popd
pushd $BDIR/apps/docker-slim
gox -osarch="linux/amd64" -output="$BDIR/bin/linux/docker-slim"
gox -osarch="darwin/amd64" -output="$BDIR/bin/mac/docker-slim"
popd
pushd $BDIR/apps/docker-slim-sensor
gox -osarch="linux/amd64" -output="$BDIR/bin/linux/docker-slim-sensor"
popd
