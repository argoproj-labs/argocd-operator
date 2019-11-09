## Basic Install

The following steps can be used to manually install the operator on any Kubernetes environment with minimal overhead.

Several of the steps in this process require the `cluster-admin` ClusterRole or equivalent.

### Cluster

This guide uses [minikube](https://minikube.sigs.k8s.io/) to deploy a Kubernetes cluster locally, follow the 
guide for your platform to install. 

Run minikube and adjust resources as needed for your platform. 

```bash
minikube start -p argocd --cpus=4 --disk-size=40gb --memory=8gb
```

### Namespace

It is a good idea to create a new namespace for the operator.

```bash
kubectl create namespace argocd
```

Once the namespace is created, set up the local context to use the new namespace.

```bash
kubectl config set-context argocd/minikube --cluster argocd --namespace argocd --user argocd
kubectl config use-context argocd/minikube
```

The remaining resources will now be created in the new namespace.

### RBAC

Set up RBAC for the ArgoCD operator and components.

```bash
kubectl create -f deploy/service_account.yaml
kubectl create -f deploy/role.yaml
kubectl create -f deploy/role_binding.yaml
```

### CRDs

Add the ArgoCD CRDs to the cluster.

```bash
kubectl create -f deploy/argo-cd
```

Add the ArgoCD Operator CRD to the cluster

```bash
kubectl create -f deploy/crds/argoproj_v1alpha1_argocd_crd.yaml
```

There should be three CRDs present for ArgoCD on the cluster.

```bash
kubectl get crd
```

```bash
NAME                       CREATED AT
applications.argoproj.io   2019-11-09T02:35:47Z
appprojects.argoproj.io    2019-11-09T02:35:47Z
argocds.argoproj.io        2019-11-09T02:36:02Z
```

### Deploy Operator

Deploy the operator

```bash
kubectl create -f deploy/operator.yaml
```

The operator pod should start and enter a `Running` state after a few seconds.

```bash
kubectl get pods
```

```bash
NAME                              READY   STATUS    RESTARTS   AGE
argocd-operator-758dd86fb-sx8qj   1/1     Running   0          75s
```

### ArgoCD

Once the operator is deployed and running, create a new ArgoCD custom resource.
The following example shows the minimal required to create a new ArgoCD
environment with the default configuration.

```bash
kubectl create -f examples/argocd-minimal.yaml
```

There will be several resources created.

```bash
kubectl get pods
```
```bash
NAME                                                     READY   STATUS    RESTARTS   AGE
argocd-minimal-application-controller-7c74b5855b-brn7s   1/1     Running   0          29s
argocd-minimal-dex-server-859bd5458c-78c8k               1/1     Running   0          29s
argocd-minimal-redis-6986d5fdbd-vzzjp                    1/1     Running   0          29s
argocd-minimal-repo-server-7bfc477c58-q7d8g              1/1     Running   0          29s
argocd-minimal-server-7d56c5bf4d-9wxz6                   1/1     Running   0          29s
argocd-operator-758dd86fb-qshll                          1/1     Running   0          51s
```

The ArgoCD Server should be available via a Service.

```bash
kubectl get svc
```

```bash
NAME                            TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)             AGE
argocd-minimal-dex-server       ClusterIP   10.105.36.155    <none>        5556/TCP,5557/TCP   2m28s
argocd-minimal-metrics          ClusterIP   10.102.88.192    <none>        8082/TCP            2m28s
argocd-minimal-redis            ClusterIP   10.101.29.123    <none>        6379/TCP            2m28s
argocd-minimal-repo-server      ClusterIP   10.103.229.32    <none>        8081/TCP,8084/TCP   2m28s
argocd-minimal-server           ClusterIP   10.100.186.222   <none>        80/TCP,443/TCP      2m28s
argocd-minimal-server-metrics   ClusterIP   10.100.185.144   <none>        8083/TCP            2m28s
argocd-operator-metrics         ClusterIP   10.97.124.166    <none>        8383/TCP,8686/TCP   23m
kubernetes                      ClusterIP   10.96.0.1        <none>        443/TCP             44m
```

Forward the server port to the local machine.

```bash
kubectl port-forward service/argocd-minimal-server 8443:443
```

The server UI should be available at https://localhost:8443/ and the admin
password is the name for the server Pod from above (`argocd-minimal-server-7d56c5bf4d-9wxz6` in this example).

Follow the ArgoCD [Getting Started Guide](https://argoproj.github.io/argo-cd/getting_started/#creating-apps-via-ui) 
to create a new application from the UI.
