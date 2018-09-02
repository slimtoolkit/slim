#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here
docker rmi my/golang-app-centos
docker rmi my/golang-app-centos.slim