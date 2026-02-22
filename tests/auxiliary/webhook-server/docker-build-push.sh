#!/bin/bash

# https://docs.docker.com/build/building/multi-platform/#getting-started
# create builder instance to build multiarch image
BUIDLER=$(docker buildx create --use)

# build & push multiarch image
docker buildx build \
        --push \
        --tag quay.io/svghadi/webhook-server:latest \
        --platform linux/amd64,linux/arm64,linux/ppc64le,linux/s390x \
        -f Dockerfile .

# remove builder instance
docker buildx rm -f $BUIDLER