# Custom Roles

Admins can configure a common cluster role for all the Argo CD instances running on a cluster by specifying them in environment variables. The operator looks for the environment variable `CONTROLLER_CLUSTER_ROLE` for argocd application controller and `SERVER_CLUSTER_ROLE` for argocd server and refers them in role bindings. In the presence of custom roles, the operator will not create the default role and uses the existing custom role for every Argo CD instance. We can inject these environment variables either in the Subscription OLM object or directly into the operator deployment.

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
