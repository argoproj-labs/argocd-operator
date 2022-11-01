# Applications in any namespace

Argo CD supports managing Application resources in namespaces other than the control plane's namespace (which is usally argocd), but this feature has to be explicitly enabled and configured appropriately.

Argo CD administrators can define a certain set of namespaces where Application resources may be created, updated and reconciled in. However, applications in these additional namespaces will only be allowed to use certain AppProjects, as configured by the Argo CD administrators. This allows ordinary Argo CD users (e.g. application teams) to use patterns like declarative management of Application resources, implementing app-of-apps and others without the risk of a privilege escalation through usage of other AppProjects that would exceed the permissions granted to the application teams.

## Using application-namespaces
In order to enable this feature, the Argo CD administrator must reconfigure the argocd-server and argocd-application-controller workloads to add the --application-namespaces parameter to the container's startup command.

The `--application-namespaces` parameter takes a comma-separated list of namespaces where Applications are to be allowed in. Each entry of the list supports shell-style wildcards such as `*`, so for example the entry `app-team-*` would match `app-team-one` and `app-team-two`. To enable all namespaces on the cluster where Argo CD is running on, you can just `*`, i.e. `--application-namespaces=*`.

You can set the namespaces for argocd-server and argocd-controller in the ArgoCD yaml by setting `spec.server.sourceNamespaces` and `spec.controller.sourceNamespaces`.

## Example

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  server:
    sourceNamespaces:
    - some-namespace
  controller:
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


