FROM golang:1.14.1-alpine AS build-env

RUN apk add --update git gcc libc-dev

ENV GO111MODULE=on

ARG MIGRATE_VER=v4.6.2
RUN go get -tags 'mysql postgres file' github.com/golang-migrate/migrate/v4/cli@${MIGRATE_VER} && mv /go/bin/cli /go/bin/migrate

ENV GOFLAGS=-mod=vendor
ENV NOTARYPKG github.com/theupdateframework/notary

# Copy the local repo to the expected go path
COPY . /go/src/${NOTARYPKG}
WORKDIR /go/src/${NOTARYPKG}

# Build notary-signer
RUN go install \
    -tags pkcs11 \
    -ldflags "-w -X ${NOTARYPKG}/version.GitCommit=`git rev-parse --short HEAD` -X ${NOTARYPKG}/version.NotaryVersion=`cat NOTARY_VERSION`" \
    ${NOTARYPKG}/cmd/notary-signer


FROM busybox:latest

# the ln is for compatibility with the docker-compose.yml, making these
# images a straight swap for the those built in the compose file.
RUN mkdir -p /usr/bin /var/lib && ln -s /bin/env /usr/bin/env

COPY --from=build-env /go/bin/notary-signer /usr/bin/notary-signer
COPY --from=build-env /go/bin/migrate /usr/bin/migrate
COPY --from=build-env /lib/ld-musl-x86_64.so.1 /lib/ld-musl-x86_64.so.1
COPY --from=build-env /go/src/github.com/theupdateframework/notary/migrations/ /var/lib/notary/migrations
COPY --from=build-env /go/src/github.com/theupdateframework/notary/fixtures /var/lib/notary/fixtures
RUN chmod 0600 /var/lib/notary/fixtures/database/*

WORKDIR /var/lib/notary
# SERVICE_NAME needed for migration script
ENV SERVICE_NAME=notary_signer
ENV NOTARY_SIGNER_DEFAULT_ALIAS="timestamp_1"
ENV NOTARY_SIGNER_TIMESTAMP_1="testpassword"

ENTRYPOINT [ "/usr/bin/notary-signer" ]
CMD [ "-config=/var/lib/notary/fixtures/signer-config-local.json" ]
