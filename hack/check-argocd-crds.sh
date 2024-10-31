#!/bin/bash

# Pre-requisites
# go
# awk
# bash
# wget

set -e

ARGOCD_VERSION=$(go list -mod=readonly -m github.com/argoproj/argo-cd/v2 | awk '{print $2}')
ARGOCD_CRD_FILES="application-crd.yaml applicationset-crd.yaml appproject-crd.yaml"

check_for_sem_ver() {
  if [[ $ARGOCD_VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9])?$ ]]; then
    echo "argo-cd version from go.mod: $ARGOCD_VERSION"
    return
  else
    echo "[WARN] argo-cd module version from go.mod does not match the semver pattern."
    exit 1
  fi
}

download_manifests() {
    for argocd_crd_file in $ARGOCD_CRD_FILES;do
        output_file=argoproj.io_$(echo "${argocd_crd_file}" | sed -e s/-crd.yaml/s.yaml/g)
        wget https://raw.githubusercontent.com/argoproj/argo-cd/refs/tags/${ARGOCD_VERSION}/manifests/crds/${argocd_crd_file} -O bundle/manifests/${output_file}
    done
}

check_for_local_changes() {
    local_modified_files=$(git status --porcelain | grep "bundle/manifests/argoproj.io_app" | cat)
    if [[ ! -z "${local_modified_files}" ]]; then
        echo "[WARN] There are unexpected local changes to the argo-cd CRD manifests."
        echo "${local_modified_files}"
        echo "Please update the CRDs from the argo-cd repository, ensure that there are no diffs and re-submit"
        exit 1
    else
        echo "No local changes to argo-cd CRD manifests"
    fi
}

check_for_sem_ver
download_manifests
check_for_local_changes
