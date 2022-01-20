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

kubectl create namespace argocd-e2e
kubectl kuttl test
kubectl delete namespace argocd-e2e

kubectl create namespace argocd-e2e
kubectl kuttl test --config kuttl-test-redis-ha.yaml
kubectl delete namespace argocd-e2e

kubectl create namespace argocd-e2e-cluster-config
kubectl kuttl test --config kuttl-test-cluster-config.yaml
kubectl delete namespace argocd-e2e-cluster-config
