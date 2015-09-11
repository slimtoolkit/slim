# docker-slim: Make Your Fat Containers Skinny

[Docker Global Hack Day](https://www.docker.com/community/hackathon) project

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

## Notes

To do dynamic analysis DockerSlim will need to install a number of system monitoring tools (TBD: add info about the tools)







