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

Image Updater is disabled by default. Enabling image updater results in the operator creating the following resources on the cluster:

*  `<argocd-instance-name>-argocd-image-updater-controller` deployment
*  `<argocd-instance-name>-argocd-image-updater-controller` serviceAccount
*  `<argocd-instance-name>-argocd-image-updater-controller` role
*  `<argocd-instance-name>-argocd-image-updater-controller` roleBinding
*  `<argocd-instance-name>-default-argocd-image-updater-controller` clusterRole
*  `<argocd-instance-name>-default-argocd-image-updater-controller` clusterRoleBinding
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
