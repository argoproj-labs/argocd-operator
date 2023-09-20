# OpenShift Install

This guide uses [OpenShift 4](https://try.openshift.com/), follow the guide for your platform to install.

Once the OpenShift cluster is up and running, the operator can be deployed to watch one or more namespaces. The
preferred method to install the operator is using the OpenShift console. The operator can also be installed manually if desired.

## Console Install

The operator is published in the Operator Hub with the OpenShift console. Log into the console using the URL for your
cluster and select the Operators link, then select the OperatorHub link to display the list of operators.

Select the operator named `Argo CD` and click the **Install** button. You can select the namespace and deploy the operator.

In addition to the console interface, the [Operator Install][olm_install] section of the OLM Install Guide details the same method using manifests.

## Manual Install

The following steps can be used to manually install the operator in an OpenShift 4.x environment with minimal overhead. Note that these steps generates the manifests using kustomize.

Several of the steps in this process require the `cluster-admin` ClusterRole or equivalent.

### Authenticate

Once the cluster is up and running, log in as the `cluster-admin` user.

```
oc login -u kubeadmin
```

!!! info
    Make sure you download the source code from release section: https://github.com/argoproj-labs/argocd-operator/releases. Compiling from the source code cloned off main repo may not provide the most stable result.

### Namespace

By default, the operator is installed into the `argocd-operator-system` namespace. To modify this, update the
value of the `namespace` specified in the `config/default/kustomization.yaml` file. 

### Conversion Webhook Support

ArgoCD `v1alpha1` CRD has been **deprecated** starting from **argocd-operator v0.8.0**. To facilitate automatic migration of existing `v1alpha1` ArgoCD CRs to `v1beta1`, conversion webhook support has been introduced.

By default, the conversion webhook is disabled for the manual(non-OLM) installation of the operator. Users can modify the configurations to enable conversion webhook support using the instructions provided below.

!!! warning
    Enabling the webhook is optional. However, without conversion webhook support, users are responsible for migrating any existing ArgoCD v1alpha1 CRs to v1beta1.

##### Enable Webhook Support

To enable the operator to utilize the `Openshift Service CA Operator` for automated webhook certificate management, add following annotations.

`config/webhook/service.yaml`
```yaml
metadata:
  name: webhook-service
  annotations: 
    service.beta.openshift.io/serving-cert-secret-name: webhook-server-cert
```

`config/crd/patches/cainjection_in_argocds.yaml`
```yaml
metadata:
  name: argocds.argoproj.io
  annotations: 
    service.beta.openshift.io/inject-cabundle: true
```

Additionally, set the `ENABLE_CONVERSION_WEBHOOK`` environment variable in the operator to enable the conversion webhook.

`config/default/manager_webhook_patch.yaml`
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
oc get pods -n <argocd-operator-system>
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
