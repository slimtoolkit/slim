FROM dockercore/golang-cross:1.12.15

RUN apt-get update && apt-get install -y \
	curl \
	clang \
	file \
	libsqlite3-dev \
	patch \
	tar \
	xz-utils \
	python \
	python-pip \
	--no-install-recommends \
	&& rm -rf /var/lib/apt/lists/*

RUN useradd -ms /bin/bash notary \
	&& pip install codecov \
	&& go get golang.org/x/lint/golint github.com/fzipp/gocyclo github.com/client9/misspell/cmd/misspell github.com/gordonklaus/ineffassign github.com/securego/gosec/cmd/gosec/...

ENV NOTARYDIR /go/src/github.com/theupdateframework/notary
ENV GO111MODULE=on
ENV GOFLAGS=-mod=vendor

COPY . ${NOTARYDIR}
RUN chmod -R a+rw /go

WORKDIR ${NOTARYDIR}

# Note this cannot use alpine because of the MacOSX Cross SDK: the cctools there uses sys/cdefs.h and that cannot be used in alpine: http://wiki.musl-libc.org/wiki/FAQ#Q:_I.27m_trying_to_compile_something_against_musl_and_I_get_error_messages_about_sys.2Fcdefs.h
