FROM golang:latest
RUN mkdir -p /go/src/github.com/docker-slim/docker-slim
ADD . /go/src/github.com/docker-slim/docker-slim
WORKDIR /go/src/github.com/docker-slim/docker-slim/cmd/docker-slim
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o docker-slim .

WORKDIR /go/src/github.com/docker-slim/docker-slim/cmd/docker-slim-sensor
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o docker-slim-sensor .
