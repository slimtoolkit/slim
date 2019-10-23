default: build_in_container

build_in_container:
	rm -rfv bin
	'$(CURDIR)/scripts/docker-slim-builder.run.sh'

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
