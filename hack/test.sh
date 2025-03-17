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
# Script to run the operator tests.

set -e 

kubectl kuttl test ./tests/k8s --config ./tests/kuttl-tests.yaml

ENABLE_MANAGED_NAMESPACE_FEATURE=true kubectl kuttl test ./tests/nm  --config ./tests/kuttl-tests.yaml --test 1-048_validate_namespace_management_glob
