#!/bin/sh

HACK_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source ${HACK_DIR}/env.sh

export GO111MODULE=on
operator-sdk build ${ARGOCD_OPERATOR_IMAGE}
