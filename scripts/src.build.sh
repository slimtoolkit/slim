#!/usr/bin/env bash

set -e

source env.sh
pushd $BDIR/apps/docker-slim
gox -osarch="linux/amd64" -output="$BDIR/bin/linux/docker-slim"
gox -osarch="darwin/amd64" -output="$BDIR/bin/mac/docker-slim"
#gox -osarch="linux/arm" -output="$BDIR/bin/linux_arm/docker-slim"
popd
pushd $BDIR/apps/docker-slim-sensor
gox -osarch="linux/amd64" -output="$BDIR/bin/linux/docker-slim-sensor"
#gox -osarch="linux/arm" -output="$BDIR/bin/linux_arm/docker-slim-sensor"
popd
rm -rfv $BDIR/dist_mac
mkdir $BDIR/dist_mac
cp $BDIR/bin/mac/docker-slim $BDIR/dist_mac/docker-slim
cp $BDIR/bin/linux/docker-slim-sensor $BDIR/dist_mac/docker-slim-sensor
rm -rfv $BDIR/dist_linux
mkdir $BDIR/dist_linux
cp $BDIR/bin/linux/docker-slim $BDIR/dist_linux/docker-slim
cp $BDIR/bin/linux/docker-slim-sensor $BDIR/dist_linux/docker-slim-sensor
#rm -rfv $BDIR/dist_linux_arm
#mkdir $BDIR/dist_linux_arm
#cp $BDIR/bin/linux_arm/docker-slim $BDIR/dist_linux_arm/docker-slim
#cp $BDIR/bin/linux_arm/docker-slim-sensor $BDIR/dist_linux_arm/docker-slim-sensor
rm -rfv $BDIR/bin
