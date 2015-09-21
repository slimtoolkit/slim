#!/usr/bin/env bash

set -e

source env.sh
cd src/slim
eval "$(docker-machine env default)"
go run main.go





