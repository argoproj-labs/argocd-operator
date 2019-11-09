#!/bin/sh

HACK_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source ${HACK_DIR}/env.sh

operator-sdk generate k8s
operator-sdk generate openapi
