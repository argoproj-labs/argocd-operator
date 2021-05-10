# Kustomize Installation

The following steps can be used to install the operator on any Kubernetes environment with minimal overhead.

!!! info
    The steps in this process require the `cluster-admin` ClusterRole or equivalent.

## Cluster

This guide uses [minikube](https://minikube.sigs.k8s.io/) to deploy a Kubernetes cluster locally, follow the 
instructions for your platform to install. 

Run minikube with a dedicated profile. Adjust the system resources as needed for your platform. 

```bash
minikube start -p argocd --cpus=4 --disk-size=40gb --memory=8gb
```

## Using Kustomize to automate the manual installation process

The following section outlines the steps necessary to deploy the ArgoCD Operator using kustomize.

### Namespace

It is a good idea to create a new namespace for the operator.

```bash
kubectl create namespace argocd
```

Once the namespace is created, set up the local context to use the new namespace.

```bash
kubectl config set-context argocd --cluster argocd --namespace argocd --user argocd
kubectl config use-context argocd
```

All the resources will now be created in the new namespace.

kustomization.yaml provides a low overhead for folks getting started. Instead of having to apply each of the individual manifests, users can just run the below command.

```bash
kubectl apply -k deploy/
```

Alternatively, kustomization is also added at the root dir of operator referencing the deploy kustomization. Users can simply run the below command to install the operator and required resources.

```bash
kustomize build .
```

NOTE: Above command requires kustomize as a pre-requisite.