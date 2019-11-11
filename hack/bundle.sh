#!/bin/sh

set -e

HACK_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source ${HACK_DIR}/env.sh

# Perform metadata syntax checking and validation on the artifacts
operator-courier --verbose verify ${ARGOCD_OPERATOR_BUNDLE_MANIFEST_DIR}

# Create bundle build directory
mkdir -p ${ARGOCD_OPERATOR_BUNDLE_BUILD_DIR}

# Copy bundle artifacts
cp -r ${ARGOCD_OPERATOR_BUNDLE_DIR}/* ${ARGOCD_OPERATOR_BUNDLE_BUILD_DIR}/

# Copy manifests 
mkdir -p ${ARGOCD_OPERATOR_BUNDLE_BUILD_DIR}/manifests
cp -r ${ARGOCD_OPERATOR_BUNDLE_MANIFEST_DIR} ${ARGOCD_OPERATOR_BUNDLE_BUILD_DIR}/manifests/

# Build the bundle registry container image
docker build -t ${ARGOCD_OPERATOR_BUNDLE_IMAGE} ${ARGOCD_OPERATOR_BUNDLE_BUILD_DIR}
docker push ${ARGOCD_OPERATOR_BUNDLE_IMAGE}
