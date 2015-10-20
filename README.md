# docker-slim: Make Your Fat Containers Skinny and Tough (optimize and secure*)

[Docker Global Hack Day \#dockerhackday](https://www.docker.com/community/hackathon) project (status: ACTIVE!)

Just because the hack day is over doesn't mean the project is done :-) The project needs your help even if you don't know Docker or Go!

IRC (freenode): \#dockerslim

## DEMO VIDEO

[![DockerSlim demo](http://img.youtube.com/vi/uKdHnfEbc-E/0.jpg)](https://www.youtube.com/watch?v=uKdHnfEbc-E)

[Demo video on YouTube](https://youtu.be/uKdHnfEbc-E)

## DESCRIPTION


Creating small containers requires a lot of voodoo magic and it can be pretty painful. You shouldn't have to throw away your tools and your workflow to have skinny containers. Using Docker should be easy. 

`docker-slim` is a magic diet pill for your containers :) It will use static and dynamic analysis to create a skinny container for your app.

## CURRENT STATE

It WORKS with the sample node.js, python, and ruby images (built from `sample_apps`). More testing needs to be done to see how it works with other images.

Sample images (built with the standard Ubuntu 14.04 base image):

* nodejs app container: 431.7 MB => 14.22 MB
* python app container: 433.1 MB => 15.97 MB
* ruby app container:   406.2 MB => 13.66 MB
* java app container:   743.6 MB => 100.3 MB (yes, it's a bit bigger than others :-))

You can also run `docker-slim` in the `info` mode and it'll generate useful image information including a "reverse engineered" Dockerfile.

DockerSlim now also generates an AppArmor profile for your container.

Dependencies:

To run `docker-slim` you need to export docker environment variables. If you use `docker-machine` you get it when you run `eval "$(docker-machine env default)"`.

## USAGE

`./docker-slim <IMAGE_ID_OR_NAME> [rm-artifacts | image-info-only]`

Example: `./docker-slim 6f74095b68c9`

By default, `docker-slim` doesn't remove the artifacts it generates. To remove them set the `rm-artifacts` flag.

Example: `./docker-slim 6f74095b68c9 rm-artifacts`

To generate a Dockerfile for your "fat" image without creating a new "slim" image set the `image-info-only` flag.

Example: `./docker-slim 6f74095b68c9 image-info-only`

## DEMO STEPS

The demo run on Mac OS X, but you can build a linux version.

1. Clone this repo

	`git clone https://github.com/cloudimmunity/docker-slim.git`
	
2. Create a Docker image for the sample node.js app in `sample_apps/node`
	
	`cd docker-slim/sample_apps/node`
	
	`eval "$(docker-machine env default)"` <- optional (depends on how Docker is installed on your machine)
	
	`docker build -t my/sample-node-app .`
	 
3. Run `docker-slim`:

	`cd ../../dist`
	
	`./docker-slim my/sample-node-app`
	
	DockerSlim creates a special container based on the target image you provided.

4. Use curl (or other tools) to call the sample app (optional)

	`curl http://<YOUR_DOCKER_HOST_IP>:<PORT>`
	
	This is an optional step to make sure the target app container is doing something. Depending on the application it's an optional step. For some applications it's required if it loads new application resources dynamically based on the requests it's processing.
		
	You can get the port number either from the `docker ps` or `docker port <CONTAINER_ID>` commands. The current version of DockerSlim doesn't allow you to map exposed network ports (it works like `docker run … -P`).

5. Wait a couple of minutes until `docker-slim` says it's done

6. Once DockerSlim is done check that the new minified image is there

	`docker images`
	
	You should see `my/sample-node-app.slim` in the list of images. Right now all generated images have `.slim` at the end of its name.

7. Use the minified image

	`docker run --name="slim_node_app" -p 8000:8000 my/sample-node-app.slim`

Notes:

You can explore the artifacts DockerSlim generates when it's creating a slim image. You'll find those in `dist/container/artifacts`. One of the artifacts is a "reverse engineered" Dockerfile for the original image. It'll be called `Dockerfile.fat`. 

If you'd like to see the artifacts without running `docker-slim` you can take a look at the `sample_artifacts` directory in the repo (it doesn't include the image files though), but you'll find:

*	a reverse engineered Dockerfile (`Dockerfile.fat`), 
*	a container report file (`creport.json`),
*	and a sample AppArmor profile (which will be named based on your original image name).

If you don't want to create a minified image and only want to "reverse engineer" the Dockerfile you can use the `image-info-only` option.

You can get the current binaries for Macs [here](https://github.com/cloudimmunity/docker-slim/releases/download/v1.4/mac_dist.zip)


## BUILD PROCESS

Before you build the tool you need to install GOX and Godep (optional; you'll need it only if you have problems pulling the dependencies the old fashioned way :-))

* Godep - dependency manager ( https://github.com/tools/godep )

1: `go get github.com/tools/godep`

* GOX - to build Linux binaries on a Mac ( https://github.com/mitchellh/gox ):

1: `go get github.com/mitchellh/gox`

2: `gox -build-toolchain -os="linux" -os="darwin"` (note:  might have to run it with `sudo`)

#### Build Steps

1: Pull the dependencies: `./src.deps.get.sh`
2: Build it: `./src.build.sh`

You can use the clickable `.command` scripts on Mac OS X:

1: `mac.src.deps.get.command`
2: `mac.src.build.command`


## DESIGN

### CORE CONCEPTS

1. Inspect container metadata (static analysis)
2. Inspect container data (static analysis)
3. Inspect running application (dynamic analysis)
4. Build an application artifact graph
5. Use the collected application data to build small images
6. Use the collected application data to auto-generate various security framework configurations.

### DYNAMIC ANALYSIS OPTIONS

1. Instrument the container image (and replace the entrypoint/cmd) to collect application activity data
2. Use kernel-level tools that provide visibility into running containers (without instrumenting the containers)
3. Disable relevant namespaces in the target container to gain container visibility (can be done with runC)

### SECURITY

The goal is to auto-generate Seccomp, AppArmor, (and potentially SELinux) profiles based on the collected information.

* AppArmor profiles (the auto-generated profiles are almost usable :-)).

### CHALLENGES

Some of the advanced analysis options require a number of Linux kernel features that are not always included. The kernel you get with Docker Machine / Boot2docker is a great example of that.


## DEVELOPMENT PROGRESS

### PHASE 1 (DONE)

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

### PHASE 2 (DONE)

* Fix new image permission errors [DONE]
* Use env data from the original image [DONE]

### MILESTONE 1 - MINIFIED TEST DOCKER IMAGE (DONE)

The minified `sample_app` docker image now works! We turned a 430MB node.js app container into a 40MB image.

### PHASE 3 (ACTIVE)

* Do a better job with links [DONE] The test image is now even smaller (was: 40MB, now: 14.22MB)
* Make sure it works with other images [WIP, now: node,python,ruby,java].
* Refactor the time-based container monitoring phase.
* Automated interaction with the target container (requires app code analysis).
* Auto-generate AppArmor profiles [WIP].
* Auto-generate Seccomp filters.
* Split "monitor" from "launcher" (as it's supposed to work :-))
* Add scripting language dependency discovery to the "scanner" app.
* Support additional command line parameters to specify CMD, VOLUME, ENV info.
* Build/use a custom Boot2docker kernel with every required feature turned on.
* Explore additional dependency discovery methods.
* "Live" image create mode - to create new images from containers where users install their applications interactively.


## NOTES

1. The code is really really ugly at this point in time :)
2. Each app directory contains a dummy `.git` directory because `godep` fails to work without it.






