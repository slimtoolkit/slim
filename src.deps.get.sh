#!/usr/bin/env bash

set -e

source env.sh
cd _vendor
go get github.com/fsouza/go-dockerclient
go get github.com/dustin/go-humanize
go get -d bitbucket.org/madmo/fanotify
go get github.com/cloudimmunity/pdiscover
