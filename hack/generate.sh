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

HACK_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source ${HACK_DIR}/env.sh

# Generate CRDs for API's
operator-sdk generate crds

# Generate Kubernetes code for custom resource
operator-sdk generate k8s

# Run openapi-gen for each of the API group/version packages
openapi-gen \
    --go-header-file ./hack/boilerplate.go.txt \
    --input-dirs ./pkg/apis/argoproj/v1alpha1 \
    --logtostderr=true \
    --output-base "" \
    --output-file-base zz_generated.openapi \
    --output-package ./pkg/apis/argoproj/v1alpha1 \
    --report-filename -
