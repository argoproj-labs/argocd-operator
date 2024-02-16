# ApplicationSets in Any Namespace

**Current feature state**: Beta

!!! note
    This feature is considered beta feature in upstream Argo CD as of now. Some of the implementation details may change over the course of time until it is promoted to a stable status.

Argo CD supports managing `ApplicationSet` resources in namespaces other than the control plane's namespace. Argo CD administrators can define a certain set of namespaces where `ApplicationSet` resources may be created, updated and reconciled in. 

In order to manage `ApplicationSet` outside the Argo CD's control plane namespace, two prerequisites must be satisfied:

1. The Argo CD should be cluster-scoped
2. The enabled namespace should be entirely covered by the [Apps in Any Namespace](./apps-in-any-namespace.md)

## Enable ApplicationSets in a namespace

To enable this feature in a namespace, add the namespace name under `.spec.applicationSet.sourceNamespaces` field in ArgoCD CR.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  applicationSet:
    sourceNamespaces:
      - some-namespace
```

As of now, wildcards are not supported in `.spec.applicationSet.sourceNamespaces`. 

!!! important 
    Ensure that [Apps in Any Namespace](./apps-in-any-namespace.md) is enabled on target namespace i.e the target namespace name is covered under `.spec.sourceNamespaces` in ArgoCD CR.

The operator creates/modifies below RBAC resources when ApplicationSets in Any Namespace is enabled

|Name|Kind|Purpose|
|:-|:-|:-|
|`<argoCDName-argoCDNamespace>-argocd-applicationset-controller`|ClusteRole & ClusterRoleBinding|for `applicationset-controller` to watch & list `ApplicationSets` at cluster-level|
|`<argoCDName-argoCDNamespace>-applicationset`|Role & RoleBinding|in target namespace for `applicationset-controller` to manage `ApplicationSet` resources|
|`<argoCDName-targetNamespace>`|Role & RoleBinding|in target namespace for `argocd-server` to manipulate `ApplicationSet` resources via UI, API & CLI|

Additionally, it adds `argocd.argoproj.io/applicationset-managed-by-cluster-argocd` label to the target namespace.

Note that generated `Application` can create resources in any namespace. However, the `Application` itself will be in same namespace as `ApplicationSet`.

## Allow SCM Providers

By default, whenever ApplicationSets in Any Namespace is enabled, operator disables SCM & PR generators of security reasons. Read upstream [documentation](https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/Appset-Any-Namespace/#scm-providers-secrets-consideration) for more details. 

To use SCM & PR generators, Argo CD administrators need to explicitly define allowed SCM providers whitelist using `.spec.applicationSet.scmProviders` field in ArgoCD CR. 

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  applicationSet:
    sourceNamespaces:
      - some-namespace
    scmProviders:
      - https://git.mydomain.com/
      - https://gitlab.mydomain.com/
```

This will configure `applicationset-controller` to allow SCM & PR generators for whitelisted URLs. If any other url is used, it will be rejected by the `applicationset-controller`.

!!! important
    Please read upstream [documentation](https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/Appset-Any-Namespace/#scm-providers-secrets-consideration) carefully. Misconfiguration could lead to potential security issues.

### Things to consider

- No namespace can be managed by multiple argo-cd instances (cluster scoped or namespace scoped) i.e, only one of either `managed-by` or `applicationset-managed-by-cluster-argocd` labels can be applied to a given namespace. We will be prioritizing `managed-by` label in case of a conflict as this feature is currently in beta, so the new roles/rolebindings will not be created if namespace is already labelled with `managed-by` label, and they will be deleted if a namespace is first added to the `.spec.applicationSet.sourceNamespacs` list and is later also labelled with `managed-by` label.



