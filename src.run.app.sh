#!/usr/bin/env bash

set -e

source env.sh
cd src/app
eval "$(docker-machine env default)"
go run main.go





