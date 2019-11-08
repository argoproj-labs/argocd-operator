# Basic Install

The following basic steps can be used to install the operator on any Kubernetes environment.

Several of the steps in this process require the `cluster-admin` ClusterRole or equivalent.

### Namespace

It is a good idea to create a new namespace for the operator.

```bash
kubectl create namespace argocd
```

Once the namespace is created, set up the local context to use the new namespace.

### RBAC

Set up RBAC for the operator

```bash
kubectl create -f deploy/service_account.yaml
kubectl create -f deploy/role.yaml
kubectl create -f deploy/role_binding.yaml
```

### CRDs

Add the Argo CD CRDs to the cluster.

```bash
kubectl create -f deploy/argo-cd
```

Add the ArgoCD Operator CRD to the cluster

```bash
kubectl create -f deploy/crds/argoproj_v1alpha1_argocd_crd.yaml
```

If you are deploying to OpenShift, there are [additional steps](./install-openshift.md#cluster-admin) that are needed.

### Deploy

Deploy the operator

```bash
kubectl create -f deploy/operator.yaml
```

### ArgoCD

Once the operator is deployed, create a new ArgoCD custom resource.

```bash
kubectl create -f examples/argocd-minimal.yaml
```
