#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here

#eval "$(docker-machine env default)"
docker run -d --name="node_app_alpine" -p 8000:8000 my/sample-node-app-alpine



