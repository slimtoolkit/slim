#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here

#eval "$(docker-machine env default)"
docker run --rm -it --name="rust_service" -p 15000:15000 my/rust-service.slim
