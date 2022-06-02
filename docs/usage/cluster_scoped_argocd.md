# Cluster Configuration

When the user wishes to install Argo CD with the purpose of managing cluster resources, Argo CD is granted permissions to manage specific cluster-scoped resources which include platform operators, optional OLM operators, user management, etc. 

Note: **Argo CD is not granted cluster-admin**.

## Argo CD Installation

To manage cluster-config, deploy an ArgoCD instance using the steps provided above. 

* As an admin, update the existing Subscription Object for Argo CD Operator and add `ARGOCD_CLUSTER_CONFIG_NAMESPACES` to the spec.

Edit the Subscription yaml to add ENV, **ARGOCD_CLUSTER_CONFIG_NAMESPACES** as defined below.

```
spec:
  config:
    env:
    - name: ARGOCD_CLUSTER_CONFIG_NAMESPACES
      value: <namespace where Argo CD instance is installed>, <another namespace where Argo CD instance is installed>
```

###  In-built permissions for cluster configuration

<table>
  <tr>
    <td>Resource Groups</td>
    <td>What does it configure for the user/administrator</td>
  </tr>
  <tr>
    <td>storage.k8s.io</td>
    <td>Storage.</td>
  </tr>
  <tr>
    <td>operators.coreos.com</td>
    <td>Optional operators managed by OLM</td>
  </tr>
  <tr>
    <td>user.openshift.io , rbac.authorization.k8s.io</td>
    <td>Groups, Users and their permissions.</td>
  </tr>
  <tr>
    <td>config.openshift.io</td>
    <td>Control plane Operators managed by CVO used to configure cluster-wide build configuration, registry configuration, scheduler policies, etc.</td>
  </tr>
  <tr>
    <td>console.openshift.io</td>
    <td>Console customization.</td>
  </tr>
</table>

### Additional Permissions

User can extend the permissions provided to Argo CD instance. You can reference the below example to grant Argo CD permission to manage secrets.

Example clusterrole to grant full access to secrets.

```
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  # "namespace" omitted since ClusterRoles are not namespaced
  name: secrets-cluster-role
rules:
- apiGroups: [""] #specifies core api groups
  resources: ["secrets"]
  verbs: ["*"]
```

Example clusterrolebinding to bind the above clusterrole to Argo CD controller service account.

```
apiVersion: rbac.authorization.k8s.io/v1
# This cluster role binding allows Service Account to read secrets in any namespace.
kind: ClusterRoleBinding
metadata:
  name: read-secrets-global
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: secrets-cluster-role # Name of cluster role to be referenced
subjects:
- kind: ServiceAccount
  name: <Argo CD application controller serviceaccount name>
  namespace: <Namespace where Argo CD is Installed>
```