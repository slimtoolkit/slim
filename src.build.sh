#!/usr/bin/env bash

set -e

source env.sh
pushd src/app
gox -osarch="linux/amd64" -output="../../bin/dockerslim_linux"
gox -osarch="darwin/amd64" -output="../../bin/dockerslim_mac"
popd
pushd src/launcher
gox -osarch="linux/amd64" -output="../../bin/alauncher_linux"
gox -osarch="darwin/amd64" -output="../../bin/alauncher_mac"
popd
pushd src/monitor
gox -osarch="linux/amd64" -output="../../bin/amonitor_linux"
gox -osarch="darwin/amd64" -output="../../bin/amonitor_mac"
popd




