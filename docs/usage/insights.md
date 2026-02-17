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

[olm_guide]:../install/olm.md
[ingress_guide]:./ingress.md#access
