NAME = docker-slim/docker-slim
INSTANCE = docker-slim

.PHONY: default build 

default: build

build:
	docker rm -f $(INSTANCE); true
	docker build -f Dockerfile-build -t $(NAME)-build .
	docker create --name $(INSTANCE) $(NAME)-build
	docker cp $(INSTANCE):/go/src/github.com/$(NAME)/apps/docker-slim/docker-slim $(shell pwd)/docker-slim
	docker cp $(INSTANCE):/go/src/github.com/$(NAME)/apps/docker-slim-sensor/docker-slim-sensor $(shell pwd)/docker-slim-sensor
	docker rm $(INSTANCE)

