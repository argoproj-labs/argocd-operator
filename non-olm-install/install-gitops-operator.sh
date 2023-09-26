#!/usr/bin/env bash

NAMESPACE=${NAMESPACE:-"openshift-gitops-operator"}
NAME_PREFIX=${NAME_PREFIX:-"openshift-gitops-operator-"}
GIT_REVISION=${GIT_REVISION:-"master"}
MAX_RETRIES=3

# gitops-operator version tagged images
OPERATOR_REGISTRY=${OPERATOR_REGISTRY:-"registry.redhat.io"}
GITOPS_OPERATOR_VER=${GITOPS_OPERATOR_VER:-"v1.9.2-2"}
OPERATOR_REGISTRY_ORG=${OPERATOR_REGISTRY_ORG:-"openshift-gitops-1"}
IMAGE_PREFIX=${IMAGE_PREFIX:-""}  
OPERATOR_IMG=${OPERATOR_IMG:-"${OPERATOR_REGISTRY}/${OPERATOR_REGISTRY_ORG}/${IMAGE_PREFIX}gitops-rhel8-operator:${GITOPS_OPERATOR_VER}"}

# If enabled, operator and component image URLs would be derived from within CSV present in the bundle image.
USE_BUNDLE_IMG=${USE_BUNDLE_IMG:-"false"}
BUNDLE_IMG=${BUNDLE_IMG:-"${OPERATOR_REGISTRY}/${OPERATOR_REGISTRY_ORG}/${IMAGE_PREFIX}gitops-operator-bundle:${GITOPS_OPERATOR_VER}"}

# Image overrides
# gitops-operator version tagged images
ARGOCD_DEX_IMAGE=${ARGOCD_DEX_IMAGE:-"${OPERATOR_REGISTRY}/${OPERATOR_REGISTRY_ORG}/${IMAGE_PREFIX}dex-rhel8:${GITOPS_OPERATOR_VER}"}
ARGOCD_IMAGE=${ARGOCD_IMAGE:-"${OPERATOR_REGISTRY}/${OPERATOR_REGISTRY_ORG}/${IMAGE_PREFIX}argocd-rhel8:${GITOPS_OPERATOR_VER}"}
BACKEND_IMAGE=${BACKEND_IMAGE:-"${OPERATOR_REGISTRY}/${OPERATOR_REGISTRY_ORG}/${IMAGE_PREFIX}gitops-rhel8:${GITOPS_OPERATOR_VER}"}
GITOPS_CONSOLE_PLUGIN_IMAGE=${GITOPS_CONSOLE_PLUGIN_IMAGE:-"${OPERATOR_REGISTRY}/${OPERATOR_REGISTRY_ORG}/${IMAGE_PREFIX}console-plugin-rhel8:${GITOPS_OPERATOR_VER}"}
KAM_IMAGE=${KAM_IMAGE:-"${OPERATOR_REGISTRY}/${OPERATOR_REGISTRY_ORG}/${IMAGE_PREFIX}kam-delivery-rhel8:${GITOPS_OPERATOR_VER}"}

# other images
ARGOCD_KEYCLOAK_IMAGE=${ARGOCD_KEYCLOAK_IMAGE:-"registry.redhat.io/rh-sso-7/sso7-rhel8-operator:7.6-8"}
ARGOCD_REDIS_IMAGE=${ARGOCD_REDIS_IMAGE:-"registry.redhat.io/rhel8/redis-6:1-110"}
ARGOCD_REDIS_HA_PROXY_IMAGE=${ARGOCD_REDIS_HA_PROXY_IMAGE:-"registry.redhat.io/openshift4/ose-haproxy-router:v4.12.0-202302280915.p0.g3065f65.assembly.stream"}

# Tool Versions
KUSTOMIZE_VERSION=${KUSTOMIZE_VERSION:-"v5.1.1"}
KUBECTL_VERSION=${KUBECTL_VERSION:-"v1.26.0"}
YQ_VERSION=${YQ_VERSION:-"v4.35.1"}
REGCTL_VERSION=${REGCTL_VERSION:-"v0.5.1"}

# Operator configurations
ARGOCD_CLUSTER_CONFIG_NAMESPACES=${ARGOCD_CLUSTER_CONFIG_NAMESPACES:-"openshift-gitops"}
CONTROLLER_CLUSTER_ROLE=${CONTROLLER_CLUSTER_ROLE:-""}
DISABLE_DEFAULT_ARGOCD_INSTANCE=${DISABLE_DEFAULT_ARGOCD_INSTANCE:-"false"}
SERVER_CLUSTER_ROLE=${SERVER_CLUSTER_ROLE:-""}
WATCH_NAMESPACE=${WATCH_NAMESPACE:-""}
ENABLE_CONVERSION_WEBHOOK=${ENABLE_CONVERSION_WEBHOOK:-"true"}

# Print help message
function print_help() {
  echo "Usage: $0 [--install|-i] [--uninstall|-u] [--help|-h]"
  echo "  --install, -i    Install the openshift-gitops-operator manifests"
  echo "  --uninstall, -u  Uninstall the openshift-gitops-operator manifests"
  echo "  --migrate, -m    Migrates from OLM to non OLM manifests based installation"
  echo "  --help, -h       Print this help message"

  echo
  echo "Example usage:"
  echo "	$0 --install"
  echo "	$0 --uninstall"
  echo "	$0 --migrate"
}


# Check if a pod is ready, if it fails to get ready, rollback to PREV_IMAGE
function check_pod_status_ready() {
  # Wait for the deployment rollout to complete before trying to list the pods
  # to ensure that only pods corresponding to the new version is considered.
  ${KUBECTL} rollout status deploy -n openshift-gitops-operator --timeout=5m
  if [ $? -ne 0 ]; then
    echo "[INFO] Deployments did not reach healthy state within 5m. Rolling back"
  else
    echo "[INFO] Deployments reached healthy state."
    return 0
  fi

  pod_name=$(${KUBECTL} get pods --no-headers --field-selector="status.phase!=Succeeded" -o custom-columns=":metadata.name" -n openshift-gitops-operator | grep "${1}");
  if [ -z "$pod_name" ]; then
    echo "[WARN] Ignoring empty pod name"
    return 0
  fi
  echo "[DEBUG] Pod name : $pod_name";
  ${KUBECTL} wait pod --for=condition=Ready $pod_name -n ${NAMESPACE} --timeout=150s;
  if [ $? -ne 0 ]; then
    echo "[INFO] Pod '$pod_name' failed to become Ready in desired time. Logs from the pod:"
    ${KUBECTL} logs $pod_name -n ${NAMESPACE} --all-containers;
    echo "[ERROR] Install/Upgrade failed. Performing rollback";
    rollback
    return 1
  fi
  return 0
}

# Handle rollback for different modes
function rollback() {
  if [ "$MODE" == "Migrate" ]; then
      rollback_to_olm
  else
      rollback_to_previous_image
  fi
}

# Rollback the deployment to use previous known good image
# Applicable only for upgrade/downgrade operations.
function rollback_to_previous_image() {
  if [ ! -z "${PREV_OPERATOR_IMG}" ]; then
    export OPERATOR_IMG=${PREV_OPERATOR_IMG}    
    prepare_kustomize_files
    ${KUSTOMIZE} build ${WORK_DIR} | ${KUBECTL} apply -f -
    echo "[INFO] Operator update operation was unsuccessful!!";
  else
    echo "[INFO] Installing image for the first time. Nothing to rollback. Quitting..";
  fi
  exit 1;
}

# deletes the work directory
function cleanup() {
  # Check if timeout binary is available in the PATH environment variable
  timeout=$(which timeout)
  if [ -z ${timeout} ]; then
    echo "[INFO] Deleting directory ${WORK_DIR} without timeout"
    rm -rf "${WORK_DIR}"
  else
    # If the command hangs for more than 10 minutes kill it
    echo "[INFO] Deleting directory ${WORK_DIR} with timeout"
    timeout 600 rm -rf "${WORK_DIR}"||echo "[ERROR] Directory deletion timed out, please remove it manually"
  fi
  echo "[INFO] Deleted work working directory ${WORK_DIR}"
}

# installs the stable version kustomize binary if not found in PATH
function install_kustomize() {
  if [[ -z "${KUSTOMIZE}" ]]; then
    echo "[INFO] kustomize binary not found in \$PATH, installing kustomize-${KUSTOMIZE_VERSION} in ${WORK_DIR}"
    wget https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2F${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_$(uname | tr '[:upper:]' '[:lower:]')_$(uname -m |sed s/aarch64/arm64/ | sed s/x86_64/amd64/).tar.gz -O ${WORK_DIR}/kustomize.tar.gz
    tar zxvf ${WORK_DIR}/kustomize.tar.gz -C ${WORK_DIR}
    KUSTOMIZE=${WORK_DIR}/kustomize
    chmod +x ${WORK_DIR}/kustomize
  fi
}

# installs the stable version of kubectl binary if not found in PATH
function install_kubectl() {
  if [[ -z "${KUBECTL}" ]]; then
    echo "[INFO] kubectl binary not found in \$PATH, installing kubectl-${KUBECTL_VERSION} in ${WORK_DIR}"
    wget https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/$(uname | tr '[:upper:]' '[:lower:]')/$(uname -m | sed s/aarch64/arm64/ | sed s/x86_64/amd64/)/kubectl -O ${WORK_DIR}/kubectl
    KUBECTL=${WORK_DIR}/kubectl
    chmod +x ${WORK_DIR}/kubectl
  fi
}

# installs the stable version of regctl binary if not found in PATH
function install_regctl() {
  if [[ -z "${REGCTL}" ]]; then
    echo "[INFO] regctl binary not found in \$PATH, installing regctl-${REGCTL_VERSION} in ${WORK_DIR}"
    wget https://github.com/regclient/regclient/releases/download/${REGCTL_VERSION}/regctl-$(uname | tr '[:upper:]' '[:lower:]')-$(uname -m | sed s/aarch64/arm64/ | sed s/x86_64/amd64/) -O ${WORK_DIR}/regctl
    REGCTL=${WORK_DIR}/regctl
    chmod +x ${WORK_DIR}/regctl
  fi
}

# installs the stable version of yq binary if not found in PATH
function install_yq() {
  if [[ -z "${YQ}" ]]; then
    echo "[INFO] yq binary not found in \$PATH, installing yq-${YQ_VERSION} in ${WORK_DIR}"
    wget https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_$(uname | tr '[:upper:]' '[:lower:]')_$(uname -m | sed s/aarch64/arm64/ | sed s/x86_64/amd64/) -O ${WORK_DIR}/yq
    YQ=${WORK_DIR}/yq
    chmod +x ${WORK_DIR}/yq
  fi
}

# creates a kustomization.yaml file in the work directory pointing to the manifests available in the upstream repo.
function create_kustomization_init_file() {
  echo "[INFO] Creating kustomization.yaml file using manifests from revision '${GIT_REVISION}'"
  echo "apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: ${NAMESPACE}
namePrefix: ${NAME_PREFIX}
resources:
  - https://github.com/redhat-developer/gitops-operator/config/crd?ref=$GIT_REVISION&timeout=90s
  - https://github.com/redhat-developer/gitops-operator/config/rbac?ref=$GIT_REVISION&timeout=90s
  - https://github.com/redhat-developer/gitops-operator/config/manager?ref=$GIT_REVISION&timeout=90s
  - https://github.com/redhat-developer/gitops-operator/config/prometheus?ref=$GIT_REVISION&timeout=90s
patches:
  - path: https://raw.githubusercontent.com/redhat-developer/gitops-operator/master/config/default/manager_auth_proxy_patch.yaml 
  - path: https://raw.githubusercontent.com/redhat-developer/gitops-operator/master/config/default/manager_webhook_patch.yaml
  - path: env-overrides.yaml
  - path: security-context.yaml" > ${WORK_DIR}/kustomization.yaml
}

# creates a patch file, containing the environment variable overrides for overriding the default images
# for various gitops-operator components.
function create_image_overrides_patch_file() {
  echo "apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-manager
  namespace: system
spec:
  template:
    spec:
      containers:
      - name: manager
        image: ${OPERATOR_IMG}
        env:
        - name: ARGOCD_DEX_IMAGE
          value: ${ARGOCD_DEX_IMAGE}
        - name: ARGOCD_KEYCLOAK_IMAGE
          value: ${ARGOCD_KEYCLOAK_IMAGE}
        - name: BACKEND_IMAGE
          value: ${BACKEND_IMAGE}
        - name: ARGOCD_IMAGE
          value: ${ARGOCD_IMAGE}
        - name: ARGOCD_REPOSERVER_IMAGE
          value: ${ARGOCD_IMAGE}
        - name: ARGOCD_REDIS_IMAGE
          value: ${ARGOCD_REDIS_IMAGE}
        - name: ARGOCD_REDIS_HA_IMAGE
          value: ${ARGOCD_REDIS_IMAGE}
        - name: ARGOCD_REDIS_HA_PROXY_IMAGE
          value: ${ARGOCD_REDIS_HA_PROXY_IMAGE}
        - name: GITOPS_CONSOLE_PLUGIN_IMAGE
          value: ${GITOPS_CONSOLE_PLUGIN_IMAGE}
        - name: KAM_IMAGE
          value: ${KAM_IMAGE}
        - name: ARGOCD_CLUSTER_CONFIG_NAMESPACES
          value: \"${ARGOCD_CLUSTER_CONFIG_NAMESPACES}\"
        - name: CONTROLLER_CLUSTER_ROLE
          value: \"${CONTROLLER_CLUSTER_ROLE}\"
        - name: DISABLE_DEFAULT_ARGOCD_INSTANCE
          value: \"${DISABLE_DEFAULT_ARGOCD_INSTANCE}\"
        - name: SERVER_CLUSTER_ROLE
          value: \"${SERVER_CLUSTER_ROLE}\"
        - name: WATCH_NAMESPACE
          value: \"${WATCH_NAMESPACE}\"
        - name: ENABLE_CONVERSION_WEBHOOK
          value: \"${ENABLE_CONVERSION_WEBHOOK}\"" > ${WORK_DIR}/env-overrides.yaml
}

# Create a security context for the containers that are present in the deployment.
function create_security_context_patch_file(){
  echo "apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-manager
  namespace: system
spec:
  template:
    metadata:
      annotations:
        openshift.io/scc: restricted-v2
    spec:
      containers:
      - name: kube-rbac-proxy
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          seccompProfile:
            type: RuntimeDefault
      - name: manager
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          seccompProfile:
            type: RuntimeDefault" > ${WORK_DIR}/security-context.yaml
}

function extract_component_images_from_bundle_image() {
  ${REGCTL} image get-file "${BUNDLE_IMG}" manifests/gitops-operator.clusterserviceversion.yaml "${WORK_DIR}"/gitops-operator.clusterserviceversion.yaml

  CONTAINER_YAML=$(cat "${WORK_DIR}"/gitops-operator.clusterserviceversion.yaml | ${YQ} '.spec.install.spec | .deployments[0].spec.template.spec.containers[0]' > "${WORK_DIR}"/container.yaml)

  # Get the operator image from the CSV of the operator bundle
  OPERATOR_IMG=$(cat "${WORK_DIR}"/container.yaml | ${YQ} '.image')

  # Get the component images from the CSV of the operator bundle
  ARGOCD_DEX_IMAGE=$(cat "${WORK_DIR}"/container.yaml | ${YQ} '.env[] | select(.name=="ARGOCD_DEX_IMAGE").value')
  ARGOCD_IMAGE=$(cat "${WORK_DIR}"/container.yaml | ${YQ} '.env[] | select(.name=="ARGOCD_IMAGE").value')
  ARGOCD_KEYCLOAK_IMAGE=$(cat "${WORK_DIR}"/container.yaml | ${YQ} '.env[] | select(.name=="ARGOCD_KEYCLOAK_IMAGE").value')
  ARGOCD_REDIS_IMAGE=$(cat "${WORK_DIR}"/container.yaml | ${YQ} '.env[] | select(.name=="ARGOCD_REDIS_IMAGE").value')
  ARGOCD_REDIS_HA_PROXY_IMAGE=$(cat "${WORK_DIR}"/container.yaml | ${YQ} '.env[] | select(.name=="ARGOCD_REDIS_HA_PROXY_IMAGE").value')
  BACKEND_IMAGE=$(cat "${WORK_DIR}"/container.yaml | ${YQ} '.env[] | select(.name=="BACKEND_IMAGE").value')
  GITOPS_CONSOLE_PLUGIN_IMAGE=$(cat "${WORK_DIR}"/container.yaml | ${YQ} '.env[] | select(.name=="GITOPS_CONSOLE_PLUGIN_IMAGE").value')
  KAM_IMAGE=$(cat "${WORK_DIR}"/container.yaml | ${YQ} '.env[] | select(.name=="KAM_IMAGE").value')
}

# Initialize a temporary work directory to store the artifacts and 
# clean it up before the completion of the script run.
function init_work_directory() {
  # create a temporary directory and do all the operations inside the directory.
  WORK_DIR=$(mktemp -d "${TMPDIR:-"/tmp"}/gitops-operator-install-XXXXXXX")
  echo "[INFO] Using work directory $WORK_DIR"
  # cleanup the work directory irrespective of whether the script ran successfully or failed with an error.
  trap cleanup EXIT
}

# Checks if the pre-requisite binaries are already present in the PATH,
# if not installs appropriate versions of them.
# This function just checks if the binary is found in the PATH and 
# does not validate if the version of the binary matches the minimum required version.
function check_and_install_prerequisites {
  # Check if wget is available in PATH, if not exit immediately.
  which wget
  if [ $? -ne 0 ]; then
    echo "Mandatory pre-requsite 'wget' missing"
    exit 1
  fi

  # install kustomize in the the work directory if its not available in the PATH
  KUSTOMIZE=$(which kustomize)
  install_kustomize

  # install kubectl in the the work directory if its not available in the PATH
  KUBECTL=$(which kubectl)
  install_kubectl

  # install yq in the the work directory if its not available in the PATH
  YQ=$(which yq)
  install_yq

  if [ ${USE_BUNDLE_IMG} == "true" ];then
    # install yq in the the work directory if its not available in the PATH
    REGCTL=$(which regctl)
    install_regctl
    check_prerequisite regctl ${REGCTL}
  fi

  check_prerequisite kustomize ${KUSTOMIZE}
  check_prerequisite kubectl ${KUBECTL}
  check_prerequisite yq ${YQ}

}

# Check if the given prerequisite binary is found in path or the script
# installed them in the path.
function check_prerequisite() {
  if [[ -z "${2}" || ! -x "${2}"  ]]; then
    echo "Prerequisite '${1}' binary could not be installed"
    exit 1
  fi
}

# Checks if the openshift-gitops-operator is already installed in the system.
# if so, stores the previous version which would be used for rollback in case of
# a failure during installation.
function get_prev_operator_image() {
  for image in $(${KUBECTL} get deploy/openshift-gitops-operator-controller-manager -n ${NAMESPACE} -o jsonpath='{..image}' 2>/dev/null)
  do
    if [[ "${image}" == *"operator"* ]]; then
      PREV_OPERATOR_IMG="${image}"
      break
    fi
  done
  if [ ! -z "${PREV_OPERATOR_IMG}" ]; then
    MODE="Update"
  fi
}

# Prepares the kustomization.yaml file in the WORK_DIR which would be used 
# for the installation.
function prepare_kustomize_files() {
  # create the required yaml files for the kustomize based install.
  create_kustomization_init_file
  if [ ${USE_BUNDLE_IMG} == "true" ]; then
    extract_component_images_from_bundle_image
  fi
  create_image_overrides_patch_file
  create_security_context_patch_file
}

# Build and apply the kustomize manifests with retries
function apply_kustomize_manifests() {
  retry_count=1
  until [ "${retry_count}" -gt ${MAX_RETRIES} ]
  do
    attempt=${retry_count}
    retry_count=$((retry_count+1))
    echo "[INFO] (Attempt ${attempt}) Executing kustomize build command"
    ${KUSTOMIZE} build ${WORK_DIR} > ${WORK_DIR}/kustomize-build-output.yaml || continue
    ${YQ} -i 'del( .metadata.creationTimestamp | select(. == "null") )' ${WORK_DIR}/kustomize-build-output.yaml
    echo "[INFO] (Attempt ${attempt}) Creating k8s resources from kustomize manifests"
    ${KUBECTL} apply --server-side=true -f ${WORK_DIR}/kustomize-build-output.yaml && break
  done
}

# Build and delete the kustomize manifests with retries
function delete_kustomize_manifests() {
  retry_count=1
  until [ "${retry_count}" -gt ${MAX_RETRIES} ]
  do
    echo "[INFO] (Attempt ${retry_count}) Executing kustomize build command"
    retry_count=$((retry_count+1))
    ${KUSTOMIZE} build ${WORK_DIR} > ${WORK_DIR}/kustomize-build-output.yaml && break
  done
  echo "[INFO] Deleting k8s resources from kustomize manifests"
  ${KUBECTL} delete -f ${WORK_DIR}/kustomize-build-output.yaml
}


function print_info() {
  echo "Run information:"
  echo "==========================================="
  echo "MANIFEST_VERSION: ${GIT_REVISION}"
  echo ""
  if [ "${USE_BUNDLE_IMG}" == "true" ]; then
    echo "Bundle image:"
    echo "-------------"
    echo "BUNDLE_IMG: ${BUNDLE_IMG}"
    echo ""
  fi
  echo "Operator image:"
  echo "---------------"
  echo "OPERATOR_IMG: ${OPERATOR_IMG}"
  echo "OPERATION_MODE: $MODE"
  if [ ! -z "${PREV_OPERATOR_IMG}" ]; then
    echo "PREVIOUS_OPERATOR_IMG: ${PREV_OPERATOR_IMG}"
    echo ""
  fi
  echo "Component images:"
  echo "-----------------"
  echo "ARGOCD_DEX_IMAGE: ${ARGOCD_DEX_IMAGE}"
  echo "ARGOCD_IMAGE: ${ARGOCD_IMAGE}"
  echo "ARGOCD_KEYCLOAK_IMAGE: ${ARGOCD_KEYCLOAK_IMAGE}"
  echo "ARGOCD_REDIS_IMAGE: ${ARGOCD_REDIS_IMAGE}"
  echo "ARGOCD_REDIS_HA_PROXY_IMAGE: ${ARGOCD_REDIS_HA_PROXY_IMAGE}"
  echo "BACKEND_IMAGE: ${BACKEND_IMAGE}"
  echo "GITOPS_CONSOLE_PLUGIN_IMAGE: ${GITOPS_CONSOLE_PLUGIN_IMAGE}"
  echo "KAM_IMAGE: ${KAM_IMAGE}"
  echo ""

  echo "Operator configurations:"
  echo "------------------------"
  echo "ARGOCD_CLUSTER_CONFIG_NAMESPACES: ${ARGOCD_CLUSTER_CONFIG_NAMESPACES}"
  if [ ! -z "${CONTROLLER_CLUSTER_ROLE}" ]; then
    echo "CONTROLLER_CLUSTER_ROLE: ${CONTROLLER_CLUSTER_ROLE}"
  fi
  echo "DISABLE_DEFAULT_ARGOCD_INSTANCE: ${DISABLE_DEFAULT_ARGOCD_INSTANCE}"
  if [ ! -z "${SERVER_CLUSTER_ROLE}" ]; then
    echo "SERVER_CLUSTER_ROLE: ${SERVER_CLUSTER_ROLE}"
  fi
  if [ ! -z "${WATCH_NAMESPACE}" ]; then
    echo "WATCH_NAMESPACE: ${WATCH_NAMESPACE}"
  fi
  if [ ! -z "${ENABLE_CONVERSION_WEBHOOK}" ]; then
    echo "ENABLE_CONVERSION_WEBHOOK: ${ENABLE_CONVERSION_WEBHOOK}"
  fi
  echo "==========================================="
}

# migration from an OLM installation to a non OLM installation.
function migrate_olm_installation() {
  extract_custom_env_in_subscription
  scale_down_olm_deploy

  if [ -f ${WORK_DIR}/migrate_env.sh ];then
    echo "Sourcing env variables used for customizing subscription"
    source ${WORK_DIR}/migrate_env.sh
  fi
  apply_kustomize_manifests
  # Check pod status if it becomes ready
  check_pod_status_ready openshift-gitops-operator-controller-manager

  if [ $? -eq 0 ]; then
    # Non OLM installation is successful and its safe to remove the OLM specific
    # objects.
    remove_subscription
    remove_installed_csv
    wait_for_olm_removal
  fi
}

# When migrating from OLM to non OLM installation, deployment created by the OLM operator
# must be scaled down to avoid 2 conflicting operators operating on the same CR.
function scale_down_olm_deploy() {
  ${KUBECTL} scale deploy/openshift-gitops-operator-controller-manager -n ${NAMESPACE} --replicas=0
}

# If migration to non OLM installation fails, revert to OLM based installation
# by scaling back the OLM created deployments from 0 to 1.
# Note: Rollback is possible only if the corresponding Subscription and ClusterServiceVersion objects are available.
function rollback_to_olm() {
  ${KUBECTL} scale deploy/openshift-gitops-operator-controller-manager -n ${NAMESPACE} --replicas=1
}

# Deletes the subscription for openshift-gitops-operator
function remove_subscription() {
  #Delete the gitops subscription
  ${KUBECTL} delete subscription openshift-gitops-operator -n ${NAMESPACE}
}

# Deletes the ClusterServiceVersion Object from the system
function remove_installed_csv() {
  # get installedCSV from subscription status
  installedCSV=$(${YQ} '.status.installedCSV' ${WORK_DIR}/subscription.yaml)
  if [ "${installedCSV}" == "null" ]; then
    echo "[INFO] No installed CSV in Subscription"
    return
  fi
  ${KUBECTL} delete clusterserviceversion ${installedCSV} -n ${NAMESPACE}
}

# Waits till the OLM removal is successful.
function wait_for_olm_removal() {
  # Wait till the operator deployment is completely removed.
  ${KUBECTL} wait --for=delete deploy/openshift-gitops-operator-controller-manager -n ${NAMESPACE} --timeout=60s
}

# Extract the custom configuration set in the Subscription and
# store the env settings in a file which can be sourced when running
# the non-OLM installation.
function extract_custom_env_in_subscription() {
  # Get the GitOps subscription object as yaml
  ${KUBECTL} get subscription openshift-gitops-operator -n ${NAMESPACE} -o yaml > ${WORK_DIR}/subscription.yaml
  # check if config.env element is present
  element=$(${YQ} '.spec.config.env' ${WORK_DIR}/subscription.yaml)
  if [ "${element}" == "null" ]; then
    echo "[INFO] No custom config present in Subscription"
    return
  fi

  # for each custom env, convert it to key=value combination.
  while IFS=$'\t' read -r name value _; do
    echo "Setting $name=$value"
    echo "export $name=$value" >> ${WORK_DIR}/migrate_env.sh
  done < <(yq e '.[] | [.name, .value] | @tsv' ${WORK_DIR}/env_overrides.yaml)
}



# Code execution starts here
function main() {
  if [ $# -eq 0 ]; then
    echo "[ERROR] No option provided"
    print_help
    exit 1
  fi 

  if [ $# -gt 1 ]; then
    echo "[ERROR] Exactly one argument is expected, but found more than one."
    print_help
    exit 1
  fi

  key=$1
  case $key in
    --install | -i)
	MODE="Install"
        init_work_directory
        check_and_install_prerequisites
        get_prev_operator_image
        prepare_kustomize_files
        print_info
        echo "[INFO] Performing $MODE operation for openshift-gitops-operator..."
        if [[ $MODE == "Install" ]]; then 
          ${KUBECTL} create ns ${NAMESPACE}
          ${KUBECTL} label ns ${NAMESPACE} openshift.io/cluster-monitoring=true
        fi
        apply_kustomize_manifests
        # Check pod status and rollback if necessary.
        check_pod_status_ready openshift-gitops-operator-controller-manager 
        exit 0
        ;;
    --uninstall | -u)
	MODE="Uninstall"
        echo "[INFO] Performing $MODE operation openshift-gitops-operator..."
        init_work_directory
        check_and_install_prerequisites
        prepare_kustomize_files
        print_info
        # Remove the GitOpsService instance created for the default
        # ArgoCD instance created in openshift-gitops namespace.
        ${KUBECTL} delete gitopsservice/cluster
        ${KUBECTL} delete ns ${NAMESPACE}
        delete_kustomize_manifests
        exit 0
        ;;
    --migrate | -m)
	MODE="Migrate"
        echo "[INFO] Performing $MODE operation openshift-gitops-operator..."
        init_work_directory
        check_and_install_prerequisites
        prepare_kustomize_files
	      print_info
        # Remove the GitOpsService instance created for the default
        # ArgoCD instance created in openshift-gitops namespace.
        migrate_olm_installation
        exit 0
        ;;
    -h | --help)
        print_help
        exit 0
        ;;
    *)
        echo "[ERROR] Invalid argument $key"
        print_help
        exit 1
        ;;
  esac
}

main "$@"
