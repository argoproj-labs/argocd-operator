# Export

Argo CD can export the details of managed application periodically, facilitating operations tasks such as backups or migrations.
The data consists of a series of Kubernetes manifests representing the various cluster resources in YAML format stored in a single file.
This exported YAML file is then `AES` encrypted before being saved to the storage backend of choice.

See the Argo CD [Disaster Recovery][argocd_dr] documentation for more information on the Argo CD recovery procedure.

See the [ArgoCDExport Reference][argocdexport_reference] for the full list of properties to configure the export process.

## Requirements

The following sections assume that an existing Argo CD cluster named `example-argocd` has been deployed by the operator using the existing basic `ArgoCD` example.

``` bash
kubectl apply -f examples/argocd-basic.yaml
```

If an `ArgoCDExport` resource is created that references an Argo CD cluster that does not exist, the operator will simply move on and wait until the Argo CD cluster does exist before taking any further action in the export process.
 
## ArgoCDExport

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

This would create a new `ArgoCDExport` resource with the name of `example-argocdexport`. The operator will provision a 
Kubernetes Job to run the built-in Argo CD export utility on the specified Argo CD cluster.

If the `Schedule` property was set using valid Cron syntax, the operator will provision a CronJob to run the export on 
a recurring schedule. Each time the CronJob executes, the export data will be overritten by the operator, only keeping 
the most recent version.

The data that is exported by the Job is owned by the `ArgoCDExport` resource, not the Argo CD cluster. So the cluster can 
come and go, starting up everytime by importing the same backup data, if desired.

See the `ArgoCD` [Import Reference][argocd_import] documentation for more information on importing the backup data when starting a new 
Argo CD cluster.

## Export Secrets

An export Secret is used by the operator to hold the backup encryption key, as well as credentials if using a cloud 
provider storage backend. The operator will create the Secret if it does not already exist, using the naming convention
`[EXPORT NAME]-export`. For example, if the `ArgoCDExport` resource is named `example-argocdexport` from above, the 
name of the generated secret would be `example-argocdexport-export`.

The `SecretName` property on the `ArgoCDExport` Storage Spec can be used to change the name of the Secret.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCDExport
metadata:
  name: example-argocdexport
  labels:
    example: secret-name
spec:
  argocd: example-argocd
  storage:
    secretName: my-backup-secret
```

The following property is common across all storage backends. See the sections below for additional properties that are
required for the different cloud provider backends. 

**backup.key**

The `backup.key` is the encryption key used by the operator when encrypting or decrypting the exported data. This key
will be generated automatically if not provided.

## Storage Backend

The exported data can be saved on a variety of backend storage locations. This can be persisted locally in the 
Kubernetes cluster or remotely using a cloud provider.

See the `ArgoCDExport` [Storage Reference][storage_reference] for information on controlling the underlying storage 
options.

### Local

By default, the operator will use a `local` storage backend for the export process. The operator will provision a 
PersistentVolumeClaim using the defaults below to store the export data locally in the cluster on a PersistentVolume.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCDExport
metadata:
  name: example-argocdexport
  labels:
    example: pvc
spec:
  argocd: example-argocd
  storage:
    backend: local
    pvc:
      accessModes:
        - ReadWriteOnce
      resources:
        requests:
          storage: 2Gi
      storageClassName: standard
```

#### Local Example

Create an `ArgoCDExport` resource in the `argocd` namespace using the basic example.

``` bash
kubectl apply -n argocd -f examples/argocdexport-basic.yaml
```

You can view the list of `ArgoCDExport` resources.

``` bash
kubectl get argocdexports
```
```
NAME                   AGE
example-argocdexport   15m
```

Creating the resource will result in the operator provisioning a Kubernetes Job to perform the export process. The Job 
should not take long to complete.

``` bash
kubectl get pods -l job-name=example-argocdexport
```
```
NAME                         READY   STATUS      RESTARTS   AGE
example-argocdexport-q92qm   0/1     Completed   0          1m
```

if the Job fails for some reason, view the logs of the Pod to help in troubleshooting.

``` bash
kubectl logs example-argocdexport-q92qm
```

Output similar to what is shown below indicates a successful export.

```
exporting argo-cd
creating argo-cd backup
encrypting argo-cd backup
argo-cd export complete
```

View the PersistentVolumeClaim created by the operator for the export data.

``` bash
kubectl get pvc -n argocd
```
```
NAME                   STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
example-argocdexport   Bound    pvc-6d15143d-184a-4e5a-a185-6b86924af8bd   2Gi        RWO            gp2            39s
```

There should also be a corresponding PersistentVolume if dynamic volume support is enabled on the Kubernetes cluster.

``` bash
kubectl get pv -n argocd
```
```
NAME                                       CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM                         STORAGECLASS   REASON   AGE
pvc-6d15143d-184a-4e5a-a185-6b86924af8bd   2Gi        RWO            Delete           Bound    argocd/example-argocdexport   gp2                     34s
```

### AWS

The operator can use an Amazon Web Services S3 bucket to store the export data.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCDExport
metadata:
  name: example-argocdexport
  labels:
    example: aws
spec:
  argocd: example-argocd
  storage:
    backend: aws
    secretName: aws-backup-secret
```

#### AWS Secrets

The storage `SecretName` property should reference an existing secret that contains the AWS credentials and bucket information.

``` yaml
apiVersion: v1
kind: Secret
metadata:
  name: aws-backup-secret
  labels:
    example: aws
type: Opaque
data:
  aws.bucket.name: ...
  aws.bucket.region: ...
  aws.access.key.id: ...
  aws.secret.access.key: ....
```

The following properties must exist on the Secret referenced in the `ArgoCDExport` resource when using `aws` as the storage backend.

**aws.bucket.name**

The name of the AWS S3 bucket. This should be the name of the bucket only, do not prefix the value `s3://`, as the operator will handle this automatically.

**aws.bucket.region**

The region of the AWS S3 bucket.

**aws.access.key.id**

The AWS IAM Access Key ID.

**aws.secret.access.key**

The AWS IAM Secret Access Key.

#### AWS Example

Once the required AWS credentials are set on the export Secret, create the `ArgoCDExport` resource in the `argocd` 
namespace using the included AWS example.

``` bash
kubectl apply -n argocd -f examples/argocdexport-aws.yaml
```

Creating the resource will result in the operator provisioning a Kubernetes Job to perform the export process. 

``` bash
kubectl get pods -l job-name=example-argocdexport
```

The Job should not take long to complete.

```
NAME                         READY   STATUS      RESTARTS   AGE
example-argocdexport-q92qm   0/1     Completed   0          1m
```

If the Job fails for some reason, view the logs of the Pod to help in troubleshooting.

``` bash
kubectl logs example-argocdexport-q92qm
```

Output similar to what is shown below indicates a successful export.

```
exporting argo-cd
creating argo-cd backup
encrypting argo-cd backup
pushing argo-cd backup to aws
make_bucket: example-argocdexport
upload: ../../backups/argocd-backup.yaml to s3://example-argocdexport/argocd-backup.yaml
argo-cd export complete
```

#### AWS IAM Configuration

TODO: Add the required Role and Service Account configuration needed through AWS.

### Azure

The operator can use a Micosoft Azure Storage Container to store the export data as Blob.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCDExport
metadata:
  name: example-argocdexport
  labels:
    example: azure
spec:
  argocd: example-argocd
  storage:
    backend: azure
    secretName: azure-backup-secret
```

#### Azure Secrets

The storage `SecretName` property should reference an existing secret that contains the Azure credentials and bucket information.

``` yaml
apiVersion: v1
kind: Secret
metadata:
  name: azure-backup-secret
  labels:
    example: azure
type: Opaque
data:
  azure.container.name: ...
  azure.service.id: ...
  azure.service.cert: |
    ...
  azure.storage.account: ...
  azure.tenant.id: ...
```

The following properties must exist on the Secret referenced in the `ArgoCDExport` resource when using `azure` as the storage backend.

**azure.container.name**

The name of the Azure Storage Container. This should be the name of the container only. If the container does not 
already exist, the operator will attempt to create it.

**azure.service.id**

The ID for the Service Principal that will be used to access Azure Storage.

**azure.service.cert**

The combination of certificate and private key for authenticating the Service Principal that will be used to access Azure Storage.

**azure.storage.account**

The name of the Azure Storage Account that owns the Container.

**azure.tenant.id**

The ID for the Azure Tenant that owns the Service Principal.

#### Azure Example

Once the required Azure credentials are set on the export Secret, create the `ArgoCDExport` resource in the `argocd` 
namespace using the included AWS example.

``` bash
kubectl apply -n argocd -f examples/argocdexport-azure.yaml
```

Creating the resource will result in the operator provisioning a Kubernetes Job to perform the export process. 

``` bash
kubectl get pods -l job-name=example-argocdexport
```

The Job should not take long to complete.

```
NAME                         READY   STATUS      RESTARTS   AGE
example-argocdexport-q92qm   0/1     Completed   0          1m
```

If the Job fails for some reason, view the logs of the Pod to help in troubleshooting.

``` bash
kubectl logs example-argocdexport-q92qm
```

Output similar to what is shown below indicates a successful export.

```
exporting argo-cd
creating argo-cd backup
encrypting argo-cd backup
pushing argo-cd backup to azure
[
  {
    "cloudName": "...",
    "homeTenantId": "...",
    "id": "...",
    "isDefault": true,
    "managedByTenants": [],
    "name": "...",
    "state": "Enabled",
    "tenantId": "...",
    "user": {
      "name": "...",
      "type": "servicePrincipal"
    }
  }
]
{
  "created": false
}
Finished[#############################################################]  100.0000%
{
  "etag": "\"0x000000000000000\"",
  "lastModified": "2020-04-20T16:20:00+00:00"
}
argo-cd export complete
```

#### Azure AD Configuration

TODO: Add the required Role and Service Account configuration needed through Azure Active Directory.

### GCP

The operator can use a Google Cloud Storage bucket to store the export data.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCDExport
metadata:
  name: example-argocdexport
  labels:
    example: gcp
spec:
  argocd: example-argocd
  storage:
    backend: gcp
    secretName: gcp-backup-secret
```

#### GCP Secrets

The storage `SecretName` property should reference an existing secret that contains the GCP credentials and bucket information.

``` yaml
apiVersion: v1
kind: Secret
metadata:
  name: gcp-backup-secret
  labels:
    example: gcp
type: Opaque
data:
  gcp.bucket.name: ...
  gcp.project.id: ...
  gcp.key.file: |
    ...
```

The following properties must exist on the Secret referenced in the `ArgoCDExport` resource when using `gcp` as the storage backend.

**gcp.bucket.name**

The name of the GCP storage bucket. This should be the name of the bucket only, do not prefix the value `gs://`, as the operator will handle this automatically.

**gcp.project.id**

The the project ID to use for authenticating with GCP. This can be the text name or numeric ID for the GCP project.

**gcp.key.file**

The GCP key file that contains the service account authentication credentials. The key file can be JSON formatted (preferred) or p12 (legacy) format.

#### GCP Example

Once the required GCP credentials are set on the export Secret, create the `ArgoCDExport` resource in the `argocd` 
namespace using the included GCP example.

``` bash
kubectl apply -f examples/argocdexport-gcp.yaml
```

This will result in the operator creating a Job to perform the export process. 

``` bash
kubectl get pods -l job-name=example-argocdexport
```

The Job should not take long to complete.

```
NAME                         READY   STATUS      RESTARTS   AGE
example-argocdexport-q92qm   0/1     Completed   0          1m
```

If the Job fails for some reason, view the logs of the Pod to help in troubleshooting.

``` bash
kubectl logs example-argocdexport-q92qm
```

Output similar to what is shown below indicates a successful export.

```
exporting argo-cd
creating argo-cd backup
encrypting argo-cd backup
pushing argo-cd backup to gcp
Activated service account credentials for: [argocd-export@example-project.iam.gserviceaccount.com]
Creating gs://example-argocdexport/...
Copying file:///backups/argocd-backup.yaml [Content-Type=application/octet-stream]...
/ [1 files][  7.8 KiB/  7.8 KiB]
Operation completed over 1 objects/7.8 KiB.
argo-cd export complete
```

#### GCP IAM Configuration

TODO: Add the required Role and Service Account configuration needed through GCP.

## Import

See the `ArgoCD` [Import Reference][argocd_import] documentation for more information on importing the backup data when starting a new 
Argo CD cluster.

[argocdexport_reference]:../reference/argocdexport.md
[storage_reference]:../reference/argocdexport.md#storage-options
[argocd_dr]:https://argoproj.github.io/argo-cd/operator-manual/disaster_recovery/
[argocd_import]:../reference/argocd.md#import-options
