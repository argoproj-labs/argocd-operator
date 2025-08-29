# Environment Variables

The following environment variables are available in `argocd-operator`:

| Environment Variable | Default Value | Description |
| --- | --- | --- |
| `CONTROLLER_CLUSTER_ROLE` | none | Administrators can configure a common cluster role for all the managed namespaces in role bindings for the Argo CD application controller with this environment variable. Note: If this environment variable contains custom roles, the Operator doesn't create the default admin role. Instead, it uses the existing custom role for all managed namespaces. |
| `SERVER_CLUSTER_ROLE` | none | Administrators can configure a common cluster role for all the managed namespaces in role bindings for the Argo CD server with this environment variable. Note: If this environment variable contains custom roles, the Operator doesn’t create the default admin role. Instead, it uses the existing custom role for all managed namespaces. |
| `REMOVE_MANAGED_BY_LABEL_ON_ARGOCD_DELETION` | false | When an Argo CD instance is deleted, namespaces managed by that instance (via the `argocd.argoproj.io/managed-by` label ) will retain the label by default. Users can change this behavior by setting the environment variable `REMOVE_MANAGED_BY_LABEL_ON_ARGOCD_DELETION` to `true` in the Subscription. |
| `ARGOCD_LABEL_SELECTOR` | none | The label selector can be set on argocd-opertor by exporting `ARGOCD_LABEL_SELECTOR` (eg: `export ARGOCD_LABEL_SELECTOR=foo=bar`). The labels can be added to the argocd instances using the command `kubectl label argocd test1 foo=bar -n test-argocd`. This will enable the operator instance to be tailored to oversee only the corresponding ArgoCD instances having the matching label selector. |
| `LOG_LEVEL` | info | This sets the logging level of the manager (operator) pod. Valid values are "debug", "info", "warn", "error", "panic" and "fatal". |

Custom Environment Variables are supported in `applicationSet`, `controller`, `notifications`, `repo` and `server` components. For example:

```
...
kind: ArgoCD
metadata:
  name: argocd
  labels:
    example: controller
spec:
  controller:
    resources: {}
    env:
      - name: FOO
        value: bar
...
```

The following default value of images could be overridden by setting the environment variables:

| Environment Variable | Default Value |
| --- | --- |
| `ARGOCD_IMAGE` | [quay.io/argoproj/argocd](quay.io/argoproj/argocd) |
| `ARGOCD_DEX_IMAGE` | [ghcr.io/dexidp/dex](ghcr.io/dexidp/dex) |
| `ARGOCD_REDIS_IMAGE` | redis |
| `ARGOCD_REDIS_HA_IMAGE` | redis |
| `ARGOCD_REDIS_HA_PROXY_IMAGE` | haproxy |
