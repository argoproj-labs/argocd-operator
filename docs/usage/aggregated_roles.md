# Aggregated Roles

Using an aggregated cluster role enables users to easily add their own permissions without having to define a completely new cluster role from scratch. The current default cluster role for application controller is hard-coded with a specific set of permissions. This cluster role is operator managed and cannot be modified by the user. In this case administrative user can opt for custom cluster roles and define permissions by creating cluster roles from scratch or go for aggregated cluster role. 

A user can enable creation of aggregated ClusterRole by setting `argocd.spec.aggregatedClusterRoles` field to `true`.

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: basic
spec:
  aggregatedClusterRoles: true
```

When `aggregatedClusterRoles` is `true`, the default cluster role for the Argo CD application controller will be created having `aggregationRule` field. This is the base cluster role and a cluster role binding pointing to it will be created and managed by operator.

Example: Configure permissions using aggregated cluster role model for application controller:

1. Base cluster role

```yaml
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: argocd-argocd-argocd-application-controller
  annotations:
    rbac.authorization.kubernetes.io/autoupdate: 'true'
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        app.kubernetes.io/managed-by: argocd
        argocd/aggregate-to-controller: 'true'
rules: []
```

Initially there are no permissions defined in base cluster role, here it has `aggregationRule`, that means if there are other cluster roles with these two labels, then base cluster role can inherit permission from those.

2. Operator creates one cluster role to configure view permissions to base cluster role.

```yaml
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: argocd-argocd-argocd-application-controller-view
  labels:
    app.kubernetes.io/managed-by: argocd
    argocd/aggregate-to-controller: 'true'
rules:
  - verbs:
      - get
      - list
      - watch
    apiGroups:
      - '*'
    resources:
      - '*'
  - verbs:
      - get
      - list
    nonResourceURLs:
      - '*'
```
It has predefined view permissions.

3. Operator creates one cluster role to configure admin permissions to base cluster role.

```yaml
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: argocd-argocd-argocd-application-controller-admin
  labels:
    app.kubernetes.io/managed-by: argocd
    argocd/aggregate-to-controller: 'true'
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        app.kubernetes.io/managed-by: argocd
        argocd/aggregate-to-admin: 'true'
rules: []
```

Here there are no predefined rules and user can configure self-defined permissions by creating a self-defined cluster role. 

4. Now user need to create a new self-defined cluster role, but it must have matching labels given in `aggregationRule` of cluster role created for admin permissions i.e. `argocd-argocd-argocd-application-controller-admin`

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata: 
  name: my-cluster-role
  labels:
    app.kubernetes.io/managed-by: argocd
    argocd/aggregate-to-admin: 'true'
rules:
  - verbs:
      - '*'
    apiGroups:
      - ''
    resources:
      - namespaces
      - persistentvolumeclaims
      - persistentvolumes
      - configmaps
  - verbs:
      - '*'
    apiGroups:
      - machine.openshift.io
    resources:
      - '*'
  - verbs:
      - '*'
    apiGroups:
      - machineconfiguration.openshift.io
    resources:
      - '*'
```

Let's summarize this example. The `argocd-argocd-argocd-application-controller` cluster role inherits permissions from two cluster role which are `argocd-argocd-argocd-application-controller-view` for view permissions and `argocd-argocd-argocd-application-controller-admin` for admin permission. These three are operator managed. Now `argocd-argocd-argocd-application-controller-admin` inherits permissions from `my-cluster-role` which is a user defined cluster role.  

For more details on aggregated cluster role, check the [documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#aggregated-clusterroles).
