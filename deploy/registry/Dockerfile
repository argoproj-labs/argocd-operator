# upstream-registry-builder v1.26.2
FROM quay.io/operator-framework/upstream-registry-builder@sha256:dc3ef48c8f6d90ea9654cd2ac5e32afbb777d6395b29f55f9976b201b62e4145

COPY manifests manifests
RUN ./bin/initializer -o ./bundles.db

EXPOSE 50051
ENTRYPOINT ["/bin/registry-server"]
CMD ["--database", "/bundles.db"]
