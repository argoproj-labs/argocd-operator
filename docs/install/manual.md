# Manual Installation using kustomize

The following steps can be used to manually install the operator on any Kubernetes environment with minimal overhead.

!!! info
    Several of the steps in this process require the `cluster-admin` ClusterRole or equivalent.

## Cluster

This guide uses [minikube](https://minikube.sigs.k8s.io/) to deploy a Kubernetes cluster locally, follow the 
instructions for your platform to install. 

Run minikube with a dedicated profile. Adjust the system resources as needed for your platform. 

```bash
minikube start -p argocd --cpus=4 --disk-size=40gb --memory=8gb
```

## Manual Install

The following section outlines the steps necessary to deploy the ArgoCD Operator manually using standard Kubernetes 
manifests. Note that these steps generates the manifests using kustomize.

!!! info
    Make sure you download the source code from release section: https://github.com/argoproj-labs/argocd-operator/releases. Compiling from the source code cloned off main repo may not provide the most stable result.

### Namespace

By default, the operator is installed into the `argocd-operator-system` namespace. To modify this, update the
value of the `namespace` specified in the `config/default/kustomization.yaml` file. 

### Conversion Webhook Support

ArgoCD `v1alpha1` CRD has been **deprecated** starting from **argocd-operator v0.8.0**. To facilitate automatic migration of existing v1alpha1 ArgoCD CRs to v1beta1, conversion webhook support has been introduced.

By default, the conversion webhook is disabled for the manual(non-OLM) installation of the operator. Users can modify the configurations to enable conversion webhook support using the instructions provided below.

!!! warning
    Enabling the webhook is optional. However, without conversion webhook support, users are responsible for migrating any existing ArgoCD v1alpha1 CRs to v1beta1.

##### Enable Webhook Support

To enable the operator to utilize the `cert-manager` for automated webhook certificate management, ensure that it is installed in the cluster. Use [this](https://cert-manager.io/docs/installation/) guide to install `cert-manager` if not present on the cluster.

Add cert-manager annotation to CRD in `config/crd/patches/cainjection_in_argocds.yaml` file.
```yaml
metadata:
  name: argocds.argoproj.io
  annotations: 
    cert-manager.io/inject-ca-from: $(CERTIFICATE_NAMESPACE)/$(CERTIFICATE_NAME)
```

Enable `../certmanager` directory under the `bases` section in `config/default/kustomization.yaml` file.
```yaml
bases:
.....
- ../webhook
# [CERTMANAGER] To enable cert-manager, uncomment all sections with 'CERTMANAGER'. 'WEBHOOK' components are required.
- ../certmanager
```

Enable all the `vars` under the `[CERTMANAGER]` section in `config/default/kustomization.yaml` file.
```yaml
vars:
# [CERTMANAGER] To enable cert-manager, uncomment all sections with 'CERTMANAGER' prefix.
- name: CERTIFICATE_NAMESPACE # namespace of the certificate CR
  objref:
    kind: Certificate
    group: cert-manager.io
    version: v1
    name: serving-cert # this name should match the one in certificate.yaml
  fieldref:
    fieldpath: metadata.namespace
- name: CERTIFICATE_NAME
  objref:
    kind: Certificate
    group: cert-manager.io
    version: v1
    name: serving-cert # this name should match the one in certificate.yaml
- name: SERVICE_NAMESPACE # namespace of the service
  objref:
    kind: Service
    version: v1
    name: webhook-service
  fieldref:
    fieldpath: metadata.namespace
- name: SERVICE_NAME
  objref:
    kind: Service
    version: v1
    name: webhook-service
```

Additionally, set the `ENABLE_CONVERSION_WEBHOOK` environment variable in `config/default/manager_webhook_patch.yaml` file to enable the conversion webhook.
```yaml
      - name: manager
        env:
        - name: ENABLE_CONVERSION_WEBHOOK
          value: "true"
```

### Deploy Operator

Deploy the operator. This will create all the necessary resources, including the namespace. For running the make command you need to install go-lang package on your system.

```bash
make deploy
```

If you want to use your own custom operator container image, you can specify the image name using the `IMG` variable.

```bash
make deploy IMG=quay.io/my-org/argocd-operator:latest
```

The operator pod should start and enter a `Running` state after a few seconds.

```bash
kubectl get pods -n argocd-operator-system
```

```bash
NAME                                                  READY   STATUS    RESTARTS   AGE
argocd-operator-controller-manager-6c449c6998-ts95w   2/2     Running   0          33s
```
!!! info
    If you see `Error: container's runAsUser breaks non-root policy`, means container wants to have admin privilege. run `oc adm policy add-scc-to-user privileged -z default -n argocd-operator-system` to enable admin on the namespace and change the following line in deployment resource: `runAsNonRoot: false`. This is a quick fix to make it running, this is not a suggested approach for *production*.
    
## Usage 

Once the operator is installed and running, new ArgoCD resources can be created. See the [usage][docs_usage] 
documentation to learn how to create new `ArgoCD` resources.

## Cleanup 

To remove the operator from the cluster, run the following comand. This will remove all resources that were created,
including the namespace.
```bash
make undeploy
```



[docs_usage]:../usage/basics.md
