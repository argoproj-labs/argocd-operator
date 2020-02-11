# ArgoCDExport

The `ArgoCDExport` resource is a Kubernetes Custom Resource (CRD) that describes the desired state for the export of a given 
Argo CD deployment and enables disaster recovery for the components that make up Argo CD.

When the Argo CD Operator sees a new ArgoCDExport resource, the operator manages the built-in Argo CD export process.
