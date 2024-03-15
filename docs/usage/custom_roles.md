# Custom Roles

The operator supports specifying alternate roles to the ones that are created by default. This
enables administrative users to tailor the permissions used by the operator deployed Argo CD instance as needed.

## Namespace Scoped Roles

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

These cluster roles can be customized via the `CONTROLLER_CLUSTER_SCOPE_ROLE` and `SERVER_CLUSTER_SCOPE_ROLE` environment variables. If these variables are set the operator will
use the specified ClusterRole instead of creating a new ClusterRole with the default permissions.

Example: Custom cluster scoped role environment variables in operator Subscription:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: argocd-operator
  namespace: argocd
spec:
  config:
    env:
    - name: CONTROLLER_CLUSTER_SCOPE_ROLE
      value: custom-cluster-scope-controller-role
    - name: SERVER_CLUSTER_SCOPE_ROLE
      value: custom-cluster-scope-server-role
```

Example: Custom cluster scoped role environment variables in operator Deployment:

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
          - name: CONTROLLER_CLUSTER_SCOPE_ROLE
            value: custom-cluster-scope-controller-role
          - name: SERVER_CLUSTER_SCOPE_ROLE
            value: custom-cluster-scope-server-role
```
