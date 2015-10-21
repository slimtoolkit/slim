FROM golang:1.4

RUN go get github.com/mitchellh/gox && \
	gox -build-toolchain -os="linux" -os="darwin"



