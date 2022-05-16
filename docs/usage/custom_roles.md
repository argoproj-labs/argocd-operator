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


When an Argo CD instance is deleted, namespaces managed by that instance (via the `argocd.argoproj.io/managed-by` label ) will retain the label by default. Users can change this behavior by setting the environment variable `REMOVE_MANAGED_BY_LABEL_ON_ARGOCD_DELETION` to `true` in the Subscription.

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: argocd-operator
  namespace: argocd
spec:
  config:
    env:
    - name: REMOVE_MANAGED_BY_LABEL_ON_ARGOCD_DELETION
      value: "true"
```