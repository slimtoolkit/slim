#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
BDIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

rm -rfv $BDIR/_gopath
rm -rfv $BDIR/dist_linux
rm -fv $BDIR/dist_linux.tar.gz
rm -rfv $BDIR/dist_linux_arm
rm -fv $BDIR/dist_linux_arm.tar.gz
rm -rfv $BDIR/dist_linux_arm64
rm -fv $BDIR/dist_linux_arm64.tar.gz
rm -rfv $BDIR/dist_mac
rm -fv $BDIR/dist_mac.zip
rm -rfv $BDIR/dist_mac_m1
rm -fv $BDIR/dist_mac_m1.zip
rm -rfv $BDIR/bin
