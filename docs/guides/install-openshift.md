## OpenShift Install

The following steps can be used to manually install the operator in an OpenShift 4.x environment with minimal overhead.

Several of the steps in this process require the `cluster-admin` ClusterRole or equivalent.

### Cluster

This guide uses [OpenShift 4](https://try.openshift.com/), follow the 
guide for your platform to install. 

Once the cluster is up and running, log in as the `cluster-admin` user.

```
oc login -u kubeadmin
```

### Manual Install

The following section outlines the steps necessary to deploy the ArgoCD Operator manually using standard Kubernetes manifests.

#### Namespace

It is a good idea to create a new namespace for the operator.

```bash
oc new-project argocd
```

The remaining resources will now be created in the new namespace.

#### RBAC

Provision the ServiceAccounts, Roles and RoleBindings to set up RBAC for the operator.

```bash
oc create -f deploy/service_account.yaml
oc create -f deploy/role.yaml
oc create -f deploy/role_binding.yaml
```

#### Cluster Admin

By default Argo CD prefers to run with the cluster-admin role. Give cluster-admin access to the Argo CD Application Controller.
Be sure to update `ARGO_NS` to use the actual namespace where you have the operator installed.

```bash
export ARGO_NS=argocd
oc adm policy add-cluster-role-to-user cluster-admin system:serviceaccount:${ARGO_NS}:argocd-application-controller
```

#### CRDs

Add the Argo CD CRDs to the cluster.

```bash
oc create -f deploy/argo-cd
```

Add the Argo CD Operator CRD to the cluster

```bash
oc create -f deploy/crds/argoproj_v1alpha1_argocd_crd.yaml
```

There should be three CRDs present for Argo CD on the cluster.

```bash
oc get crd | grep argo
```

```bash
NAME                       CREATED AT
applications.argoproj.io   2019-11-09T06:36:59Z
appprojects.argoproj.io    2019-11-09T06:36:59Z
argocds.argoproj.io        2019-11-09T06:37:06Z
```

#### Deploy Operator

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

#### ArgoCD Instance

Once the operator is deployed and running, create a new ArgoCD custom resource.
The following example shows the minimal required to create a new ArgoCD
environment with the default configuration.

```bash
oc create -f examples/argocd-minimal.yaml
```

There will be several resources created.

```bash
oc get pods
```
```bash
NAME                                                     READY   STATUS    RESTARTS   AGE
argocd-minimal-application-controller-7c74b5855b-brn7s   1/1     Running   0          29s
argocd-minimal-dex-server-859bd5458c-78c8k               1/1     Running   0          29s
argocd-minimal-redis-6986d5fdbd-vzzjp                    1/1     Running   0          29s
argocd-minimal-repo-server-7bfc477c58-q7d8g              1/1     Running   0          29s
argocd-minimal-server-7d56c5bf4d-9wxz6                   1/1     Running   0          29s
argocd-operator-758dd86fb-qshll                          1/1     Running   0          51s
```

The ArgoCD Server should be available via an OpenShift Route.

```bash
oc get routes
```

```bash
NAME                        HOST/PORT                                               PATH   SERVICES                 PORT   TERMINATION     WILDCARD
argocd-minimal-grafana      argocd-minimal-grafana-argocd.apps.test.runk8s.com             argocd-minimal-grafana   http                   None
argocd-minimal-prometheus   argocd-minimal-prometheus-argocd.apps.test.runk8s.com          prometheus-operated      web                    None
argocd-minimal-server       argocd-minimal-server-argocd.apps.test.runk8s.com              argocd-minimal-server    http   edge/Redirect   None
```

The Route is `argocd-minimal-server` in this example and should be available at
the HOST/PORT value listed. The admin password is the name for the server Pod
from above (`argocd-minimal-server-7d56c5bf4d-9wxz6` in this example).

Follow the ArgoCD [Getting Started Guide](https://argoproj.github.io/argo-cd/getting_started/#creating-apps-via-ui) 
to create a new application from the UI.
