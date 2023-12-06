# Enabling/Disabling Core Components of ArgoCD
The Operator oversees all Argo CD workloads, including the API server, repository server, application controller, and more.

Currently, the following workloads are managed:

* argocd-server (API server and UI)
* argocd-repo-server (Repository server)
* argocd-application-controller (Main reconciliation controller)
* argocd-applicationset-controller (ApplicationSet reconciliation controller)
* argocd-redis (volatile cache)

To support installations with minimal resource requirements and to facilitate distribution across clusters or namespaces, the ability to selectively install specific Argo CD components has been introduced.

To enable/disable a particular Argo CD workload, a new flag, `spec.<component>.enabled`, has been implemented. The default value of the flag is `true`, implying that if the flag is unspecified, the Argo CD workload is enabled by default.

To disable a specific Argo CD component, set the `spec.<component>.enabled` flag to `false`.

Consider the following example:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example
spec:
  controller:
    enabled: false
```

In this example, only the controller component is disabled, while all other components continue to run normally.

# Specifying External URLs for Redis and RepoServer Components
When disabling core components like Redis or Repo Server, you may wish to provide an external URL for components running in external clusters. The remote URL can be set using the `spec.<component>.remote` flag (where the component can only be `redis` or `repo`).

For example,

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example
spec:
  repo:
    enabled: false
    remote: 'https://www.example.com/repo-server'
```

!!! Note: The remote flag can only be set if the enabled flag of the component is set to false.