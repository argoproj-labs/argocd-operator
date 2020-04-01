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

The following steps can be used to manually install the operator in an OpenShift 4.x environment with minimal overhead.

Several of the steps in this process require the `cluster-admin` ClusterRole or equivalent.

### Authenticate

Once the cluster is up and running, log in as the `cluster-admin` user.

```
oc login -u kubeadmin
```

The following section outlines the steps necessary to deploy the ArgoCD Operator manually using standard Kubernetes manifests.

### Namespace

It is a good idea to create a new namespace for the operator.

```bash
oc new-project argocd
```

The remaining resources will now be created in the new namespace.

### RBAC

Provision the ServiceAccounts, Roles and RoleBindings to set up RBAC for the operator.

NOTE: The ClusterRoleBindings defined in `deploy/role_binding.yaml` use the `argocd` namespace. You will need to update these if using a different namespace.

```bash
oc create -f deploy/service_account.yaml
oc create -f deploy/role.yaml
oc create -f deploy/role_binding.yaml
```

Argo CD needs several ClusterRole resources to function, however the ClusterRoles have been refined to read-only for the cluster resources.

To enable full cluster admin access on OpenShift, run the following command. This step is optional.

``` bash
oc adm policy add-cluster-role-to-user cluster-admin -z argocd-application-controller -n argocd
```

### CRDs

Add the Argo CD CRDs to the cluster.

```bash
oc create -f deploy/argo-cd
```

Add the Argo CD Operator CRDs to the cluster

```bash
oc create -f deploy/crds
```

There should be four CRDs present for Argo CD on the cluster.

```bash
oc get crd | grep argo
```

```bash
NAME                       CREATED AT
applications.argoproj.io   2019-11-09T06:36:59Z
appprojects.argoproj.io    2019-11-09T06:36:59Z
argocdexports.argoproj.io  2019-11-09T06:37:06Z
argocds.argoproj.io        2019-11-09T06:37:06Z
```

### Deploy Operator

Provision the operator using a Deployment manifest.

```bash
oc create -f deploy/operator.yaml
```

An operator Pod should start and enter a `Running` state after a few seconds.

```bash
oc get pods
```

```bash
NAME                              READY   STATUS    RESTARTS   AGE
argocd-operator-758dd86fb-sx8qj   1/1     Running   0          75s
```

## Usage 

Once the operator is installed and running, new ArgoCD resources can be created. See the [usage][docs_usage] 
documentation to learn how to create new `ArgoCD` resources.

[olm_install]:./olm.md#operator-install
[docs_usage]:../usage/basics.md
