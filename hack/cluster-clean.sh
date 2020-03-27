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

# Clean up operator resources from a kubernetes cluster

# ClusterRoles/Bindings
kubectl delete clusterrolebinding \
    argocd-application-controller \
    argocd-server

kubectl delete clusterrole \
    argocd-application-controller \
    argocd-server

# Roles/Bindings
kubectl delete rolebinding -n ${ARGOCD_OPERATOR_NAMESPACE} \
    argocd-application-controller \
    argocd-dex-server \
    argocd-operator \
    argocd-redis-ha \
    argocd-repo-server \
    argocd-server

kubectl delete role -n ${ARGOCD_OPERATOR_NAMESPACE} \
    argocd-application-controller \
    argocd-dex-server \
    argocd-operator \
    argocd-redis-ha \
    argocd-repo-server \
    argocd-server

# ServiceAccounts
kubectl delete sa -n ${ARGOCD_OPERATOR_NAMESPACE} \
    argocd-application-controller \
    argocd-dex-server \
    argocd-operator \
    argocd-redis-ha \
    argocd-repo-server \
    argocd-server

# CustomResourceDefinitions
kubectl delete crd \
    applications.argoproj.io \
    appprojects.argoproj.io \
    argocdexports.argoproj.io \
    argocds.argoproj.io

# Deployments
kubectl delete deployment -n ${ARGOCD_OPERATOR_NAMESPACE} \
    argocd-operator

kubectl delete secret -n ${ARGOCD_OPERATOR_NAMESPACE} \
    scorecard-kubeconfig
