# Metrics

The Argo CD Operator exposes performance metrics over HTTPS on port `8443`.
It also creates a metrics service named `argocd-operator-controller-manager-metrics-service` in the `argocd-operator-system` namespace.

The `/metrics` endpoint is protected using controller-runtime authentication and authorization.

The operator's ServiceAccount needs permission to create:

- `tokenreviews.authentication.k8s.io`
- `subjectaccessreviews.authorization.k8s.io`

These permissions are included by default in `config/rbac/auth_proxy_role.yaml`.

Any metrics scraper (for example Prometheus) also needs permission to `GET /metrics`.
The project provides a `ClusterRole` named `metrics-reader` (`config/rbac/auth_proxy_client_clusterrole.yaml`) for this purpose.
Bind it to your scraper ServiceAccount, for example:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: prometheus-metrics-reader
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: metrics-reader
subjects:
  - kind: ServiceAccount
    name: prometheus-k8s
    namespace: monitoring
```

The metrics exposed by the operator currently are:

- `active_argocd_instances_total` [Gauge] - This metric tracks the total number of active Argo CD instances managed by the operator at a given time
- `active_argocd_instances_by_phase{phase=\"<phase>\"}` [Gauge] - This metric tracks the count of active Argo CD instances by phase (`Available`, `Pending`, `Failed`, `Unknown`)
- `active_argocd_instance_reconciliation_count{namespace=\"<argocd-instance-ns>\"}` [Counter] - This metric tracks the total number of reconciliations that have occurred for the instance in the given namespace at any given point in time
- `controller_runtime_reconcile_time_seconds_per_instance_bucket{namespace=\"<argocd-instance-ns>\",le=\"0.5\"}` [Histogram]- This metric tracks the number of reconciliations that took under 0.5s to complete for a given instance. The operator has a set of pre-configured buckets.
