#!/usr/bin/env bash

set -e

source env.sh
cd $BDIR/src/slim
godep restore
