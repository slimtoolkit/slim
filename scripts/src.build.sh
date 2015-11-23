#!/usr/bin/env bash

set -e

source env.sh
pushd $BDIR/apps/docker-slim
#gox -osarch="linux/amd64" -output="$BDIR/bin/linux/docker-slim"
gox -osarch="darwin/amd64" -output="$BDIR/bin/mac/docker-slim"
popd
pushd $BDIR/apps/docker-slim-sensor
gox -osarch="linux/amd64" -output="$BDIR/bin/linux/docker-slim-sensor"
popd
rm -rfv $BDIR/dist_mac
mkdir $BDIR/dist_mac
cp $BDIR/bin/mac/docker-slim $BDIR/dist_mac/docker-slim
cp $BDIR/bin/linux/docker-slim-sensor $BDIR/dist_mac/docker-slim-sensor
