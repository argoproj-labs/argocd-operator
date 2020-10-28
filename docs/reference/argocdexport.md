# ArgoCDExport

The `ArgoCDExport` resource is a Kubernetes Custom Resource (CRD) that describes the desired state for the export of a given 
Argo CD cluster and enables disaster recovery for the components that make up Argo CD.

When the Argo CD Operator sees a new ArgoCDExport resource, the operator manages the built-in Argo CD export process.

The ArgoCDExport Custom Resource consists of the following properties.

Name | Default | Description
--- | --- | ---
[**Argocd**](#argocd) | [Empty] | The name of an ArgoCD instance to export.
[**Image**](#image) | `quay.io/jmckind/argocd-operator-util` | The container image for the export Job.
[**Schedule**](#schedule) | [Empty] | Export schedule in Cron format, see https://en.wikipedia.org/wiki/Cron.
[**Storage**](#storage-options) | [Object] | The storage configuration options.
[**Version**](#version) | v0.0.15 (SHA) | The tag to use with the container image for the export Job.

## Argocd

The name of an ArgoCD instance to export.

### Argocd Example

The following example sets the name of an ArgoCD resource to export.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCDExport
metadata:
  name: example-argocdexport
  labels:
    example: argocd
spec:
  argocd: example-argocd
```

## Image

The container image for the export Job.

### Image Example

The following example sets the default value using the `Image` property on the `ArgoCDExport` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCDExport
metadata:
  name: example-argocdexport
  labels:
    example: image
spec:
  image: quay.io/jmckind/argocd-operator-util
```

## Schedule

The export schedule in Cron format, see https://en.wikipedia.org/wiki/Cron.

### Schedule Example

The following example sets a recurring export schedule that runs daily at midnight. 

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCDExport
metadata:
  name: example-argocdexport
  labels:
    example: schedule
spec:
  schedule: "0 0 * * *"
```

## Storage Options

The following properties are available for configuring the storage for the export data.

Name | Default | Description
--- | --- | ---
Backend | `local` | The storage backend to use, must be "local", "aws", "azure" or "gcp".
PVC | [Object] | The [PersistentVolumeClaimSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#persistentvolumeclaimspec-v1-core) specifying the desired characteristics for a PersistentVolumeClaim.
SecretName | [Export Name] | The name of a Secret with encryption key, credentials, etc.

### Storage Example

The following example sets the default values using the `Storage` property on the `ArgoCDExport` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCDExport
metadata:
  name: example-argocdexport
  labels:
    example: storage
spec:
  storage:
    backend: local
    pvc: {}
    secretName: example-argocdexport
```

## Version

The tag to use with the container image for all Argo CD components.

### Version Example

The following example sets the default value using the `Version` property on the `ArgoCDExport` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCDExport
metadata:
  name: example-argocdexport
  labels:
    example: version
spec:
  version: v0.0.15
```
