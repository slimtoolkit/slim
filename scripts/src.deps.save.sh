#!/usr/bin/env bash

set -e

source env.sh
pushd $BDIR/src/slim
godep save
rm -rfv Godeps/_workspace
popd
pushd $BDIR/src/launcher
godep save
rm -rfv Godeps/_workspace
popd
pushd $BDIR/src/monitor
godep save
rm -rfv Godeps/_workspace
popd
pushd $BDIR/src/scanner
godep save
rm -rfv Godeps/_workspace
popd