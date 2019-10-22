#NAME = docker-slim/docker-slim
#INSTANCE = docker-slim
default: build_in_container

build_in_container:
	rm -rfv bin
	'$(CURDIR)/scripts/docker-slim-builder.run.sh'
	#docker rm -f $(INSTANCE); true
	#docker build -f Dockerfile-build -t $(NAME)-build .
	#docker create --name $(INSTANCE) $(NAME)-build
	#docker cp $(INSTANCE):/go/src/github.com/$(NAME)/cmd/docker-slim/docker-slim $(shell pwd)/docker-slim
	#docker cp $(INSTANCE):/go/src/github.com/$(NAME)/cmd/docker-slim-sensor/docker-slim-sensor $(shell pwd)/docker-slim-sensor
	#docker rm $(INSTANCE)

build_prep:
	'$(CURDIR)/scripts/src.prep.sh'

build_run:
	'$(CURDIR)/scripts/src.build.sh'

fmt:
	'$(CURDIR)/scripts/src.fmt.sh'

inspect:
	'$(CURDIR)/scripts/src.inspect.sh'

tools:
	'$(CURDIR)/scripts/tools.get.sh'

clean:
	'$(CURDIR)/scripts/src.cleanup.sh'

.PHONY: default build_in_container build_prep build_run fmt inspect tools clean
