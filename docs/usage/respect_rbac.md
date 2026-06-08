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

## Cluster Scoped Instances

For cluster-scoped Argo CD instances, it is recommended to disable the default cluster roles to retain full control over the Kubernetes permissions granted to the application-controller. This can be achieved by setting `ArgoCD.Spec.DefaultClusterScopedRoleDisabled` field to `true`. Refer to the [Custom Roles documentation](custom_roles.md#cluster-scoped-roles) for further details.


> Note: When respectRBAC is enabled on a cluster-scoped Argo CD instance, the application-controller and server service accounts still require some cluster-wide permissions on the following argoproj.io CRDs: Application and ApplicationSet. These permissions are necessary because the controllers manage watches for these resources independently of the watches Argo CD maintains for deployed resources.

