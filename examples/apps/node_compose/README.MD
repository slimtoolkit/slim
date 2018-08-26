# Service with a Redis Data Store - Minifying Images with Dependencies

## Overview

DockerSlim relies on dynamic analysis to minify Docker images. Most real world services require additional dependencies to function properly. This is a simplified example where a service has a dependency on Redis. The provided Docker Compose file (`docker-compose.yaml`) starts the service along with its dependency.

The example shows how DockerSlim can connect to the default network created by Docker Compose when it's minifying the target service image. You need to make sure the dependencies are up and running before you try to minify the target service. This can be done by telling Docker Compose to start specific components declared in the Docker Compose file similar to how it's done in the `up_deps_only.command` helper script.

Make sure to pass the appropriate network name to DockerSlim if your Docker Compose file defines a custom network you (using the `--network` parameter).

## Helper Scripts
* `build.command` - helper script to build the service image explicitly (or you can rely on Docker Compose to do it for you).
* `up.command` - runs the original service and its Redis dependency.
* `up_deps_only.command` - starts the service dependencies (make sure you run this before you try to minify the service image).
* `slim.command` - minifies the service image.
* `up_slim.command` - runs the minified service and its Redis dependency.


