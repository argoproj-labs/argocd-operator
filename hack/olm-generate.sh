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
# Script to generate the OLM artifacts for the operator.

HACK_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source ${HACK_DIR}/env.sh

# Generate CSV 
echo "Generating CSV for version ${ARGOCD_OPERATOR_VERSION}"
operator-sdk generate packagemanifests \
    --operator-name ${ARGOCD_OPERATOR_NAME} \
    --version ${ARGOCD_OPERATOR_VERSION} \
    --from-version ${ARGOCD_OPERATOR_PREVIOUS_VERSION} \
    --update-crds
