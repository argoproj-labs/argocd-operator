# Configure Argo CD ConfigMap for features not supported by Argo CD CRD

## Motivation

As the Argo CD upstream project evolves, new features are continuously added and new Argo CD config map configurations are constantly introduced. It is very difficult to keep Argocd Operator up to date with Argo CD’s new configurations. Argocd Operator is lagging behind to support new Argo CD features in months or more.

But, oftentimes to support a new feature, it is as simple as reconciling a configmap entry.  However, the code to make this a first class configuration by making it as a field in Argocd CR requires many changes. It takes quite a bit of development effort to reconcile just one new configmap entry. Also, since we made a new field in Argo CD CR, User will have to read the Argocd Operator’s manual in order to figure out the name of the field even though the user may already know the configmap entry key.

## Using ExtraConfig

Users can add dynamic entries to Argo CD configmap using `ExtraConfig` in Argocd CR. It is completely optional and has no default value.

When `ExtraConfig` is set, the entries specified are reconciled to the live Argo CD configmap. Users can specify arbitrary configmap entries with this `ExtraConfig`. This allows users to specify a new configuration even though the configuration is not supported by Argo CD CRD.

!!! note
    `ExtraConfig` takes precedence over Argo CD CRD.

## Example

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  extraConfig: 
    "ping": "pong" # The same entry is reflected in Argo CD Configmap.
  server:
    ingress:
      enabled: true
```
