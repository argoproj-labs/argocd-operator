# ApplicationSets in Any Namespace

**Current feature state**: Beta

!!! note
    This feature is considered beta feature in upstream Argo CD as of now. Some of the implementation details may change over the course of time until it is promoted to a stable status.

Argo CD supports managing `ApplicationSet` resources in non-control plane namespaces. Argo CD administrators can define a certain set of namespaces to create, update, and reconcile `ApplicationSet` resources.

To manage the `ApplicationSet` resources in non-control plane namespaces i.e outside the Argo CD's namespace, you must satisfy the following prerequisites:

1. The Argo CD instance should be cluster-scoped
2. [Apps in Any Namespace](./apps-in-any-namespace.md) should be enabled on target namespaces

## Enable ApplicationSets in a namespace

To enable this feature in a namespace, add the namespace name under `.spec.applicationSet.sourceNamespaces` in the ArgoCD CR.
This field supports both:
 - Glob-style wildcards (e.g., `team-*`, `team-frontend`, `app-??`)
 - Regular expressions (wrapped in forward slashes, e.g., `/^team-(frontend|backend)$/`, `/^team-.*$/`)
 
 The operator resolves these patterns to actual namespaces at reconcile time and passes the expanded, concrete list to the ApplicationSet controller.
 
 !!! note
     Regular expression patterns must be wrapped in forward slashes (`/pattern/`) to be treated as regex. Patterns without slashes are treated as glob patterns. For example:
     - `team-*` - glob pattern (matches team-1, team-2, etc.)
     - `/^team-[0-9]+$/` - regex pattern (matches team-1, team-2, but not team-frontend)

### Enable ApplicationSets in a specific namespace

For example, following configuration will allow `example` Argo CD instance to create & manage `ApplicationSet` resource in `foo` namespace. 
```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example
spec:
  applicationSet:
    sourceNamespaces:
      - foo
```

### Enable ApplicationSets in namespaces matching a pattern

You can use wildcard patterns or regular expressions to automatically provision ApplicationSet permissions in all namespaces that match the pattern:

**Using glob patterns:**
```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example
spec:
  applicationSet:
    sourceNamespaces:
      - team-*           # glob pattern
```

**Using regular expressions:**
```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example
spec:
  applicationSet:
    sourceNamespaces:
      - /^team-(frontend|backend)$/    # regex pattern (note the /.../ wrapper)
      - /^team-[0-9]+$/                # regex: matches team-1, team-2, etc. (numbers only)
```

In the glob pattern example, permissions are granted to namespaces matching `team-*`, such as `team-1`, `team-2`, `team-frontend`, etc. In the regex example, permissions are granted only to namespaces matching the specific regex patterns (e.g., `team-frontend` and `team-backend` for the first pattern, or numeric-only namespaces like `team-1`, `team-2` for the second pattern).

The Operator will automatically create the necessary RBAC permissions in all existing namespaces that match the pattern, and will continue to provision permissions for newly created namespaces that match the pattern. 

!!! warning 
    Exercise caution when using broad wildcard patterns such as `*` or `*-prod`. These patterns can match a large number of namespaces, including system namespaces or sensitive environments, potentially granting unintended access. Always use the most specific pattern that meets your requirements and regularly audit which namespaces match your patterns.    

!!! important 
    Ensure that [Apps in Any Namespace](./apps-in-any-namespace.md) is enabled on target namespace i.e the target namespace name is part of `.spec.sourceNamespaces` field in ArgoCD CR.
    
For more details about ApplicationSets in Any Namespace, see the upstream [Argo CD documentation](https://argo-cd.readthedocs.io/en/latest/operator-manual/applicationset/Appset-Any-Namespace/).

The Operator creates/modifies below RBAC resources when ApplicationSets in Any Namespace is enabled

|Name|Kind|Purpose|
|:-|:-|:-|
|`<argoCDName-argoCDNamespace>-argocd-applicationset-controller`|ClusteRole & ClusterRoleBinding|For ApplicationSet controller to watch and list `ApplicationSet` resources at cluster-level|
|`<argoCDName-argoCDNamespace>-applicationset`|Role & RoleBinding|For ApplicationSet controller to manage `ApplicationSet` resources in target namespace|
|`<argoCDName-targetNamespace>`|Role & RoleBinding|For Argo CD server to manage `ApplicationSet` resources in target namespace via UI, API or CLI|

Additionally, it adds `argocd.argoproj.io/applicationset-managed-by-cluster-argocd` label to the target namespace.

Note that generated `Application` can create resources in any namespace. However, the `Application` itself will be in same namespace as `ApplicationSet`.

## Allow SCM Providers

By default, whenever you enable the ApplicationSets in Any Namespace feature, the Operator disables Source Code Manager (SCM) Provider generator & Pull Request (PR) generator for security reasons. Read upstream [documentation](https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/Appset-Any-Namespace/#scm-providers-secrets-consideration) for more details. 

To use SCM Provider & PR generators, Argo CD administrators must explicitly define a list of allowed SCM providers using the `.spec.applicationSet.scmProviders` field in the ArgoCD CR. 

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example
spec:
  applicationSet:
    sourceNamespaces:
      - foo
    scmProviders:
      - https://git.mydomain.com/
      - https://gitlab.mydomain.com/
```

This will configure ApplicationSet controller to allow the defined URLs for SCM Provider & PR generators. If any other url is used, it will be rejected by the ApplicationSet controller.

!!! important
    Please read upstream [documentation](https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/Appset-Any-Namespace/#scm-providers-secrets-consideration) carefully. Misconfiguration could lead to potential security issues.

### Things to consider

Only one of either `managed-by` or `applicationset-managed-by-cluster-argocd` labels can be applied to a given namespace. We will be prioritizing `managed-by` label in case of a conflict as this feature is currently in beta, so the new roles/rolebindings will not be created if namespace is already labelled with `managed-by` label, and they will be deleted if a namespace is first added to the `.spec.applicationSet.sourceNamespaces` list and is later also labelled with `managed-by` label.



