#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here

eval "$(docker-machine env default)"
docker run -it --rm --name="java_app" -p 8080:8080 my/java-app



