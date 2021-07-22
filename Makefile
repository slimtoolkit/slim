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

.PHONY: default help build_in_docker build_m1_in_docker build build_m1 fmt inspect tools clean
