# docker-slim: Make Your Fat Containers Skinny

[Docker Global Hack Day](https://www.docker.com/community/hackathon) project (status: STARTED!)

## Description


Creating small containers requires a lot of voodoo magic and it can be pretty painful. You shouldn't have to throw away your tools and your workflow to have skinny containers. Using Docker should be easy. 

`docker-slim` is a magic diet pill for your containers :) It will use static and dynamic analysis to create a skinny container for your app.

## Goals

Build something useful :-)


### Basic Use Case

1. Build a "fat"/unoptimized container for a python/ruby/node web app using a standard Dockerfile (with Ubuntu 14.04)
2. Run DockerSlim to generate an optimized container

## How

1. Inspect container metadata (static analysis)
2. Inspect container data (static analysis)
3. Inspect running application (dynamic analysis)
4. Build an application artifact graph

## Dynamic Analysis Options

1. Instrument the container image (and replace the entrypoint/cmd) to collect application activity data
2. Use kernel-level tools that provide visibility into running containers (without instrumenting the containers)
3. Disable relevant namespaces in the target container to gain container visibility (can be done with runC)

## Phase 1

Goal: build basic infrastructure

Create the "slim" app that:

*  collects basic container image metadata
*  create a custom image replacing/hooking the original entrypoint/cmd

Create the "slim" launcher that:

* starts the original application (based on the original entrypoint/cmd data)
* monitors process activity (saving events in a log file)
* monitors file activity (saving events in a log file)

Explore additional dependency discovery methods


## People

* [Dmitriy Vorobyev](https://github.com/pydima)
* [Larry Hitchon](https://github.com/lhitchon)
* [Kyle Quest](https://github.com/kcq)

## Communication

* IRC (freenode): `#dokerslim`
* Slack: `http://dockerslim.slack.com`
* Github issues and wiki `https://github.com/cloudimmunity/docker-slim`








