# GitOps Promoter

The [GitOps Promoter](https://gitops-promoter.readthedocs.io/) can be deployed as an optional workload through cluster-scoped instances reconciled by the Argo CD operator.
This allows for environment promotion for GitOps via the rendered manifests pattern. This pattern can be accomplished by using Argo CD's [Source Hydrator](https://argocd-operator.readthedocs.io/en/latest/reference/argocd/#source-hydrator-options).

## Installation

The GitOps Promoter's resources can be enabled/disabled via the `.spec.promoter` field. An example of the most basic installation:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-cluster-scoped-argocd-with-promoter-enabled
  namespace: some-ns
spec:
  promoter:
    enabled: true
```

As mentioned above, the Argo CD CR **must** be cluster-scoped to use the GitOps Promoter. Instructions on how to enable a cluster-scoped Argo CD can be found on [this](../basics/#cluster-scoped-instance) docs page.

There are more configuration settings that can be found on the [API Spec](../reference/api-v1beta1/#promoterspec) page.

### Resources Created

The following resources are created when the Promoter is enabled:

**Controller Manager resources (created when promoter is enabled):**

- `<instance-name>-promoter-controller-manager` serviceAccount
- `<instance-name>-<instance-namespace>-promoter-controller-manager` clusterRole (not created if PROMOTER_CONTROLLER_CLUSTER_ROLE env is set on the operator)
- `<instance-name>-<instance-namespace>-promoter-controller-manager` clusterRoleBinding
- `promoter-controller-configuration` controllerConfiguration
- `<instance-name>-promoter-controller-manager` deployment
- `<instance-name>-promoter-controller-manager` service (only when .spec.promoter.webhook.enabled: true)

**API Server resources (created when promoter is enabled, unless .spec.promoter.apiserver.enabled: false):**

- `<instance-name>-promoter-apiserver` serviceAccount
- `<instance-name>-<instance-namespace>-promoter-apiserver` clusterRole (not created if PROMOTER_API_SERVER_CLUSTER_ROLE env is set on the operator)
- `<instance-name>-<instance-namespace>-promoter-apiserver-promotionstrategydetails-viewer` clusterRole
- `<instance-name>-<instance-namespace>-promoter-apiserver` clusterRoleBinding
- `<instance-name>-<instance-namespace>-promoter-apiserver-promotionstrategydetails-viewer` clusterRoleBinding
- `<instance-name>-<instance-namespace>-promoter-apiserver-auth-delegator` clusterRoleBinding
- `<instance-name>-<instance-namespace>-promoter-apiserver-extension-auth-reader` roleBinding (in kube-system namespace)
- `<instance-name>-promoter-apiserver` service
- `v1alpha1.view.promoter.argoproj.io` apiService
- `<instance-name>-promoter-apiserver` deployment

### TLS for API Server

The API Server requires certificates for the APIService. Currently these must be manually provided (this will change in the future).

A CA is required for the APIService and then a certificate generated from that CA is required for the deployment of the API Server.

To provide TLS certificates, the field `.spec.promoter.apiserver.tls` is used. An example of how to configure this is shown below, along with the Secrets used to store the certificates:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-promoter-apiserver-tls
  namespace: some-ns
spec:
  promoter:
    enabled: true
    apiserver:
      tls:
        certSecretName: tls-secret
        caSecretName: ca-cert-secret
        caSecretKey: ca.crt # ca.crt is the default value and does not need to be provided. If anything else is used it must be set to that.
---
apiVersion: v1
kind: Secret
metadata:
  name: tls-secret
  namespace: some-ns
type: kubernetes.io/tls
data:
  tls.crt: <base64-encoded-tls-cert>
  tls.key: <base64-encoded-tls-key>
---
apiVersion: v1
kind: Secret
metadata:
  name: ca-cert-secret
  namespace: some-ns
type: Opaque
data:
  ca.crt: <base64-encoded-ca-cert>
```

### Argo CD UI Extension

The GitOps Promoter has an Argo CD UI Extension that allows for information on Promoter resources that are deployed by Argo CD applications to be visible in the UI.
For more information see the [upstream docs page](https://gitops-promoter.readthedocs.io/en/latest/integrating-with-argocd/).

This can be enabled with the following options set.

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-promoter-with-ui-extension
  namespace: some-ns
spec:
  promoter:
    enabled: true
    argoCDUIExtensionEnabled: true
```

## Uninstallation

The GitOps Promoter can be disabled by setting the `.spec.promoter.enabled` to `false`. When moving to false all resources created to run the Promoter will be deleted.

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-uninstall-promoter
  namespace: some-ns
spec:
  promoter:
    enabled: false
```