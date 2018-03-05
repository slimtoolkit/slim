#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here
docker-slim --debug build --http-probe --show-clogs --show-blogs my/php-web-app
