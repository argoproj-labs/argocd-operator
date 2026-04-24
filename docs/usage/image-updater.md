## Overview

[Argo CD Image Updater](https://argocd-image-updater.readthedocs.io/) controller is now available as an optional workload that can be configured through the Argo CD operator. 

## Installation

Argo CD Image Updater controller can be enabled/disabled using a new toggle within the Argo CD CR with default specs as follows:

``` yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  imageUpdater:
    enabled: true
```

Users may also specify advanced configuration such as the resource requirements or environment variables for the image updater controller. The full list of available settings can be found in the [API spec](../reference/argocd.md#image-updater-controller-options).

### Watch scope

The Image Updater controller's watch scope determines which namespaces it monitors for `ImageUpdater` CRs and Argo CD `Applications`. It is controlled by the `IMAGE_UPDATER_WATCH_NAMESPACES` environment variable, set via `.spec.imageUpdater.env`.

The operator supports two installation modes:

#### Install into the Argo CD namespace (recommended, default)

The controller runs in the same namespace as the Argo CD instance and watches only that namespace. This is the default behavior when `IMAGE_UPDATER_WATCH_NAMESPACES` is not set or is empty.

If you use Argo CD's [Applications in any namespace](https://argo-cd.readthedocs.io/en/stable/operator-manual/app-any-namespace/) feature and have `Application` resources in additional namespaces, you can specify a comma-separated list of namespaces to watch:

``` yaml
spec:
  imageUpdater:
    enabled: true
    env:
      - name: IMAGE_UPDATER_WATCH_NAMESPACES
        value: "app-ns1,app-ns2"
```

The operator will create a `Role` and `RoleBinding` in each listed namespace so the controller can access resources there.

#### Cluster-scoped installation

For environments where the controller must watch `ImageUpdater` CRs in all namespaces, set `IMAGE_UPDATER_WATCH_NAMESPACES` to `"*"`:

``` yaml
spec:
  imageUpdater:
    enabled: true
    env:
      - name: IMAGE_UPDATER_WATCH_NAMESPACES
        value: "*"
```

This mode requires the ArgoCD instance to be installed in a cluster configuration namespace. The operator will create a `ClusterRole` and `ClusterRoleBinding` instead of namespace-scoped RBAC.

!!! note
    Installing Image Updater into a namespace separate from the Argo CD namespace (upstream Option 2) is **not supported** by this operator. Refer to the [upstream documentation](https://argocd-image-updater.readthedocs.io/) for details on that installation pattern.

### Resources created

The resources created depend on the selected watch scope:

**namespace-scoped (default or comma-separated namespaces):**

*  `<instance-name>-argocd-image-updater-controller` deployment
*  `<instance-name>-argocd-image-updater-controller` serviceAccount
*  `<instance-name>-argocd-image-updater-controller` role (in the ArgoCD namespace)
*  `<instance-name>-argocd-image-updater-controller` roleBinding (in the ArgoCD namespace)
*  `<instance-name>_<namespace>` role + roleBinding (one per extra namespace, when a comma-separated list is provided)
*  `argocd-image-updater-config` configmap
*  `argocd-image-updater-ssh-config` configmap
*  `argocd-image-updater-secret` secret

**cluster-scoped (`IMAGE_UPDATER_WATCH_NAMESPACES="*"`):**

*  `<instance-name>-argocd-image-updater-controller` deployment
*  `<instance-name>-argocd-image-updater-controller` serviceAccount
*  `<instance-name>-argocd-image-updater-controller` role (in the ArgoCD namespace)
*  `<instance-name>-argocd-image-updater-controller` roleBinding (in the ArgoCD namespace)
*  `<instance-name>-<instance-namespace>-argocd-image-updater-controller` clusterRole
*  `<instance-name>-<instance-namespace>-argocd-image-updater-controller` clusterRoleBinding
*  `argocd-image-updater-config` configmap
*  `argocd-image-updater-ssh-config` configmap
*  `argocd-image-updater-secret` secret

The operator creates the `argocd-image-updater-config` and `argocd-image-updater-ssh-config` configmaps that are editable to users, and will not be reconciled/overwritten by the operator. The `argocd-image-updater-secret` is an empty secret that can be used to configure credentials for the supported image updater services.

Instructions for appropriate configuration of these resources can be found within [upstream documentation](https://argocd-image-updater.readthedocs.io/).

## Uninstallation

Argo CD Image Updater controller can be disabled by setting `.spec.imageUpdater.enabled` to `false` :

``` yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  imageUpdater:
    enabled: false
```
This will clean up all the aforementioned image updater controller resources that were created by the operator.
