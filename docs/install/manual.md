# Manual Installation using kustomize

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
manifests. Note that these steps generates the manifests using kustomize.

### Namespace

By default, the operator is installed into the `argocd-operator-system` namespace. To modify this, update the
value of the `namespace` specified in the `config/default/kustomization.yaml` file. 

### Deploy Operator

Deploy the operator. This will create all the necessary resources, including the namespace.

```bash
make deploy
```

If you want to use your own custom operator container image, you can specify the image name using the `IMG` variable.

```bash
make deploy IMG=quay.io/my-org/argocd-operator:latest
```

The operator pod should start and enter a `Running` state after a few seconds.

```bash
kubectl get pods -n argocd-operator-system
```

```bash
NAME                                                  READY   STATUS    RESTARTS   AGE
argocd-operator-controller-manager-6c449c6998-ts95w   2/2     Running   0          33s
```

## Usage 

Once the operator is installed and running, new ArgoCD resources can be created. See the [usage][docs_usage] 
documentation to learn how to create new `ArgoCD` resources.

## Cleanup 

To remove the operator from the cluster, run the following comand. This will remove all resources that were created,
including the namespace.
```bash
make undeploy
```



[docs_usage]:../usage/basics.md
