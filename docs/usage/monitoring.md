# Instance workload monitoring 

Instance workload monitoring allows you to be better informed of the state of your Argo CD instance by enabling alerts for the instance. Alerts are triggered when the instance's component workload (application-controller, repo-server, server etc.) pods are unable to come up for whatever reason and there is a drift between the number of ready replicas and the number of desired replicas for a certain period of time.  

**Note:** Pre-requisites for this feature:
- Your cluster is configured with Prometheus, and the Argo CD instance is in a namespace that can be monitored via Prometheus
- Your cluster has the `kube-state-metrics` service running 

**Note:** This feature is only concerned with availability of Argo CD workloads, and does not focus on monitoring for performance. 


Instance workload monitoring can be enabled by setting `.spec.monitoring.enabled` to `true` on a given Argo CD instance.
For example:

```
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: example-argocd
spec:
  ...
  monitoring:
    enabled: true
  ...
```

Instance workload monitoring is set to `false` by default.

Enabling this setting allows the operator to create a `PrometheusRule` containing pre-configured alert rules for all the workloads 9statefulsets/deployments) managed by the instance. Here is a sample alert rule included in the PrometheusRule created by the operator:

```
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: argocd-component-status-alert
  namespace: $NAMESPACE
spec:
  groups:
    - name: ArgoCDComponentStatus
      rules:
        ...
        - alert: ApplicationSetControllerNotReady
          annotations:
            message: >-
              applicationSet controller deployment for Argo CD instance in
              namespace "default" is not running
          expr: >-
            kube_deployment_status_replicas{deployment="argocd-applicationset-controller",
            namespace="default"} !=
            kube_deployment_status_replicas_ready{deployment="argocd-applicationset-controller",
            namespace="default"} 
          for: 10m
          labels:
            severity: warning
        ...
```

Users are free to modify/delete alert rules as wish, changes made to the rules will not be overwritten by the operator. 

Instance workload monitoring can be disabled by setting `.spec.monitoring.enabled` to `false` on a given Argo CD instance.
For example:

```
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: example-argocd
spec:
  ...
  monitoring:
    enabled: false
  ...
```

Disabling workload monitoring will delete the created PrometheusRule. 
