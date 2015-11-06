#!/usr/bin/env bash

set -e

source env.sh
cd src
go tool vet .
golint ./...





