# Deploy resources to a different namespace

To grant Argo CD the permissions to manage resources in multiple namespaces, we need to configure the namespace with a label `argocd.argoproj.io/managed-by` and the value being the namespace of the managing Argo CD instance.

For example, If Argo CD instance deployed in the namespace `foo` wants to manage resources in namespace `bar`. Update the namespace `bar` as shown below.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: bar
  labels:
    argocd.argoproj.io/managed-by: foo # namespace of managing Argo CD instance
```

!!! note
    The above described method assumes that the user has admin privileges on their cluster, which would allow them to apply labels to namespaces. 


Alternatively, users can achieve the same behavior by leveraging the `.spec.syncPolicy` field of an application. SyncPolicy allows users to have a namespace created  with certain labels pre-configured at the time of application sync. Consider the following example Application:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
spec:
  syncPolicy:
    managedNamespaceMetadata:
      labels:
        argocd.argoproj.io/managed-by: foo # namespace of managing Argo CD instance 
    syncOptions:
    - CreateNamespace=true
  destination:
    server: https://kubernetes.default.svc
    namespace: bar 
```

The above described application will create a new namespace `bar` carrying a label `argocd.arogproj.io/managed-by: foo` at the time of application sync, and then deploy application resources in it, without requiring the user to be able to label namespace `bar` manually.

Users creating applications using the Argo CD UI instead of CLI must check the "auto-create namespace" box, and then switch to the yaml editor to add the label into `.spec.syncPolicy.managedNamespaceMetadata.labels` as described above.

A few points to keep in mind:

- This method requires that the user create the namespace at app sync time using `createNamespace=true` or checking the `auto-create namespace` box in the UI, and not include their own namespace manifest in their git repository. 
- A destination namespace must be set in `.spec.destination.namespace` 
- Users should have admin privileges and/or access to a cluster scoped Argo CD instance  

See [Namespace Metadata](https://argo-cd.readthedocs.io/en/stable/user-guide/sync-options/#namespace-metadata) in Argo CD docs for more information.

!!! note
    There is a possibility that sync might fail at first try when using the above method. In such cases a follow up sync should be successful.
