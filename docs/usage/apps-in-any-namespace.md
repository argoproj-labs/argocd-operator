# Applications in any namespace

**Current feature state**: Beta

Argo CD supports managing Application resources in namespaces other than the control plane's namespace (which is usally argocd), but this feature has to be explicitly enabled and configured appropriately.

Argo CD administrators can define a certain set of namespaces where Application resources may be created, updated and reconciled in. However, applications in these additional namespaces will only be allowed to use certain AppProjects, as configured by the Argo CD administrators. This allows ordinary Argo CD users (e.g. application teams) to use patterns like declarative management of Application resources, implementing app-of-apps and others without the risk of a privilege escalation through usage of other AppProjects that would exceed the permissions granted to the application teams.

!!! note
    This feature is considered beta feature in upstream Argo CD as of now. Some of the implementation details may change over the course of time until it is promoted to a stable status.

## Using application-namespaces

In order to enable this feature, specify the namespaces where Argo CD should manage applications in the ArgoCD YAML with `spec.sourceNamespaces`. 
This field supports both:
 - Glob-style wildcards (e.g., `team-*`, `team-frontend`, `app-??`)
 - Regular expressions (wrapped in forward slashes, e.g., `/^team-(frontend|backend)$/`, `/^team-.*$/`)

The operator resolves these patterns to actual namespaces at reconcile time and passes the expanded, concrete list to the Application controller.

!!! note
    Regular expression patterns must be wrapped in forward slashes (`/pattern/`) to be treated as regex. Patterns without slashes are treated as glob patterns. For example:
    - `team-*` - glob pattern (matches team-1, team-2, etc.)
    - `/^team-[0-9]+$/` - regex pattern (matches team-1, team-2, but not team-frontend)

## Enable application creation in a specific namespace
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  sourceNamespaces:
    - some-namespace
```
In this example:

- Permissions are granted only to the specific namespace `some-namespace`.

## Enable application creation in namespaces matching a glob pattern

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd-wildcard-pattern
spec:
  sourceNamespaces:
    - app-team-*
```
In this example:

- Permissions are granted to namespaces matching the pattern `app-team-*`, such as `app-team-1`, `app-team-2`, etc.

## Enable application creation in namespaces matching regular expressions

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd-regex
spec:
  sourceNamespaces:
    - /^app-team-(frontend|backend)$/   # only frontend or backend
    - /^app-team-[0-9]+$/               # numeric suffix (app-team-1, app-team-2)
```

In these examples, permissions are granted only to namespaces that match the provided regex patterns.

## Enable application creation in all namespaces

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd-all-namespaces
spec:
  sourceNamespaces:
    - '*'
```
In this example:

- Permissions are granted for all namespaces on the Argo CD cluster using the `*` wildcard.

For additional details on allowing namespaces in an AppProject, check the [documentation](https://argo-cd.readthedocs.io/en/stable/operator-manual/app-any-namespace/#allowing-additional-namespaces-in-an-appproject). This feature is also essential to enable apps-in-any-namespace.

When a namespace is specified under `sourceNamespaces`, operator adds `argocd.argoproj.io/managed-by-cluster-argocd` label to the specified namespace. For example, the namespace would look like below:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  labels:
    argocd.argoproj.io/managed-by-cluster-argocd: example-argocd
    kubernetes.io/metadata.name: some-namespace
  name: some-namespace
```

**Things to consider:**

* No namespace can be managed by multiple argo-cd instances (cluster scoped or namespace scoped) i.e, only one of either `managed-by` or `managed-by-cluster-argocd` labels can be applied to a given namespace. We will be prioritizing `managed-by` label in case of a conflict as this feature is currently in beta, so the new roles/rolebindings will not be created if namespace is already labelled with `managed-by` label, and they will be deleted if a namespace is first added to the `sourceNamespacs` list and is later also labelled with `managed-by` label.

* Users will not be create/manage apps and create app resources in the same namespace that is added to `sourceNamespaces` (as they both require their own labels) out of the box. As a workaround users will have to create custom roles to be able to create app resources in the namespace added to `sourceNamespaces`.


