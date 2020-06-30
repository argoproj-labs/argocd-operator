# upstream-registry-builder v1.12.6
FROM quay.io/operator-framework/upstream-registry-builder@sha256:3157989c0f70da76f5358adbeda3021b3bad963500c461bd631317bdebf50d5e as builder

COPY manifests manifests
RUN ./bin/initializer -o ./bundles.db

FROM scratch
COPY --from=builder /build/bundles.db /bundles.db
COPY --from=builder /build/bin/registry-server /registry-server
COPY --from=builder /bin/grpc_health_probe /bin/grpc_health_probe
EXPOSE 50051
ENTRYPOINT ["/registry-server"]
CMD ["--database", "bundles.db"]
