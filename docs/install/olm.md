# OLM Install

The following steps can be used to install the operator using the [Operator Lifecycle Manager][olm_home] on any Kubernetes 
environment with minimal overhead.

## Cluster Setup

This guide uses [minikube](https://minikube.sigs.k8s.io/) to deploy a Kubernetes cluster locally, follow the 
instructions for your platform to install. If you already have a Kubernetes cluster ready to go, skip to 
the [OLM](#operator-lifecycle-manager) section.

Run minikube with a dedicated profile. Adjust the system resources as needed for your platform. 

```bash
minikube start -p argocd --cpus=4 --disk-size=40gb --memory=8gb
```

## Operator Lifecycle Manager

Install the OLM components manually. If you already have OLM installed, skip to the [Operator](#operator-install) section.

```bash
kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.12.0/crds.yaml
kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.12.0/olm.yaml
```

Verify that OLM is installed. There should be two new namespaces, `olm` and `operators` created as a result.

```bash
kubectl get ns
```

```bash
NAME              STATUS   AGE
default           Active   15m
kube-node-lease   Active   15m
kube-public       Active   15m
kube-system       Active   15m
olm               Active   24s
operators         Active   24s
```

Verfiy that the OLM Pods are running in the `olm` namespace.

```bash
kubectl get pods -n olm
```

```bash
NAME                                READY   STATUS    RESTARTS   AGE
catalog-operator-7dfcfcb46b-xxjdq   1/1     Running   0          3m1s
olm-operator-76d446f94c-pn6kx       1/1     Running   0          3m1s
operatorhubio-catalog-h4hc8         1/1     Running   0          2m46s
packageserver-fb649b58c-96wh6       1/1     Running   0          2m45s
packageserver-fb649b58c-qn8vx       1/1     Running   0          2m45s
```

That's it, OLM should be installed and availble to manage the Argo CD Operator.

## Operator Install

Use the following steps to install the operator using an OLM Catalog.

### Namespace

Create a new namespace for the operator.

```bash
kubectl create namespace argocd
```

### Operator Catalog

Create a `CatalogSource` in the `olm` namespace. This manifest references a container image that has the Argo CD 
Operator packaged for use in OLM. For more information on packaging the operator, see the [development][docs_dev] documentation.

```bash
kubectl create -n olm -f deploy/catalog_source.yaml
```

Verify that the Argo CD operator catalog has been created.

```bash
kubectl get catalogsources -n olm
```

```bash
NAME                    DISPLAY               TYPE   PUBLISHER        AGE
argocd-catalog          Argo CD Operators     grpc   Argo CD          6s
operatorhubio-catalog   Community Operators   grpc   OperatorHub.io   25m
```

Verify that the registry Pod that serves the catalog is running.

```bash
kubectl get pods -n olm -l olm.catalogSource=argocd-catalog
```

```bash
NAME                   READY   STATUS    RESTARTS   AGE
argocd-catalog-nxn79   1/1     Running   0          55s
```

### Operator Group

Create an `OperatorGroup` in the `argocd` namespace that defines the namespaces that the Argo CD Operator will watch for 
new resources.

```bash
kubectl create -n argocd -f deploy/operator_group.yaml
```

Verify that the new OperatorGroup was created in the `argocd` namespace.

```bash
kubectl get operatorgroups -n argocd
```

```bash
NAME              AGE
argocd-operator   10s
```

### Subscription

Once the OperatorGroup is present, create a new `Subscription` for the Argo CD Operator in the new `argocd` namespace.

```bash
kubectl create -n argocd -f deploy/subscription.yaml
```

Verify that the Subscription was created in the `argocd` namespace.
```bash
kubectl get subscriptions -n argocd
```

```bash 
NAME              PACKAGE           SOURCE           CHANNEL
argocd-operator   argocd-operator   argocd-catalog   alpha
```

The Subscription should result in an `InstallPlan` being created in the `argocd` namespace.

```bash
kubectl get installplans -n argocd
```

```bash
NAME            CSV                      APPROVAL    APPROVED
install-slbfr   argocd-operator.v0.0.2   Automatic   true
```

Finally, verify that the Argo CD Operator Pod is running in the `argocd` namespace.

```bash
kubectl get pods -n argocd
```

```
NAME                               READY   STATUS    RESTARTS   AGE
argocd-operator-746b886cd5-cd7m7   1/1     Running   0          4m27s
```

## Usage 

Once the operator is installed and running, see the [usage][docs_usage] documentation on how to create new `ArgoCD` resources.

[docs_dev]:../development.md
[docs_usage]:../usage.md
[olm_home]:https://github.com/operator-framework/operator-lifecycle-manager


## Cleanup 

kubectl delete -n argocd -f deploy/subscription.yaml
kubectl delete -n argocd -f deploy/operator_group.yaml
kubectl delete -n olm -f deploy/catalog_source.yaml
kubectl delete namespace argocd