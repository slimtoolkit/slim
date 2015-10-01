# docker-slim: Make Your Fat Containers Skinny

[Docker Global Hack Day \#dockerhackday](https://www.docker.com/community/hackathon) project (status: ACTIVE!)

Just because the hack day is over doesn't mean the project is done :-) The project needs your help even if you don't know Docker or Go!

IRC (freenode): \#dockerslim

## DESCRIPTION


Creating small containers requires a lot of voodoo magic and it can be pretty painful. You shouldn't have to throw away your tools and your workflow to have skinny containers. Using Docker should be easy. 

`docker-slim` is a magic diet pill for your containers :) It will use static and dynamic analysis to create a skinny container for your app.


### BASIC USE CASE

1. Build a "fat"/unoptimized container for a python/ruby/node web app using a standard Dockerfile (with Ubuntu 14.04)
2. Run DockerSlim to generate an optimized container

## CURRENT STATE

You can pass an image ID/name to `dockerslim` and it'll make a smaller image from it. The `sample_app` is 430MB and `dockerslim` turns it into 40MB.

You can also run `dockerslim` in the `info` mode and it'll generate useful image information including a "reverse engineered" Dockerfile.

Dependencies:

To run `docker-slim` you need to export docker environment variables. If you use `docker-machine` you get it when you run `eval "$(docker-machine env default)"`.

## USAGE

`./dockerslim <IMAGE_ID> [rm-artifacts | image-info-only]`

Example: `./dockerslim 6f74095b68c9`

By default, `dockerslim` doesn't remove the artifacts it generates. To remove them set the `rm-artifacts` flag.

Example: `./dockerslim 6f74095b68c9 rm-artifacts`

To generate a Dockerfile for your "fat" image without creating a new "slim" image set the `image-info-only` flag.

Example: `./dockerslim 6f74095b68c9 image-info-only`

## PHASE 1 (DONE)

Goal: build basic infrastructure

Create the "slim" app that:

*  collects basic container image metadata [DONE]
*  "reverse engineers" the Dockerfile used to create the target image [DONE]
*  creates a container replacing/hooking the original entrypoint/cmd [DONE]
*  creates a new "slim" image from the collected information and artifacts [DONE]

Create the "slim" launcher that:

* starts the original application (based on the original entrypoint/cmd data) [DONE]
* monitors process activity (saving events in a log file) [DONE] (note: doesn't work with all kernels)
* monitors file activity (saving events in a log file) [DONE]

## PHASE 2 (DONE)

* Fix new image permission errors [DONE]
* Use env data from the original image [DONE]

## MILESTONE 1 - MINIFIED TEST DOCKER IMAGE (DONE)

The minified `sample_app` docker image now works! We turned a 430MB node.js app container into a 40MB image.

## PHASE 3

Make sure it works with other images.

Do a better job with links.

Split "monitor" from "launcher" (as it's supposed to work :-))

Add scripting language dependency discovery to the "scanner" app.

Support additional command line parameters to specify CMD, VOLUME, ENV info.

Build/use a custom Boot2docker kernel with every required feature turned on.

Explore additional dependency discovery methods.

"Live" image create mode - to create new images from containers where users install their applications interactively.

## HOW

1. Inspect container metadata (static analysis)
2. Inspect container data (static analysis)
3. Inspect running application (dynamic analysis)
4. Build an application artifact graph

## DYNAMIC ANALYSIS OPTIONS

1. Instrument the container image (and replace the entrypoint/cmd) to collect application activity data
2. Use kernel-level tools that provide visibility into running containers (without instrumenting the containers)
3. Disable relevant namespaces in the target container to gain container visibility (can be done with runC)

## CHALLENGES

Some of the advanced analysis options require a number of Linux kernel features that are not always included. The kernel you get with Docker Machine / Boot2docker is a great example of that.

## DEMO STEPS

1. Create a Docker image for `sample_app` (it's a `hello world` node.js app that handles GET / requests).
2. Run `dockerslim` from the `dist` directory.
3. Use curl (or other tools) to call the `sample_app` (optional)
4. Wait a couple of minutes until `dockerslim` says it's done
5. `dist/container/artifacts` will contain files and metadata to build a new image.
6. The generated `slim` image will have `.slim` at the end of its name.
7. If you selected the `image-info-only` feature you'll find `Dockerfile.fat` in `./container/artifacts/`.

## BUILD DEPENDENCIES

* Godep - dependency manager ( https://github.com/tools/godep )

`go get github.com/tools/godep`

* GOX - to build Linux binaries on a Mac ( https://github.com/mitchellh/gox ):

`go get github.com/mitchellh/gox`

`gox -build-toolchain -os="linux" -os="darwin"` (note:  might have to run it with `sudo`)

## NOTES

1. The code is really really ugly at this point in time :)
2. Each app directory contains a dummy `.git` directory because `godep` fails to work without it.






