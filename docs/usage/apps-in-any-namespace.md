# Applications in any namespace

**Current feature state**: Beta

Argo CD supports managing Application resources in namespaces other than the control plane's namespace (which is usally argocd), but this feature has to be explicitly enabled and configured appropriately.

Argo CD administrators can define a certain set of namespaces where Application resources may be created, updated and reconciled in. However, applications in these additional namespaces will only be allowed to use certain AppProjects, as configured by the Argo CD administrators. This allows ordinary Argo CD users (e.g. application teams) to use patterns like declarative management of Application resources, implementing app-of-apps and others without the risk of a privilege escalation through usage of other AppProjects that would exceed the permissions granted to the application teams.

!!! note
    This feature is considered beta feature in upstream Argo CD as of now. Some of the implementation details may change over the course of time until it is promoted to a stable status.

## Using application-namespaces
In order to enable this feature, the Argo CD administrator must reconfigure the argocd-server and argocd-application-controller workloads to add the --application-namespaces parameter to the container's startup command.

The `--application-namespaces` parameter takes a comma-separated list of namespaces where Applications are to be allowed in. Each entry of the list supports shell-style wildcards such as `*`, so for example the entry `app-team-*` would match `app-team-one` and `app-team-two`. To enable all namespaces on the cluster where Argo CD is running on, you can just `*`, i.e. `--application-namespaces=*`.

You can set the namespaces for argocd-server and argocd-controller in the ArgoCD yaml by setting `spec.sourceNamespaces`.

## Example

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  sourceNamespaces:
    - some-namespace
```

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


