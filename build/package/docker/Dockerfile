# Stage 1: Build CA certificates
FROM alpine:latest AS ca-certs
LABEL build-role="ca-certs"

# Use modern best practice: avoid apk update/upgrade
RUN apk add --no-cache ca-certificates && update-ca-certificates

# Final minimal image
FROM scratch

# OCI recommended metadata
LABEL app="slim"
LABEL org.opencontainers.image.title="docker-slim"
LABEL org.opencontainers.image.description="DockerSlim runtime image containing the slim binary and required certificates"
LABEL org.opencontainers.image.source="https://github.com/docker-slim/docker-slim"

# Workdir for clarity (optional but cleaner)
WORKDIR /bin

# Copy CA certificates
COPY --from=ca-certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy the slim binary package
COPY dist_linux /bin

# Copy DockerSlim container metadata file
COPY build/package/docker/.ds.container.d3e2c84f976743bdb92a7044ef12e381 /.ds.container.d3e2c84f976743bdb92a7044ef12e381

# Slim keeps state here
VOLUME /bin/.slim-state

ENTRYPOINT ["/bin/slim"]
