#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here
docker rmi my/golang-app-ubuntu
docker rmi my/golang-app-ubuntu.slim