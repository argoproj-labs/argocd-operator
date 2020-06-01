# upstream-registry-builder v1.12.4
FROM quay.io/operator-framework/upstream-registry-builder@sha256:8d392ab0587d5bcd19a40d60ac6bc8214e6b515605be3113edd01ea99ce95942 as builder

COPY manifests manifests
RUN ./bin/initializer -o ./bundles.db

FROM scratch
COPY --from=builder /build/bundles.db /bundles.db
COPY --from=builder /build/bin/registry-server /registry-server
COPY --from=builder /bin/grpc_health_probe /bin/grpc_health_probe
EXPOSE 50051
ENTRYPOINT ["/registry-server"]
CMD ["--database", "bundles.db"]
