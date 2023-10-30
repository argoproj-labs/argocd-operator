#!/bin/bash

set -e

# This script installs operator via OLM for testing.
# Note: This script will update local configurations files. 
# Ensure you revert changes before committing if running locally.

check_command() {
    command=$1
    if ! command -v $command &> /dev/null; then
        echo "$command is not installed. Please install it before proceeding."
        exit 1
    fi
}

check_deps() {
    echo "Checking dependencies..."
    check_command kubectl
    check_command operator-sdk
    check_command make
    echo "All required dependencies are installed."
}

wait_for_deployment() {
    NAMESPACE=$1
    DEPLOYMENT_NAME=$2
    REPLICAS=${3:-1}
    MAX_WAIT_SECONDS=300

    echo "Waiting for $DEPLOYMENT_NAME deployment replicas to be ready..."

    # Loop until replicas are ready or timeout is reached
    WAITED_SECONDS=0
    while [ $WAITED_SECONDS -lt $MAX_WAIT_SECONDS ]; do
        READY_REPLICAS=$(kubectl get deployment -n $NAMESPACE $DEPLOYMENT_NAME -o jsonpath='{.status.readyReplicas}') || true
        if [ "$READY_REPLICAS" -eq "$REPLICAS" ]; then
            echo "All $REPLICAS replicas are ready!"
            break
        else
            echo "Waiting for replicas to be ready. Current ready replicas: $READY_REPLICAS"
            sleep 10
            WAITED_SECONDS=$((WAITED_SECONDS + 10))
        fi
    done

    if [ $WAITED_SECONDS -ge $MAX_WAIT_SECONDS ]; then
        echo "Timed out waiting for replicas to be ready."
        exit 1
    fi
}

install_olm() {
    if kubectl get deployment/olm-operator -n olm >/dev/null 2>&1; then
        echo "OLM is installed on the cluster."
    else
        echo "Installing OLM...."
        operator-sdk olm install || true
    fi
    wait_for_deployment "olm" "olm-operator"
}

# env setup
REGISTRY="${REGISTRY:-k3d-registry.localhost:5000}"
ORG="${ORG:-local}"

export IMAGE_TAG_BASE="$REGISTRY/$ORG/argocd-operator"
export VERSION="${VERSION:-0.8.0}"

check_deps
install_olm

# build and push operator image
make generate manifests
make docker-build docker-push

# build and push bundle image
make bundle 
make bundle-build bundle-push

# build and push catalog image
make catalog-build catalog-push

# install catalog
CATALOG_IMAGE="$IMAGE_TAG_BASE-catalog:v$VERSION"
sed "s|image:.*|image: $CATALOG_IMAGE|" deploy/catalog_source.yaml | kubectl apply -n operators -f -

# create subscription
sed "s/sourceNamespace:.*/sourceNamespace: operators/" deploy/subscription.yaml | kubectl apply -n operators -f -

wait_for_deployment "operators" "argocd-operator-controller-manager"


# cleanup
#operator-sdk cleanup argocd-operator -n operators