# Webhook Server

This directory contains source for webhook server used in argocd-operator testing. The server is built using [adnanh/webhook](https://github.com/adnanh/webhook).

## Multiarch Container Image

Use `docker-build-push.sh` shell script to build & push multiarch container image. It uses `docker buildx` to build image for amd64, arm64, ppc64le & s390x architecture. Before you run below script, ensure you have push access to the image registry referenced in script.

```bash
./docker-build-push.sh
```

## Local Build (for development)

Use build command to build image for debugging/testing changes to this image.

```bash
docker build -t quay.io/svghadi/webhook-server:latest -f Dockerfile .
```