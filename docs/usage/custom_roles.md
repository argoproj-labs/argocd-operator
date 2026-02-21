# Custom Roles

As an administrative user, when you give Argo CD access to a namespace by using the `argocd.argoproj.io/managed-by` label, it assumes namespace-admin privileges. These privileges are an issue for administrators who provide namespaces to non-administrators, such as development teams, because the privileges enable non-administrators to modify objects such as network policies. With this update, administrators can configure a common cluster role for all the managed namespaces. In role bindings for the Argo CD application controller, the Operator refers to the CONTROLLER_CLUSTER_ROLE environment variable. In role bindings for the Argo CD server, the Operator refers to the SERVER_CLUSTER_ROLE environment variable. If these environment variables contain custom roles, the Operator doesn't create the default admin role. Instead, it uses the existing custom role for all managed namespaces.

Example: Custom role environment variables in operator Subscription:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: argocd-operator
  namespace: argocd
spec:
  config:
    env:
    - name: CONTROLLER_CLUSTER_ROLE
      value: custom-controller-role
    - name: SERVER_CLUSTER_ROLE
      value: custom-server-role
```

Example: Custom role environment variables in operator Deployment:

```yaml
kind: Deployment
apiVersion: apps/v1
metadata:
  name: argocd-operator-controller-manager
  namespace: argocd
spec:
  replicas: 1
  template:
    spec:
      containers:
          env:
          - name: CONTROLLER_CLUSTER_ROLE
            value: custom-controller-role
          - name: SERVER_CLUSTER_ROLE
            value: custom-server-role
```

## Cluster Scoped Roles

When the administrative user deploys Argo CD as a cluster scoped instance, the operator creates additional ClusterRoles and ClusterRoleBindings for the
application-controller and server components. These provide the additional permissions that Argo CD requires to operate at the cluster level.

Specifying alternate ClusterRoles enables the administrative user to add or remove permissions
as needed and have them applied across all cluster scoped instances. For example, features such as the [Auto Respect RBAC For Controller](https://argo-cd.readthedocs.io/en/stable/operator-manual/declarative-setup/#auto-respect-rbac-for-controller) enables specifying more granular permissions for the application-controller service account.

These customized ClusterRoles need to be created and referred in ClusterRoleBinding by admin. A user can disable creation of default ClusterRoles by setting `ArgoCD.Spec.DefaultClusterScopedRoleDisabled` field to `true`.

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: basic
spec:
  defaultClusterScopedRoleDisabled: true
```

When `defaultClusterScopedRoleDisabled` is `true`, the default ClusterRole/ClusterRoleBindings for the Argo CD instance will not be created, and the administrative user is free to create and customize these independent of the operator. The field can later be set to `false`, to recreate these resources, if needed.
