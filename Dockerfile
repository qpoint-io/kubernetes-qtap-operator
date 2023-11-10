# Install ca-certs on ubuntu
FROM ubuntu as ubuntu-ca-base
RUN apt-get update && \
    apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Fedora includes ca-certs by default
FROM fedora as fedora-ca-base

# Alpine includes ca-certs by default
FROM alpine as alpine-ca-base

# Build the manager binary
FROM golang:1.21 as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/

# Copy the ca-certificates from the base distros
COPY --from=ubuntu-ca-base /etc/ssl/certs/ca-certificates.crt api/v1/assets/ubuntu-ca-certificates.crt
COPY --from=fedora-ca-base /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem api/v1/assets/fedora-ca-bundle.crt
COPY --from=alpine-ca-base /etc/ssl/certs/ca-certificates.crt api/v1/assets/alpine-cert.pem

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
