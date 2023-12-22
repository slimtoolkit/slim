ARCH ?= $(shell uname -m)
DSLIM_EXAMPLES_DIR ?= '$(CURDIR)/../examples'

GO_TEST_FLAGS =  # E.g.: make test-e2e-sensor GO_TEST_FLAGS='-run TestXyz'

# run sensor only e2e tests
test-e2e-sensor:
	go generate github.com/slimtoolkit/slim/pkg/appbom
	go test -v -tags e2e -count 5 -timeout 30m $(GO_TEST_FLAGS) $(CURDIR)/pkg/app/sensor

# run all e2e tests at once
.PHONY:
test-e2e-all: test-e2e-compose
test-e2e-all: test-e2e-distroless
test-e2e-all: test-e2e-dotnet
test-e2e-all: test-e2e-elixir
test-e2e-all: test-e2e-golang
test-e2e-all: test-e2e-haskell
test-e2e-all: test-e2e-http-probe
test-e2e-all: test-e2e-image-edit
test-e2e-all: test-e2e-java
test-e2e-all: test-e2e-node
test-e2e-all: test-e2e-php
test-e2e-all: test-e2e-python
test-e2e-all: test-e2e-ruby
test-e2e-all: test-e2e-rust
test-e2e-all:
	@echo "OK"

.PHONY:
test-e2e-compose:
	make -f $(DSLIM_EXAMPLES_DIR)/node_compose/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/node_redis_compose/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/vuejs-compose/Makefile test-e2e

.PHONY:
test-e2e-distroless:
	make -f $(DSLIM_EXAMPLES_DIR)/distroless/nodejs/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/distroless/python2.7/Makefile test-e2e

.PHONY:
test-e2e-dotnet:
	[ "${ARCH}" = "arm64" ] || make -f $(DSLIM_EXAMPLES_DIR)/dotnet_alpine/Makefile test-e2e
	[ "${ARCH}" = "arm64" ] || make -f $(DSLIM_EXAMPLES_DIR)/dotnet_aspnetcore_ubuntu/Makefile test-e2e
	[ "${ARCH}" = "arm64" ] || make -f $(DSLIM_EXAMPLES_DIR)/dotnet_debian/Makefile test-e2e
	[ "${ARCH}" = "arm64" ] || make -f $(DSLIM_EXAMPLES_DIR)/dotnet_ubuntu/Makefile test-e2e

.PHONY:
test-e2e-elixir:
	true || make -f $(DSLIM_EXAMPLES_DIR)/elixir_phx_standard/Makefile test-e2e  # TODO: Broken one!

.PHONY:
test-e2e-golang:
	make -f $(DSLIM_EXAMPLES_DIR)/golang_alpine/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/golang_centos/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/golang_gin_standard/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/golang_standard/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/golang_ubuntu/Makefile test-e2e

.PHONY:
test-e2e-haskell:
	make -f $(DSLIM_EXAMPLES_DIR)/haskell_scotty_standard/Makefile test-e2e

.PHONY:
test-e2e-http-probe:
	make -f $(DSLIM_EXAMPLES_DIR)/http_probe_cmd_file/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/http_probe_swagger/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/http_probe_swagger_http2_https/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/http_probe_swagger_http2_plain/Makefile test-e2e

.PHONY:
test-e2e-image-edit:
	make -f $(DSLIM_EXAMPLES_DIR)/image_edit_basic/Makefile test-e2e

.PHONY:
test-e2e-java:
	make -f $(DSLIM_EXAMPLES_DIR)/java_corretto/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/java_standard/Makefile test-e2e

.PHONY:
test-e2e-node:
	make -f $(DSLIM_EXAMPLES_DIR)/node17_express_yarn_standard/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/node_alpine/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/node_ubuntu/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/node_ubuntu_focal/Makefile test-e2e

.PHONY:
test-e2e-php:
	make -f $(DSLIM_EXAMPLES_DIR)/php7_fpm_fastcgi/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/php7_fpm_nginx/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/php7_builtin_web_server/Makefile test-e2e

.PHONY:
test-e2e-python:
	make -f $(DSLIM_EXAMPLES_DIR)/python2_flask_alpine/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/python2_flask_standard/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/python_ubuntu_18.04_user/Makefile test-e2e
	make -f $(DSLIM_EXAMPLES_DIR)/python_ubuntu_18_py27_from_dockerfile/Makefile test-e2e

.PHONY:
test-e2e-ruby:
	[ "${ARCH}" = "arm64" ] || make -f $(DSLIM_EXAMPLES_DIR)/ruby2_rails5_alpine/Makefile test-e2e
	[ "${ARCH}" = "arm64" ] || make -f $(DSLIM_EXAMPLES_DIR)/ruby2_rails5_alpine_puma/Makefile test-e2e
	[ "${ARCH}" = "arm64" ] || make -f $(DSLIM_EXAMPLES_DIR)/ruby2_rails5_alpine_puma_sh/Makefile test-e2e
	[ "${ARCH}" = "arm64" ] || make -f $(DSLIM_EXAMPLES_DIR)/ruby2_rails5_alpine_unicorn_rails/Makefile test-e2e
	[ "${ARCH}" = "arm64" ] || make -f $(DSLIM_EXAMPLES_DIR)/ruby2_rails5_standard/Makefile test-e2e
	[ "${ARCH}" = "arm64" ] || make -f $(DSLIM_EXAMPLES_DIR)/ruby2_rails5_standard_puma/Makefile test-e2e
	[ "${ARCH}" = "arm64" ] || make -f $(DSLIM_EXAMPLES_DIR)/ruby2_sinatra_ubuntu/Makefile test-e2e

.PHONY:
test-e2e-rust:
	[ "${GITHUB_ACTIONS}" = "true" ] || make -f $(DSLIM_EXAMPLES_DIR)/rust_standard/Makefile test-e2e  # TODO: HTTP probe always fails on CI - need to investigate more.
