#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here

#eval "$(docker-machine env default)"
docker-slim build --http-probe --network node_compose_default --show-clogs my/node-compose-service


