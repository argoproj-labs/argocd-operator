# Manual Installation

The following steps can be used to manually install the operator on any Kubernetes environment with minimal overhead.

!!! info
    Several of the steps in this process require the `cluster-admin` ClusterRole or equivalent.

## Cluster

This guide uses [minikube](https://minikube.sigs.k8s.io/) to deploy a Kubernetes cluster locally, follow the 
instructions for your platform to install. 

Run minikube with a dedicated profile. Adjust the system resources as needed for your platform. 

```bash
minikube start -p argocd --cpus=4 --disk-size=40gb --memory=8gb
```

## Manual Install

The following section outlines the steps necessary to deploy the ArgoCD Operator manually using standard Kubernetes 
manifests.

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

Add the upstream Argo CD CRDs to the cluster.

```bash
kubectl create -f deploy/argo-cd
```

Add the ArgoCD Operator CRDs to the cluster.

```bash
kubectl create -f deploy/crds
```

There should be three CRDs present for ArgoCD on the cluster.

```bash
kubectl get crd
```

```bash
NAME                       CREATED AT
applications.argoproj.io   2019-11-09T02:35:47Z
appprojects.argoproj.io    2019-11-09T02:35:47Z
argocdexports.argoproj.io  2019-11-09T02:36:02Z
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

## Usage 

Once the operator is installed and running, new ArgoCD resources can be created. See the [usage][docs_usage] 
documentation to learn how to create new `ArgoCD` resources.

## Cleanup 

TODO

[docs_usage]:../usage/basics.md
