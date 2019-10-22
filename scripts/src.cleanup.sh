#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
SDIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

source $SDIR/env.sh
rm -rfv $BDIR/_gopath
rm -rfv $BDIR/dist_linux
rm -fv $BDIR/dist_linux.tar.gz
rm -rfv $BDIR/dist_linux_arm
rm -fv $BDIR/dist_linux_arm.tar.gz
rm -rfv $BDIR/dist_mac
rm -fv $BDIR/dist_mac.zip
rm -rfv $BDIR/bin