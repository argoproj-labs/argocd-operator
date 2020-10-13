# upstream-registry-builder v1.14.3
FROM quay.io/operator-framework/upstream-registry-builder@sha256:30ac65141ceab0493f7423f966a323842c3aeca817c59a7c23fed8055856ede3 as builder

COPY manifests manifests
RUN ./bin/initializer -o ./bundles.db

FROM scratch
COPY --from=builder /build/bundles.db /bundles.db
COPY --from=builder /build/bin/registry-server /registry-server
COPY --from=builder /bin/grpc_health_probe /bin/grpc_health_probe
EXPOSE 50051
ENTRYPOINT ["/registry-server"]
CMD ["--database", "bundles.db"]
