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

refdocs \
    -api-dir "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1" \
    -config "${ARGOCD_OPERATOR_DOCS_DIR}/apidocs-config.json" \
    -out-file "${ARGOCD_OPERATOR_DOCS_DIR}/api.html" \
    -template-dir "docs/template"
