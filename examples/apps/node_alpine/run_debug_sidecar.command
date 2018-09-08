#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here

docker run --rm -it --pid=container:node_app_alpine --net=container:node_app_alpine --cap-add sys_admin alpine sh

