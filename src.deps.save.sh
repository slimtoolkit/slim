#!/usr/bin/env bash

set -e

source env.sh
cd src/app
godep save
rm -rfv Godeps/_workspace

