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
#
# Script to build the operator from source and create a new container image.

OPERATOR_SDK=${OPERATOR_SDK:-operator-sdk}

HACK_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source ${HACK_DIR}/env.sh

echo "Building image ${ARGOCD_OPERATOR_IMAGE}"
${OPERATOR_SDK} build ${ARGOCD_OPERATOR_IMAGE} ${ARGOCD_BUILD_ARGS} --image-builder ${ARGOCD_OPERATOR_IMAGE_BUILDER}
# export ${ARGOCD_BUILD_ARGS}="--go-build-args "-tags openshift"" to add openshift package to the build process that
# registers a reconciler hook which modifies the reconciler to create ArgoCD for Cluster Config. 
