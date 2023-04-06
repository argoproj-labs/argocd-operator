# Deploy resources to a different namespace

To grant Argo CD the permissions to manage resources in multiple namespaces, we need to configure the namespace with a label `argocd.argoproj.io/managed-by` and the value being the namespace of the managing Argo CD instance.

For example, If Argo CD instance deployed in the namespace `foo` wants to manage resources in namespace `bar`. Update the namespace `bar` as shown below.

```yml
apiVersion: v1
kind: Namespace
metadata:
  name: bar
  labels:
    argocd.argoproj.io/managed-by: foo // namespace of the Argo CD instance
```
