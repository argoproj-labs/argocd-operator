# Insights

The operator exposes the metrics exported by Argo CD using Prometheus and Grafana.

Currently, the Argo CD operator deploys and manages Grafana itself. However for Prometheus, the operator relies on the
Prometheus Operator, which must be installed. Once installed the Argo CD operator will create Prometheus resources that
will be managed by the Prometheus operator.

## Example

The following example shows how to enable prometheus and grafana to provide operator insights.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: insights
spec:
  grafana:
    enabled: true
  prometheus:
    enabled: true
```
