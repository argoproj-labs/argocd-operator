# Enabling/Disabling ArgoCD core components

The Operator manages all Argo CD workloads, such as API server, repository server, application controller, etc.

Right now, the following workloads are managed:

* argocd-server (API server and UI)
* argocd-repo-server (Repository server)
* argocd-application-controller (Main reconciliation controller)
* argocd-applicationset-controller (ApplicationSet reconciliation controller)
* argocd-redis (volatile cache)

To support installations with a minimal resource footprint and to distribute installations across clusters or namespaces, we need to be able to only install a partial Argo CD.

In order to enable/disable a specific Argo CD workload, a new flag `spec.<component>.enabled` has been introduced. The default value of the flag is `true`, which means that even if the flag is not specified, the ArgoCD workload would be enabled by default.

For disabling a specific Argo CD component, set the `spec.<component>.enabled` flag to `false`. The flag is available for components - controller, repo, server, redis, applicationset.

Let's consider the below example,

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example
spec:
  controller:
    enabled: false
```

In the above example, only the `controller` component is disabled and all the other components will continue running as normal.


# Providing external URL for Redis and RepoServer components

While disabling the core components like `Redis` or `Repo Server`, you might want to specify an external URL for the components running in external clusters. The remote URL can now be set using the flag `spec.<component>.Remote` (where component can only be `redis` or `repo`).

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

!!! Note: The `remote` flag can only be set if the `enabled` flag is set as `false`.