#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here
docker rmi my/php-web-app .
docker rmi my/php-web-app.slim .