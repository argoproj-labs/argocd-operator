# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.21 as builder

ARG TARGETOS TARGETARCH

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
COPY common/ common/
COPY controllers/ controllers/
COPY version/ version/

# Build
ARG LD_FLAGS
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="$LD_FLAGS" -a -o manager cmd/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .

# install redis artifacts
COPY build/redis /var/lib/redis

USER 65532:65532

ENTRYPOINT ["/manager"]
