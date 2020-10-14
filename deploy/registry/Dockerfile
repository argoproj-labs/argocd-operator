# upstream-registry-builder v1.13.4
FROM quay.io/operator-framework/upstream-registry-builder@sha256:ec1f9ec32f0d011b4fae7ac765ab85cf7eb84fc866f5540c9747a4cbe3688d65 as builder

COPY manifests manifests
RUN ./bin/initializer -o ./bundles.db

FROM scratch
COPY --from=builder /build/bundles.db /bundles.db
COPY --from=builder /build/bin/registry-server /registry-server
COPY --from=builder /bin/grpc_health_probe /bin/grpc_health_probe
EXPOSE 50051
ENTRYPOINT ["/registry-server"]
CMD ["--database", "bundles.db"]
