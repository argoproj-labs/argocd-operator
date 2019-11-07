# Basic Install

The following steps can be used to install the operator on Kubernetes.

### RBAC

Set up RBAC for the operator

```bash
kubectl create -f deploy/service_account.yaml
kubectl create -f deploy/role.yaml
kubectl create -f deploy/role_binding.yaml
```

### CRDs

Add the Argo CD server CRDs to the cluster.

```bash
kubectl create -f deploy/argo-cd
```

Add the ArgoCD Operator CRD to the cluster

```bash
kubectl create -f deploy/crds/argoproj_v1alpha1_argocd_crd.yaml
```

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
