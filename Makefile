default: build_in_docker

build_in_docker:
	rm -rfv bin
	'$(CURDIR)/scripts/docker-builder.run.sh'

build:
	'$(CURDIR)/scripts/src.build.sh'

fmt:
	'$(CURDIR)/scripts/src.fmt.sh'

inspect:
	'$(CURDIR)/scripts/src.inspect.sh'

tools:
	'$(CURDIR)/scripts/tools.get.sh'

clean:
	'$(CURDIR)/scripts/src.cleanup.sh'

.PHONY: default build_in_docker build fmt inspect tools clean
