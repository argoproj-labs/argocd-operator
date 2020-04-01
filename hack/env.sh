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

# General vars
export ARGOCD_OPERATOR_NAME=${ARGOCD_OPERATOR_NAME:-"argocd-operator"}
export ARGOCD_OPERATOR_NAMESPACE=${ARGOCD_OPERATOR_NAMESPACE:-"argocd"}
export ARGOCD_OPERATOR_VERSION=${ARGOCD_OPERATOR_VERSION:-`awk '$1 == "Version" {gsub(/"/, "", $3); print $3}' version/version.go`}
export ARGOCD_OPERATOR_PREVIOUS_VERSION=${ARGOCD_OPERATOR_PREVIOUS_VERSION:-`awk '$1 == "Version" {gsub(/"/, "", $3); print $3}' version/version.go`}
export ARGOCD_OPERATOR_BUILD_DIR=${ARGOCD_OPERATOR_BUILD_DIR:-"build"}
export ARGOCD_OPERATOR_DEPLOY_DIR=${ARGOCD_OPERATOR_DEPLOY_DIR:-"deploy"}
export ARGOCD_OPERATOR_DOCS_DIR=${ARGOCD_OPERATOR_DOCS_DIR:-"docs"}

# Container image vars
export ARGOCD_OPERATOR_IMAGE_BUILDER=${ARGOCD_OPERATOR_IMAGE_BUILDER:-"podman"}
export ARGOCD_OPERATOR_IMAGE_REPO=${ARGOCD_OPERATOR_IMAGE_REPO:-"quay.io/jmckind/${ARGOCD_OPERATOR_NAME}"}
export ARGOCD_OPERATOR_IMAGE_TAG=${ARGOCD_OPERATOR_IMAGE_TAG:-"latest"}
export ARGOCD_OPERATOR_IMAGE=${ARGOCD_OPERATOR_IMAGE:-"${ARGOCD_OPERATOR_IMAGE_REPO}:${ARGOCD_OPERATOR_IMAGE_TAG}"}

# Operator bundle vars
export ARGOCD_OPERATOR_BUNDLE_DIR=${ARGOCD_OPERATOR_BUNDLE_DIR:-"deploy/bundle"}
export ARGOCD_OPERATOR_BUNDLE_BUILD_DIR=${ARGOCD_OPERATOR_BUNDLE_BUILD_DIR:-"build/_output/bundle"}
export ARGOCD_OPERATOR_BUNDLE_MANIFEST_DIR=${ARGOCD_OPERATOR_BUNDLE_MANIFEST_DIR:-"deploy/olm-catalog/${ARGOCD_OPERATOR_NAME}"}
export ARGOCD_OPERATOR_BUNDLE_IMAGE_NAME=${ARGOCD_OPERATOR_BUNDLE_IMAGE_NAME:-"${ARGOCD_OPERATOR_NAME}-registry"}
export ARGOCD_OPERATOR_BUNDLE_IMAGE_REPO=${ARGOCD_OPERATOR_BUNDLE_IMAGE_REPO:-"quay.io/jmckind/${ARGOCD_OPERATOR_BUNDLE_IMAGE_NAME}"}
export ARGOCD_OPERATOR_BUNDLE_IMAGE_TAG=${ARGOCD_OPERATOR_BUNDLE_IMAGE_TAG:-"latest"}
export ARGOCD_OPERATOR_BUNDLE_IMAGE=${ARGOCD_OPERATOR_BUNDLE_IMAGE:-"${ARGOCD_OPERATOR_BUNDLE_IMAGE_REPO}:${ARGOCD_OPERATOR_BUNDLE_IMAGE_TAG}"}

# Misc
export GO111MODULE=on
