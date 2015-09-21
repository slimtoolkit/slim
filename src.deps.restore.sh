#!/usr/bin/env bash

set -e

source env.sh
cd src/slim
godep restore
