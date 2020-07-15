# upstream-registry-builder v1.13.1
FROM quay.io/operator-framework/upstream-registry-builder@sha256:8f5db34a1bf254d4f664e280c437e4d708eb54ded5b61597fa267d533ca394c0

COPY manifests manifests
RUN ./bin/initializer -o ./bundles.db

FROM scratch
COPY --from=builder /build/bundles.db /bundles.db
COPY --from=builder /build/bin/registry-server /registry-server
COPY --from=builder /bin/grpc_health_probe /bin/grpc_health_probe
EXPOSE 50051
ENTRYPOINT ["/registry-server"]
CMD ["--database", "bundles.db"]
