#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here

#eval "$(docker-machine env default)"
docker run -it --rm --name="python_app_standard" -p 9000:9000 my/sample-python-app-standard



