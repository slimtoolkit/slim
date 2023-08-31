default: build_in_docker ## build docker-slim in docker by default

build_in_docker:   ## build docker-slim in docker
	rm -rfv bin
	'$(CURDIR)/scripts/docker-builder.run.sh'

build_m1_in_docker:
	rm -rfv bin
	'$(CURDIR)/scripts/docker-builder-m1.run.sh'

build:  ## build docker-slim
	'$(CURDIR)/scripts/src.build.sh'

build_m1:  ## build docker-slim
	'$(CURDIR)/scripts/src.build.m1.sh'

build_dev:  ## build docker-slim for development (quickly), in bin/
	'$(CURDIR)/scripts/src.build.quick.sh'

fmt:  ## format all golang files
	'$(CURDIR)/scripts/src.fmt.sh'

help: ## prints out the menu of command options
	@awk -F ':.*?## ' '/^[a-zA-Z0-9_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

inspect: ## report suspicious constructs and linting errors
	'$(CURDIR)/scripts/src.inspect.sh'

tools: ## install necessary tools
	'$(CURDIR)/scripts/tools.get.sh'

## run unit tests
test: export GO_TEST_FLAGS ?=
test:
	'$(CURDIR)/scripts/src.test.sh'

clean: ## clean up
	'$(CURDIR)/scripts/src.cleanup.sh'

include $(CURDIR)/test/e2e-tests.mk

.PHONY: default help build_in_docker build_m1_in_docker build build_m1 build_dev fmt inspect tools test clean
