#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here
docker-slim --verbose build --http-probe --show-blogs my/php-web-app
