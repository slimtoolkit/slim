#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here
docker rmi my/golang-app-alpine .
docker rmi my/golang-app-alpine.slim .