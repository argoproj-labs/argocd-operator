FROM docker.io/library/golang:1.20 as builder

WORKDIR /go/src/github.com/adnanh/webhook
RUN git clone --depth 1 -b 2.8.1 https://github.com/adnanh/webhook.git ./ && \
    go build -o server github.com/adnanh/webhook

FROM docker.io/library/ubuntu:22.04

WORKDIR /var/webhook
COPY hooks.yaml /var/webhook/hooks.yaml
COPY --from=builder /go/src/github.com/adnanh/webhook/server /var/webhook/server
RUN chmod +x /var/webhook/server

CMD ["/var/webhook/server", "-hooks", "/var/webhook/hooks.yaml", "-verbose"]
