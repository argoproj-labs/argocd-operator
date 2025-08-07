# upstream-registry-builder v1.26.2
FROM quay.io/operator-framework/upstream-registry-builder@sha256:70043025cb6b3cdce4800140428de4a18726aa93898bedf97fcd371d3240d9ee

COPY manifests manifests
RUN ./bin/initializer -o ./bundles.db

EXPOSE 50051
ENTRYPOINT ["/bin/registry-server"]
CMD ["--database", "/bundles.db"]
