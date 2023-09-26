### Non OLM based operator installation

`install-gitops-operator.sh` is a bash script utility, that can be used to install, update(upgrade/downgrade) or uninstall the Openshift GitOps Operator without using the `Operator Lifecycle Manager (OLM)`. It uses latest version of the `kustomize` manifests available in the github repository for creating/updating/deleting the kubernetes resources required for the openshift-gitops-operator.

### Usage

The `install-gitops-operator.sh` script supports two methods of installation.
1. Using operator and component images set as environment variables (default method)
2. Derive the operator and component images from the `ClusterServiceVersion` manifest present in the operator bundle
**Note**: Use environment variables `USE_BUNDLE_IMG`, `BUNDLE_IMG` for this method of installation


### Known issues and work arounds

1. Missing RBAC access to update CRs in `argoproj.io` domain

Affected versions:
- 1.7.4 and older versions
- 1.8.3 and older versions

Fixed versions:
- 1.8.4 and later versions
- 1.9.0 and later versions

Issue:
https://github.com/redhat-developer/gitops-operator/issues/148

Workaround:
Run the following script to create the required `ClusterRole` and `ClusterRoleBinding`

```
${KUBECTL} apply -f https://raw.githubusercontent.com/redhat-developer/gitops-operator/master/hack/non-bundle-install/rbac-patch.yaml
```
### Prerequisites
- kustomize (v4.57 or later) 
- kubectl (v1.26.0 or later)
- yq (v4.31.2 or later)
**Note**: If the above binaries are not present, the script installs them to temporary work directory and are removed once the script execution is complete.
- bash (v5.0 or later)
- git (v2.39.1 or later)
- wget (v1.21.3 or later)

### Environment Variables
The following environment variables can be set to configure various options for the installation/uninstallation process.

#### Variables for Operator image and related manifests
| Environment | Description |Default Value |
| ----------- | ----------- |------------- |
| **NAMESPACE_PREFIX** | Namespace prefix to be used in the kustomization.yaml file when running kustomize | `gitops-operator-` |
| **GIT_REVISION** | The revision of the kustomize manifest to be used. | master |
| **OPERATOR_REGISTRY** |Registry server for downloading the container images |registry.redhat.io |
| **OPERATOR_REGISTRY_ORG** | Organization in the registry server for downloading the container images | openshift-gitops-1 |
| **GITOPS_OPERATOR_VER**|Version of the gitops operator version to use|1.8.1-1|
| **OPERATOR_IMG**|Operator image to be used for the installation|`${OPERATOR_REGISTRY}/${OPERATOR_REGISTRY_ORG}/${IMAGE_PREFIX}gitops-rhel8-operator:${GITOPS_OPERATOR_VER}` |
| **IMAGE_PREFIX** | Prefix used for internal images from rh-osbs org in the registry which generally is prefixed with the target organization name | "" |
| **USE_BUNDLE_IMG** | If the operator image and other component image needs to be derived from a bundle image, set this flag to true. | false |
| **BUNDLE_IMG** | used only when USE_BUNDLE_IMG is set to true | `${OPERATOR_REGISTRY}/openshift-gitops-1/gitops-operator-bundle:${GITOPS_OPERATOR_VER}` |

#### Variables for 3rd party tools used in the script
| Environment | Description |Default Value |
| ----------- | ----------- |------------- |
| **KUSTOMIZE_VERSION** | Version of kustomize binary to be installed if not found in PATH | v4.5.7 |
| **KUBECTL_VERSION** | Version of the kubectl client binary to be installed if not found in PATH | v1.26.0 |
| **YQ_VERSION** | Version of the yq binary to be installed if not found in PATH | v4.31.2 |
| **REGCTL_VERSION** | Version of the regctl binary to be installed if not found in PATH  | v0.4.8 |

#### Variables for Component Image Overrides
| Environment | Description |Default Value |
| ----------- | ----------- |------------- |
| **ARGOCD_DEX_IMAGE** | Image override for Argo CD DEX component| `${OPERATOR_REGISTRY}/${OPERATOR_REGISTRY_ORG}/${IMAGE_PREFIX}dex-rhel8:${GITOPS_OPERATOR_VER}` |
| **ARGOCD_IMAGE** | Image override for Argo CD component | `${OPERATOR_REGISTRY}/${OPERATOR_REGISTRY_ORG}/${IMAGE_PREFIX}argocd-rhel8:${GITOPS_OPERATOR_VER}` |
| **ARGOCD_KEYCLOAK_IMAGE** | Image override for Keycloak component | `registry.redhat.io/rh-sso-7/sso7-rhel8-operator:7.6-8` |
| **ARGOCD_REDIS_IMAGE** | Image override for Redis component | `registry.redhat.io/rhel8/redis-6:1-110` |
| **ARGOCD_REDIS_HA_PROXY_IMAGE** | Image override for Redis HA proxy component | `registry.redhat.io/openshift4/ose-haproxy-router:v4.12.0-202302280915.p0.g3065f65.assembly.stream` |
| **BACKEND_IMAGE** | Image override for Backend component |`${OPERATOR_REGISTRY}/${OPERATOR_REGISTRY_ORG}/${IMAGE_PREFIX}gitops-rhel8:${GITOPS_OPERATOR_VER}`|
| **GITOPS_CONSOLE_PLUGIN_IMAGE** | Image override for console plugin component | `${OPERATOR_REGISTRY}/${OPERATOR_REGISTRY_ORG}/${IMAGE_PREFIX}console-plugin-rhel8:${GITOPS_OPERATOR_VER}` |
| **KAM_IMAGE** | Image override for KAM component | `${OPERATOR_REGISTRY}/${OPERATOR_REGISTRY_ORG}/kam-delivery-rhel8:${GITOPS_OPERATOR_VER}` |


#### Variables for Operator parameters
| Environment | Description |Default Value |
| ----------- | ----------- |------------- |
| **ARGOCD_CLUSTER_CONFIG_NAMESPACES** |OpenShift GitOps instances in the identified namespaces are granted limited additional permissions to manage specific cluster-scoped resources, which include platform operators, optional OLM operators, user management, etc.Multiple namespaces can be specified via a comma delimited list. | openshift-gitops |
| **CONTROLLER_CLUSTER_ROLE** | This environment variable enables administrators to configure a common cluster role to use across all managed namespaces in the role bindings the operator creates for the Argo CD application controller. | None |
| **DISABLE_DEFAULT_ARGOCD_INSTANCE** | When set to `true`, this will disable the default 'ready-to-use' installation of Argo CD in the `openshift-gitops` namespace. |false |
| **SERVER_CLUSTER_ROLE** |This environment variable enables administrators to configure a common cluster role to use across all of the managed namespaces in the role bindings the operator creates for the Argo CD server. | None |
| **WATCH_NAMESPACE** | namespaces in which Argo applications can be created | None |
### Running the script

#### Usage

```
install-gitops-operator.sh [--install|-i] [--uninstall|-u] [--migrate|-m] [--help|-h]
```

| Option | Description |
| -------| ----------- |
| --install, -i | installs the openshift-gitops-operator if no previous version is found, else updates (upgrade/dowgrade) the existing operator |
| --uninstall, -u | uninstalls the openshift-gitops-operator |
| --migrate, -m | migrates from an OLM based installation to non OLM manifests based installation|
| --help, -h | prints the help message |

#### Local Run
##### Installation
The below command installs the latest available openshift-gitops-operator version
```
./install-gitops-operator.sh -i
```
[or]
```
./install-gitops-operator.sh --install
```
##### Uninstallation
```
./install-gitops-operator.sh -u

```
[or]
```
./install-gitops-operator.sh --uninstall
```

##### Migration
To migrate from an OLM based installation to the latest version using non OLM manifests based installation, run the following command.
```
./install-gitops-operator.sh -m

```
[or]
```
./install-gitops-operator.sh --migrate
```

#### Running it from a remote URL

```
curl -L https://raw.githubusercontent.com/redhat-developer/gitops-operator/master/hack/non-olm-install/install-gitops-operator.sh | bash -s -- -i

```

#### Running install with custom Operator image

```
OPERATOR_REGISTRY=brew.registry.redhat.io OPERATOR_REGISTRY_ORG=rh-osbs IMAGE_PREFIX=openshift-gitops-1- GITOPS_OPERATOR_VER=v99.9.0-88 ./install-gitops-operator.sh -i
```

#### Installing nightly gitops-operator build using bundle image

##### Create ImageContentSourcePolicy Custom Resource
The below `ImageContentSourcePolicy` would redirect images requests for `registry.redhat.io` to `brew.registry.redhat.io`

```k apply -f - <<EOF
apiVersion: operator.openshift.io/v1alpha1
kind: ImageContentSourcePolicy
metadata:
  name: brew-registry
spec:
  repositoryDigestMirrors:
  - mirrors:
    - brew.registry.redhat.io
    source: registry.redhat.io
  - mirrors:
    - brew.registry.redhat.io
    source: registry.stage.redhat.io
  - mirrors:
    - brew.registry.redhat.io
    source: registry-proxy.engineering.redhat.com
EOF
```

##### Login to brew.registry.redhat.io
```
docker login brew.registry.redhat.io -u $USERNAME -p <TOKEN>
```
##### Update the image pull secret to include credentials for brew.registry.redhat.io

```#!/usr/bin/env bash

oldauth=$(mktemp)
newauth=$(mktemp)

# Get current information
oc get secrets pull-secret -n openshift-config -o template='{{index .data ".dockerconfigjson"}}' | base64 -d > ${oldauth}

# Get Brew registry credentials
brew_secret=$(jq '.auths."brew.registry.redhat.io".auth' ${HOME}/.docker/config.json | tr -d '"')

# Append the key:value to the JSON file
jq --arg secret ${brew_secret} '.auths |= . + {"brew.registry.redhat.io":{"auth":$secret}}' ${oldauth} > ${newauth}

# Update the pull-secret information in OCP
oc set data secret pull-secret -n openshift-config --from-file=.dockerconfigjson=${newauth}

# Cleanup
rm -f ${oldauth} ${newauth}
```

###### Install the nightly operator bundle
```
OPERATOR_REGISTRY=brew.registry.redhat.io OPERATOR_REGISTRY_ORG=rh-osbs GITOPS_OPERATOR_VER=v99.9.0-<build_number> IMAGE_PREFIX="openshift-gitops-1-" ./install-gitops-operator.sh -i
```

###### Uninstall the nightly operator bundle
```
./install-gitops-operator.sh -u
```

##### Migrate from an OLM based install to non-OLM based installation (nightly-build)

```
OPERATOR_REGISTRY=brew.registry.redhat.io OPERATOR_REGISTRY_ORG=rh-osbs IMAGE_PREFIX=openshift-gitops-1- GITOPS_OPERATOR_VER=v99.9.0-<build_number> ./install-gitops-operator.sh -m
```
