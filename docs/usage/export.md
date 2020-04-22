# Export

See the [ArgoCDExport Reference][argocdexport_reference] for the full list of properties and defaults to configure the export process for an Argo CD cluster.

## Create

The following example shows the most minimal valid manifest to export (backup) an Argo CD cluster that was provisioned using the Argo CD Operator. 

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCDExport
metadata:
  name: example-argocdexport
  labels:
    example: basic
spec:
  argocd: example-argocd
```

Create a new `ArgoCDExport` resource in the `argocd` namespace using the provided basic example.

```bash
kubectl create -n argocd -f examples/argocdexport-basic.yaml
```

Once the export resource has been created, the operator will provision a Kubernetes Job to run the built-in Argo CD export utility.

## Data

The Argo CD export data consists of a series of Kubernetes manifests in YAML format stored in a single file. This YAML file is `AES-256` encrypted before being saved to the storage backend of choice. 

## Storage Backend

The operator supports several storage mechanisms for the exported data.

### Local

By default the operator will use a `local` storage backend for the export process. The operator will provision a PersistentVolumeClaim to store the export locally in the cluster on a PersistentVolume.

### AWS

The operator can use an AWS S3 bucket to store the export data.

[argocdexport_reference]:../reference/argocdexport.md
