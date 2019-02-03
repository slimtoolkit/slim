#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here

#eval "$(docker-machine env default)"
docker-slim --debug build --http-probe --show-clogs my/ruby-rails-app


