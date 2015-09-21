#!/usr/bin/env bash

set -e

source env.sh
pushd src/slim
godep save
rm -rfv Godeps/_workspace
popd
pushd src/launcher
godep save
rm -rfv Godeps/_workspace
popd
pushd src/monitor
godep save
rm -rfv Godeps/_workspace
popd
pushd src/scanner
godep save
rm -rfv Godeps/_workspace
popd