#!/usr/bin/env bash

set -e

source env.sh
pushd $BDIR/src/slim/app
#gox -osarch="linux/amd64" -output="$BDIR/bin/linux/docker-slim"
gox -osarch="darwin/amd64" -output="$BDIR/bin/mac/docker-slim"
popd
#pushd $BDIR/src/scanner
#gox -osarch="linux/amd64" -output="$BDIR/bin/linux/ascanner"
#gox -osarch="darwin/amd64" -output="$BDIR/bin/mac/ascanner"
#popd
pushd $BDIR/src/launcher/app
gox -osarch="linux/amd64" -output="$BDIR/bin/linux/alauncher"
popd
#pushd $BDIR/src/monitor
#gox -osarch="linux/amd64" -output="$BDIR/bin/linux/amonitor"
#popd




