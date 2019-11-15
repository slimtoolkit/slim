#!/usr/bin/env bash

set -e

docker build -t docker-slim -f Dockerfile ../../..

