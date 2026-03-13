# Respect RBAC for controller

See the [upstream documentation](https://argo-cd.readthedocs.io/en/stable/operator-manual/declarative-setup/#auto-respect-rbac-for-controller) for more information.

This feature can be enabled by setting `respectRBAC` field in ArgoCD resource. To configure value in `argocd-cm` ConfigMap via ArgoCD resource, users need to configure `argocd.spec.controller.respectRBAC` field. Possible values for this field are `strict`, `normal` or empty (default).


```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  controller:
    respectRBAC: strict
```


