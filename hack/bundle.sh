#!/bin/sh

# Copyright 2019 ArgoCD Operator Developers
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# 	http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

HACK_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source ${HACK_DIR}/env.sh

# Create bundle build directory
mkdir -p ${ARGOCD_OPERATOR_BUNDLE_BUILD_DIR}

# Copy bundle artifacts
cp -r ${ARGOCD_OPERATOR_BUNDLE_DIR}/* ${ARGOCD_OPERATOR_BUNDLE_BUILD_DIR}/

# Copy manifests 
mkdir -p ${ARGOCD_OPERATOR_BUNDLE_BUILD_DIR}/manifests
cp -r ${ARGOCD_OPERATOR_BUNDLE_MANIFEST_DIR} ${ARGOCD_OPERATOR_BUNDLE_BUILD_DIR}/manifests/

# Build the bundle registry container image
echo "Building image ${ARGOCD_OPERATOR_BUNDLE_IMAGE}"
${ARGOCD_OPERATOR_IMAGE_BUILDER} build -t ${ARGOCD_OPERATOR_BUNDLE_IMAGE} ${ARGOCD_OPERATOR_BUNDLE_BUILD_DIR}

# Push the bundle registry container image
echo "Pushing image ${ARGOCD_OPERATOR_BUNDLE_IMAGE}"
${ARGOCD_OPERATOR_IMAGE_BUILDER} push ${ARGOCD_OPERATOR_BUNDLE_IMAGE}
