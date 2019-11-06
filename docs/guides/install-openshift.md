# Installing Argo CD Operator on OpenShift

This guide will walk through installing the Argo CD Operator on OpenShift 4.

## Manual Install

As a cluster-admin, install the ServiceAccounts, Roles and RoleBindings to set up RBAC for the operator.

```bash
oc create -f deploy/service_account.yaml
oc create -f deploy/role.yaml
oc create -f deploy/role_binding.yaml
```

### Cluster Admin

By default Argo CD prefers to run with the cluster-admin role. Give cluster-admin access to the Argo CD Application Controller.

```bash
export ARGO_NS=argocd
oc adm policy add-cluster-role-to-user cluster-admin system:serviceaccount:${ARGO_NS}:argocd-application-controller
```

Argo CD uses several custom Kubernetes resources (CRDs) to manage workloads. As a cluster-admin, 
add the CRDs for Argo CD to the cluster. Be sure to update `ARGO_NS` to use the actual namespace where you have the operator installed.

```bash
oc create -f deploy/argo-cd
```

The ArgoCD Operator manages a CRD as well. As a cluster-admin, add it to the cluster.

```bash
oc create -f deploy/crds/argoproj_v1alpha1_argocd_crd.yaml
```

Deploy the operator.

```bash
oc create -f deploy/operator.yaml
```

Once the operator is deployed, create a new ArgoCD custom resource.

```bash
oc create -f examples/argocd-minimal.yaml
```
