#!/usr/bin/env bash

set -e

source env.sh
go get github.com/codegangsta/cli
go get github.com/Sirupsen/logrus
go get github.com/franela/goreq
go get github.com/gdamore/mangos
go get github.com/cloudimmunity/go-dockerclientx
go get github.com/dustin/go-humanize
go get -d bitbucket.org/madmo/fanotify
go get github.com/cloudimmunity/pdiscover
pushd $BDIR/src/slim/app
gox -osarch="linux/amd64" -output="$BDIR/bin/linux/docker-slim"
gox -osarch="darwin/amd64" -output="$BDIR/bin/mac/docker-slim"
popd
pushd $BDIR/src/launcher/app
gox -osarch="linux/amd64" -output="$BDIR/bin/linux/alauncher"
popd

#go get -v ./...