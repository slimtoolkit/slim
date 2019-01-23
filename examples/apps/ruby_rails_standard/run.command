#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here

#eval "$(docker-machine env default)"
docker run -it --rm --name="ruby_rails_app" -p 3333:3333 my/ruby-rails-app



