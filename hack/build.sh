#!/bin/sh

HACK_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source ${HACK_DIR}/env.sh

operator-sdk build ${ARGOCD_OPERATOR_IMAGE}
