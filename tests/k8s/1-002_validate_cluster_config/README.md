This test will fail if ran against namespaced mode.
This test needs to run in cluster scope.
Add this to subscription, and run test.
```
spec:
  channel: alpha
  config:
    env:
    - name: ARGOCD_CLUSTER_CONFIG_NAMESPACES
      value: argocd-e2e-cluster-config
```