TARGET_PLATFORM_DEV=$(shell go env GOOS)_$(shell go env GOARCH)

default: build_in_docker ## build docker-slim in docker by default

build_in_docker:  ## build docker-slim for all supported architectures, in docker
	rm -rfv bin
	'$(CURDIR)/scripts/docker-builder.run.sh'

build_in_docker_dev:  ## build docker-slim for the current platform in bin/, in docker
	rm -rfv bin
	TARGET_PLATFORMS=$(TARGET_PLATFORM_DEV) '$(CURDIR)/scripts/docker-builder.run.sh'

build:  ## build docker-slim for all supported architectures
	'$(CURDIR)/scripts/src.build.sh'

build_dev:  ## build docker-slim for the current platform in bin/
	TARGET_PLATFORMS=$(TARGET_PLATFORM_DEV) '$(CURDIR)/scripts/src.build.sh'

fmt:  ## format all golang files
	'$(CURDIR)/scripts/src.fmt.sh'

help: ## prints out the menu of command options
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

inspect: ## report suspicious constructs and linting errors
	'$(CURDIR)/scripts/src.inspect.sh'

tools: ## install necessary tools
	'$(CURDIR)/scripts/tools.get.sh'

clean: ## clean up
	'$(CURDIR)/scripts/src.cleanup.sh'

.PHONY: default help build_in_docker build_m1_in_docker build build_m1 build_dev fmt inspect tools clean
