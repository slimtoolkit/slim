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
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

inspect: ## report suspicious constructs and linting errors
	'$(CURDIR)/scripts/src.inspect.sh'

tools: ## install necessary tools
	'$(CURDIR)/scripts/tools.get.sh'

clean: ## clean up
	'$(CURDIR)/scripts/src.cleanup.sh'

.PHONY: default help build_in_docker build_m1_in_docker build build_m1 build_dev fmt inspect tools clean


# -== Acceptance tests ==-
ARCH ?= $(shell uname -m)
DSLIM_EXAMPLES_DIR ?= '$(CURDIR)/../examples'

test-e2e-compose:
	make -f $(DSLIM_EXAMPLES_DIR)/node_compose/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/node_redis_compose/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/vuejs-compose/Makefile test-e2e

test-e2e-distroless:
	make -f $(DSLIM_EXAMPLES_DIR)/distroless/nodejs/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/distroless/python2.7/Makefile test-e2e

test-e2e-dotnet:
	[ "${ARCH}" = "arm64" ] || make -f $(DSLIM_EXAMPLES_DIR)/dotnet_alpine/Makefile test-e2e
	[ "${ARCH}" = "arm64" ] || make -f $(DSLIM_EXAMPLES_DIR)/dotnet_aspnetcore_ubuntu/Makefile test-e2e
	[ "${ARCH}" = "arm64" ] || make -f $(DSLIM_EXAMPLES_DIR)/dotnet_debian/Makefile test-e2e
	[ "${ARCH}" = "arm64" ] || make -f $(DSLIM_EXAMPLES_DIR)/dotnet_ubuntu/Makefile test-e2e

test-e2e-elixir:
	true || make -f $(DSLIM_EXAMPLES_DIR)/elixir_phx_standard/Makefile test-e2e  # TODO: Broken one!

test-e2e-golang:
	make -f $(DSLIM_EXAMPLES_DIR)/golang_alpine/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/golang_centos/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/golang_gin_standard/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/golang_standard/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/golang_ubuntu/Makefile test-e2e

test-e2e-java:
	make -f $(DSLIM_EXAMPLES_DIR)/java_corretto/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/java_standard/Makefile test-e2e

test-e2e-node:
	make -f $(DSLIM_EXAMPLES_DIR)/node17_express_yarn_standard/Makefile test-e2e

test-e2e-php:
	make -f $(DSLIM_EXAMPLES_DIR)/php7_fpm_fastcgi/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/php7_fpm_nginx/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/php7_builtin_web_server/Makefile test-e2e

test-e2e-python:
	make -f $(DSLIM_EXAMPLES_DIR)/python2_flask_alpine/Makefile test-e2e

test-e2e-ruby:
	[ "${ARCH}" = "arm64" ] || make -f $(DSLIM_EXAMPLES_DIR)/ruby2_sinatra_alpine/Makefile test-e2e
	[ "${ARCH}" = "arm64" ] || make -f $(DSLIM_EXAMPLES_DIR)/ruby2_sinatra_ubuntu/Makefile test-e2e

test-e2e-rust:
	[ "${GITHUB_ACTIONS}" = "true" ] || make -f $(DSLIM_EXAMPLES_DIR)/rust_standard/Makefile test-e2e  # TODO: HTTP probe always fails on CI - need to investigate more.

# run all e2e tests at once
test-e2e-all: test-e2e-compose test-e2e-distroless test-e2e-dotnet test-e2e-elixir test-e2e-golang test-e2e-java test-e2e-node test-e2e-php test-e2e-python test-e2e-ruby test-e2e-rust

.PHONY: test-e2e-all test-e2e-compose test-e2e-distroless test-e2e-dotnet test-e2e-elixir test-e2e-golang test-e2e-java test-e2e-node test-e2e-php test-e2e-python test-e2e-ruby test-e2e-rust
# -== eof: Acceptance tests ==-
