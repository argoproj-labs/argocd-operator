# Insights

The Argo CD Operator exposes the metrics exported by Argo CD components to be consumed by Prometheus.

## Overview

Argo CD exports many metrics that can be used to monitor and provide insights into the state and health of the cluster. The operator creates ServiceMonitors and PrometheusRules to make these metrics available to Prometheus for scraping.

## Cluster

This section builds on the example minishift cluster from the [OLM Install Guide][olm_guide].

## Prometheus

The Prometheus Operator is available through [operatorhub.io](https://operatorhub.io/operator/prometheus) and is also present in the embedded OpenShift Operator Hub.

Install the Prometheus Operator by creating a Subscription in the same namespace where the Argo CD cluster will reside. An example Subscription can be found below.

``` yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: prometheus-operator
spec:
  channel: beta
  name: prometheus
  source: operatorhubio-catalog
  sourceNamespace: olm
```

Verify that an OperatorGroup is present in the namespace before creating the Subscription.

``` bash
kubectl get operatorgroups -n argocd
```

The OperatorGroup created as part of the [OLM Istall Guide][olm_guide] will work.

``` bash
NAME              AGE
argocd-operator   2m47s
```

With an OperatorGroup present, the Subscription for the Prometheus Operator can be created.

``` bash
kubectl apply -n argocd -f deploy/prometheus.yaml
```

Verify that the Prometheus Operator is running.

``` bash
kubectl get pods
```

The operator should start after several moments.

``` bash
NAME                                  READY   STATUS    RESTARTS   AGE
prometheus-operator-7f6dfb7686-wb9h2  1/1     Running   0          9m4s
```

## Example

The following example shows how to enable metrics exposure for Argo CD components. When enabled, the operator will create ServiceMonitors and PrometheusRules that allow your Prometheus instance to scrape metrics from Argo CD.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: insights
spec:
  ingress:
    enabled: true
  prometheus:
    enabled: true
  server:
    insecure: true
```

With the Prometheus Operator running in the namespace, create the Argo CD cluster using the example above and verify that the cluster is running.

``` bash
kubectl get pods -n argocd
```

You should see the Argo CD component pods:

``` bash
NAME                                                    READY   STATUS    RESTARTS   AGE
example-argocd-application-controller-6c9c8fc6c-27lvv   1/1     Running   0          15h
example-argocd-dex-server-94477bc6f-lzn8m               1/1     Running   0          15h
example-argocd-redis-756b6764-4r2q4                     1/1     Running   0          15h
example-argocd-repo-server-5ddfd76c48-xmwdt             1/1     Running   0          15h
example-argocd-server-65dbd7c68b-kbjgr                  1/1     Running   0          15h
```

Verify that the ServiceMonitors are created for metrics collection:

``` bash
kubectl get servicemonitors -n argocd
```

``` bash
NAME                                    AGE
example-argocd-metrics                  15h
example-argocd-repo-server-metrics      15h
example-argocd-server-metrics           15h
```

If Ingress was enabled, you can access the Argo CD resources via Ingress.

``` bash
kubectl get ing -n argocd
```

This example shows the default hostnames that are configured for the resources.

``` bash
NAME                  CLASS    HOSTS                 ADDRESS         PORTS     AGE
example-argocd        <none>   example-argocd        192.168.39.68   80, 443   15h
example-argocd-grpc   <none>   example-argocd-grpc   192.168.39.68   80, 443   15h
```

For OpenShift clusters, Routes will be created when route is enabled as shown in the below example.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: insights
spec:
  prometheus:
    enabled: true
  server:
    insecure: true
    route:
      enabled: true
```

Password can be obtained from the secret by running the below command.

```
oc -n argocd extract secret/example-argocd-cluster --to=-
```

Refer to the [Ingress Guide][ingress_guide] for further steps on accessing these resources.

## Metrics Service Annotations

The operator supports adding custom annotations to metrics services for all Argo CD components. This is useful for service-based autodiscovery with monitoring tools like Datadog, Dynatrace, or other APM solutions that rely on service annotations for configuration.

**Note:** This feature is for adding annotations to the metrics **Services** (not pods). Since the operator creates ServiceMonitors for Prometheus integration, you typically do not need `prometheus.io/*` annotations. Use this feature when you need service-level annotations for:

- Service-based monitoring autodiscovery (e.g., Datadog, Dynatrace)
- Custom metadata for monitoring platforms
- Integration with service mesh observability tools

### Important: Annotation Reconciliation Behavior

!!! warning "Custom annotations are actively managed"
    The operator treats the `metrics.annotations` field as the **source of truth**. During reconciliation:
    
    - ✅ Annotations specified in `metrics.annotations` will be applied to the service
    - ✅ System annotations (kubernetes.io/*, k8s.io/*, openshift.io/*, service.beta.openshift.io/*, service.alpha.openshift.io/*) are automatically preserved
    - ❌ **Any custom annotations not in the spec will be automatically removed**
    
    This ensures consistency between your ArgoCD CR and the actual service state. If you need to preserve a custom annotation, you must add it to the `metrics.annotations` map in your ArgoCD CR.

**Example:** System annotations like `service.beta.openshift.io/serving-cert-secret-name` are automatically preserved even if not specified in the annotations map.

### Overview

Custom annotations can be applied to the metrics services of the following components:

- Application Controller
- Server
- Repo Server
- Notifications Controller
- ArgoCD Agent (both Agent and Principal)

### Configuration

Annotations are configured through the `metrics.annotations` field in each component's specification.

### Example: Datadog Autodiscovery

The following example shows how to configure Datadog autodiscovery annotations for the Application Controller metrics service:

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  prometheus:
    enabled: true
  controller:
    metrics:
      annotations:
        ad.datadoghq.com/service.check_names: '["openmetrics"]'
        ad.datadoghq.com/service.init_configs: '[{}]'
        ad.datadoghq.com/service.instances: |
          [{
            "openmetrics_endpoint": "http://%%host%%:%%port%%/metrics",
            "namespace": "argocd_controller",
            "metrics": [".*"]
          }]
```

### Example: Multiple Components

You can configure annotations for multiple components simultaneously:

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  prometheus:
    enabled: true
  controller:
    metrics:
      interval: "30s"
      annotations:
        monitoring.example.com/scrape: "true"
        monitoring.example.com/path: "/metrics"
  server:
    metrics:
      interval: "30s"
      annotations:
        monitoring.example.com/scrape: "true"
        monitoring.example.com/path: "/metrics"
  repo:
    metrics:
      annotations:
        monitoring.example.com/scrape: "true"
  notifications:
    enabled: true
    metrics:
      annotations:
        monitoring.example.com/scrape: "true"
```

### Example: Custom Monitoring Metadata

For custom monitoring solutions that require metadata annotations:

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  prometheus:
    enabled: true
  controller:
    metrics:
      annotations:
        monitoring.example.com/team: "platform"
        monitoring.example.com/environment: "production"
        monitoring.example.com/service-tier: "critical"
```

### Verification

After applying the configuration, verify that the annotations are present on the metrics services:

``` bash
kubectl get service example-argocd-metrics -n argocd -o json | jq '.metadata.annotations'
```

Expected output:
``` json
{
  "ad.datadoghq.com/service.check_names": "[\"openmetrics\"]",
  "ad.datadoghq.com/service.init_configs": "[{}]",
  "ad.datadoghq.com/service.instances": "[{\"openmetrics_endpoint\":\"http://%%host%%:%%port%%/metrics\",\"namespace\":\"argocd_controller\",\"metrics\":[\".*\"]}]"
}
```

### Updating Annotations

To update or remove annotations, modify the ArgoCD CR and the operator will reconcile the changes:

- **Adding annotations**: Add new key-value pairs to the `metrics.annotations` map
- **Updating annotations**: Change the value for an existing key
- **Removing annotations**: Delete the key from the `metrics.annotations` map (or set `metrics.annotations` to `{}` to remove all custom annotations)

The operator will automatically update the service annotations during the next reconciliation cycle.

### Notes

- All custom annotations not present in the spec will be removed during reconciliation
- System annotations (kubernetes.io/*, k8s.io/*, openshift.io/*, service.beta.openshift.io/*, service.alpha.openshift.io/*) are always preserved
- Annotation values can contain JSON strings for complex configurations
- Empty or nil `metrics.annotations` maps will result in all custom annotations being removed

[olm_guide]:../install/olm.md
[ingress_guide]:./ingress.md#access
