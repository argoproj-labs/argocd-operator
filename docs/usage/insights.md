# Insights

The Argo CD Operator aggregates, visualizes and exposes the metrics exported by Argo CD using Prometheus and Grafana.

## Overview

Argo CD exports many metrics that can be used to monitor and provide insights into the state and health of the cluster. The operator makes use of the Prometheus Operator to provision a Prometheus instance for string the metrics from both Argo CD and the operator itself.

Currently the Argo CD operator deploys and manages Grafana using a Deployment and does not make use of the Grafana Operator just yet. There are several dashboards provided for visualizing the Argo CD environment.

## Cluster

This section builds on the example minishift cluster from the [OLM Istall Guide][olm_guide]. 

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

The following example shows how to enable Prometheus and Grafana to provide operator insights. This example also enables Ingress for accessing the cluster resources.

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
  ingress:
    enabled: true
  prometheus:
    enabled: true
  server:
    insecure: true
```

With the Prometheus Operator running in the namespace, create the Argo CD cluster using the example above and verify that the cluster is running.

``` Bash
kubectl get pods -n argocd
```

Look for the Grafana and Prometheus Pods.

``` bash
NAME                                                    READY   STATUS    RESTARTS   AGE
argocd-operator-5fc46479bd-pp9b2                        1/1     Running   0          15h
example-argocd-application-controller-6c9c8fc6c-27lvv   1/1     Running   0          15h
example-argocd-dex-server-94477bc6f-lzn8m               1/1     Running   0          15h
example-argocd-grafana-86ccc6bf9c-l8dqw                 1/1     Running   0          15h
example-argocd-redis-756b6764-4r2q4                     1/1     Running   0          15h
example-argocd-repo-server-5ddfd76c48-xmwdt             1/1     Running   0          15h
example-argocd-server-65dbd7c68b-kbjgr                  1/1     Running   0          15h
prometheus-example-argocd-0                             3/3     Running   1          14m
prometheus-operator-7f6dfb7686-wb9h2                    1/1     Running   0          14m
```

Grafana and Prometheus can be accessed via Ingress resources.

``` bash
kubectl get ing -n argocd
```

This example shows the default hostnames that are configured for the resources.

``` bash
NAME                        CLASS    HOSTS                       ADDRESS         PORTS     AGE
example-argocd              <none>   example-argocd              192.168.39.68   80, 443   15h
example-argocd-grafana      <none>   example-argocd-grafana      192.168.39.68   80, 443   15h
example-argocd-grpc         <none>   example-argocd-grpc         192.168.39.68   80, 443   15h
example-argocd-prometheus   <none>   example-argocd-prometheus   192.168.39.68   80, 443   15h
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
  grafana:
    enabled: true
    route:
      enabled: true
  prometheus:
    enabled: true
    route:
      enabled: true
  server:
    insecure: true
    route:
      enabled: true
```

Initial Password for grafana is stored in the example-argocd-cluster secret. Password can be obtained from the secret by running the below command.
```
oc -n argocd extract secret/example-argocd-cluster --to=-
```

Refer to the [Ingress Guide][ingress_guide] for further steps on accessing these resources.

[olm_guide]:../install/olm.md
[ingress_guide]:./ingress.md#access
