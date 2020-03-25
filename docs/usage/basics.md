# Usage Basics

See the [ArgoCD Reference][argocd_reference] for the full list of properties and defaults to configure the Argo CD cluster.

The following example shows the most minimal valid manifest to create a new Argo CD cluster with the default configuration.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: basic
spec: {}
```

## Create

Create a new Argo CD cluster using the provided basic example in the `argocd` namespace.

```bash
kubectl create -n argocd -f examples/argocd-basic.yaml
```

There will be several Argo CD resources created that should be familiar to anyone who has deployed Argo CD.

```bash
kubectl get cm,pod -n argocd
```
```bash
NAME                                  DATA   AGE
configmap/argocd-cm                   0      55m
configmap/argocd-rbac-cm              0      55m
configmap/argocd-ssh-known-hosts-cm   1      55m
configmap/argocd-tls-certs-cm         0      55m

NAME                                                         READY   STATUS    RESTARTS   AGE
pod/example-argocd-application-controller-7c74b5855b-ssz6h   1/1     Running   0          55m
pod/example-argocd-dex-server-859bd5458c-zpgtg               1/1     Running   0          55m
pod/example-argocd-redis-6986d5fdbd-76gjf                    1/1     Running   0          55m
pod/example-argocd-repo-server-7bfc477c58-hv9gp              1/1     Running   0          55m
pod/example-argocd-server-7d56c5bf4d-r5brr                   1/1     Running   0          55m
```

The ArgoCD Server component should be available via a Service.

```bash
kubectl get svc -n argocd
```
```bash
NAME                            TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)             AGE
example-argocd-dex-server       ClusterIP   10.105.36.155    <none>        5556/TCP,5557/TCP   2m28s
example-argocd-metrics          ClusterIP   10.102.88.192    <none>        8082/TCP            2m28s
example-argocd-redis            ClusterIP   10.101.29.123    <none>        6379/TCP            2m28s
example-argocd-repo-server      ClusterIP   10.103.229.32    <none>        8081/TCP,8084/TCP   2m28s
example-argocd-server           ClusterIP   10.100.186.222   <none>        80/TCP,443/TCP      2m28s
example-argocd-server-metrics   ClusterIP   10.100.185.144   <none>        8083/TCP            2m28s
argocd-operator-metrics         ClusterIP   10.97.124.166    <none>        8383/TCP,8686/TCP   23m
```

## Server API & UI

The Argo CD server component exposes the API and UI. The operator creates a Service to expose this component and 
can be accessed through the various methods available in Kubernetes.

### Local Machine

In the most simple case, the Service port can be forwarded to the local machine.

```bash
kubectl port-forward service/example-argocd-server 8443:443
```

The server UI should be available at https://localhost:8443/ and the admin password is the name for the Argo CD server 
Pod (`example-argocd-server-7d56c5bf4d-r5brr` in this example).

### Ingress

See the [ingress][docs_ingress] documentation for steps to enable and use the Ingress support provided by the operator. 

### OpenShift Route

See the [OpenShift][docs_openshift] documentation for steps to configure the Route support provided by the operator.

[docs_ingress]:./ingress.md
[docs_openshift]:../install/openshift.md
[argocd_reference]:../reference/argocd.md
