#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here

#eval "$(docker-machine env default)"
docker run --rm -it --name="elixir_phx_app" -p 16000:16000 my/elixir-phx-app
