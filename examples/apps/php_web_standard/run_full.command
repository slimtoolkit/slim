#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here
docker run --rm -it -p 8000:8000 my/php-web-app
