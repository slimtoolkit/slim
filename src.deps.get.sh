#!/usr/bin/env bash

set -e

source env.sh
cd _vendor
go get github.com/fsouza/go-dockerclient

