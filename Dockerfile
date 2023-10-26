# Build the manager binary
<<<<<<< HEAD
FROM golang:1.18 as builder
=======
FROM golang:1.20 as builder
>>>>>>> e9c2c2c (resolve merge conflicts (#1024))

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY common/ common/
COPY controllers/ controllers/
COPY version/ version/

# Build
ARG LD_FLAGS
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$LD_FLAGS" -a -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .

# install grafana artifacts
COPY grafana /var/lib/grafana

# install redis artifacts
COPY build/redis /var/lib/redis

USER 65532:65532

ENTRYPOINT ["/manager"]
