#!/usr/bin/env bash

set -e

source env.sh
pushd src/app
gox -osarch="linux/amd64" -output="../../bin/linux/dockerslim"
gox -osarch="darwin/amd64" -output="../../bin/mac/dockerslim"
popd
pushd src/ascanner
gox -osarch="linux/amd64" -output="../../bin/linux/ascanner"
gox -osarch="darwin/amd64" -output="../../bin/mac/ascanner"
popd
pushd src/launcher
gox -osarch="linux/amd64" -output="../../bin/linux/alauncher"
popd
pushd src/monitor
gox -osarch="linux/amd64" -output="../../bin/linux/amonitor"
popd




