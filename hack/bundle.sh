#!/bin/sh

set -e

HACK_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source ${HACK_DIR}/env.sh

# Verify bundle
operator-courier --verbose verify ${ARGOCD_OPERATOR_BUNDLE_DIR}

# Obtain Quay crednetials
if [ -z ${QUAY_USERNAME} ]; then
    echo -n "Username: "
    read QUAY_USERNAME
fi

if [ -z ${QUAY_PASSWORD} ]; then
    echo -n "Password: "
    read -s QUAY_PASSWORD
fi

# Obtain Quay auth token
QUAY_TOKEN=$(curl -sH "Content-Type: application/json" -XPOST ${QUAY_LOGIN_URL} -d '
{
    "user": {
        "username": "'"${QUAY_USERNAME}"'",
        "password": "'"${QUAY_PASSWORD}"'"
    }
}' | awk -F '"' '{print $4}')

# Publish bundle
operator-courier --verbose push ${ARGOCD_OPERATOR_BUNDLE_DIR} ${ARGOCD_OPERATOR_BUNDLE_NAMESPACE} ${ARGOCD_OPERATOR_BUNDLE_REPO} ${ARGOCD_OPERATOR_BUNDLE_RELEASE} "${QUAY_TOKEN}"
