# Metrics

The Argo CD Operator exposes a set of performance metrics at port `8080`. It also creates a metrics-service called `argocd-operator-controller-manager-metrics-service`, present in the `argocd-operator-system` namespace, which makes the metrics available on port `8443`

The metrics exposed by the operator currently are:
- `active_argocd_instances_total` [Guage] - This metric produces the graph that tracks the total number of active argo-cd instances being managed by the operator at a given time
- `active_argocd_instances_by_phase{phase=\"<phase>\"}` [Guage] - This metric produces the graph that tracks the count of active Argo CD instances by their phase [Available/Pending/Failed/unknown]
- `active_argocd_instance_reconciliation_count{namespace=\"<argocd-instance-ns>\"}` [Counter] - This metric produces the graph that tracks total number of reconciliations that have occurred for the instance in the given namespace at any given point in time
- `controller_runtime_reconcile_time_seconds_per_instance_bucket{namespace=\"<argocd-instance-ns>\",le=\"0.5\"}` [Histogram]- This metric tracks the number of reconciliations that took under 0.5s to complete for a given instance. The operator has a set of pre-configured buckets.