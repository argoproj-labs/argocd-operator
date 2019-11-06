# Argo CD Operator with CodeReady Containers on macOS 

A quickstart guide to run `Argo CD` with the `guestbook` example on `CodeReady Containers` on `macOS Catalina`.

Please note: This is not a beginner guide. You should be familiar with docker, go, operator-sdk, crc and argo-cd cli and everything must be properly installed and running.

With a few changes you can adopt it easily to your personal environment.


##  Prerequisites

  * Docker (19.03.2) other versions should work too
  * Go 1.12+
  * Operator SDK 0.10+
  * CodeReady Containers (1.0.0-rc.0+34371d3)
  * Argo CD CLI (v1.2.3)

## Prepare your CodeReady Containers installation

```bash

crc delete

rm -rf ~/.crc

crc setup --vm-driver hyperkit

crc config set cpus 4
crc config set memory 13000

crc start

eval $(crc oc-env)

```

Remember the password for the `kubeadmin` user which is displayed after `crc start`. For example: `F44En-Xau6V-jQuyb-yuMXB` 

## Login OpenShift

```bash
oc login -u kubeadmin -p F44En-Xau6V-jQuyb-yuMXB https://api.crc.testing:6443
```

## Open OpenShift Console

```bash
crc console
```

Login with username `kube:admin` and password `F44En-Xau6V-jQuyb-yuMXB`




## Start monitoring, alerting, and telemetry services

```bash
oc scale --replicas=1 statefulset --all -n openshift-monitoring; oc scale --replicas=1 deployment --all -n openshift-monitoring
```

Point your Browser to:

https://console-openshift-console.apps-crc.testing/k8s/ns/openshift-monitoring/pods 

and wait until all pods are `Ready`.


## Create a new argoproj Project

```bash
oc new-project argoproj
```

## Login to OpenShift Registry

```bash
docker login -u kubedamin -p $(oc whoami -t) default-route-openshift-image-registry.apps-crc.testing
```

## Clone argocd-operator Git Repository

```bash
cd ~/
git clone https://github.com/jmckind/argocd-operator.git
```

## Build latest argocd-operator Operator

```bash
cd ~/argocd-operator

operator-sdk build default-route-openshift-image-registry.apps-crc.testing/argoproj/argocd-operator:latest
```

## Push Operator to OpenShift Registry in Project argoproj

```bash
docker push default-route-openshift-image-registry.apps-crc.testing/argoproj/argocd-operator:latest
```

## Set up RBAC for the operator

```bash
oc create -f deploy/service_account.yaml
oc create -f deploy/role.yaml
oc create -f deploy/role_binding.yaml
```

## Add the CRDs to the cluster

```bash
oc create -f deploy/crds/argoproj_v1alpha1_application_crd.yaml
oc create -f deploy/crds/argoproj_v1alpha1_appproject_crd.yaml
oc create -f deploy/crds/argoproj_v1alpha1_argocd_crd.yaml
```

## Deploy the operator

```bash
oc create -f deploy/operator.yaml
```

Point your Browser to:

https://console-openshift-console.apps-crc.testing/k8s/ns/argoproj/pods

and wait until the operator pod is `Ready`.



## Create a new Argo CD custom resource

```bash
oc create -f deploy/crds/argoproj_v1alpha1_argocd_cr.yaml
```

Again point your Browser to:

https://console-openshift-console.apps-crc.testing/k8s/ns/argoproj/pods

and wait until alls pods are `Ready`.


## Get your crc ip

```bash
crc ip

Example output: 192.168.64.45
```

## Prepare your /etc/hosts

```bash
sudo vi /etc/hosts

look for an entry: 192.168.64.45 api.crc.testing oauth-openshift.apps-crc.testing

and change it to: 192.168.64.45 api.crc.testing oauth-openshift.apps-crc.testing argocd-server-route-argoproj.apps-crc.testing
```


## Get your initial Argo CD password

```bash
oc get pods -n argoproj -l app.kubernetes.io/name=argocd-server -o name | cut -d'/' -f 2 

Example: argocd-server-67df99877f-tk9zz
```


## Login to Argo CD the first time with CLI

```bash
argocd login --insecure --username admin --password argocd-server-67df99877f-tk9zz argocd-server-route-argoproj.apps-crc.testing
```

## Change your Argo CD initial password to admin

```bash
argocd account update-password --insecure --current-password argocd-server-67df99877f-tk9zz --new-password admin 
```

## Allow the Argo CD controller to act as cluster-admin

```bash
oc adm policy add-cluster-role-to-user cluster-admin system:serviceaccount:argoproj:argocd-application-controller
```

## Install the Argo CD guestbook example in the `default`namespace

```bash
argocd app create guestbook \
  --repo https://github.com/argoproj/argocd-example-apps.git \
  --path guestbook \
  --dest-server https://kubernetes.default.svc \
  --dest-namespace default
```

## Check the existence of your new app

```bash
argocd app get guestbook
```
Wait until you see something like this:

```bash
Name:               guestbook
Project:            default
Server:             https://kubernetes.default.svc
Namespace:          default
URL:                https://argocd-server-route-argoproj.apps-crc.testing/applications/guestbook
Repo:               https://github.com/argoproj/argocd-example-apps.git
Target:             
Path:               guestbook
Sync Policy:        <none>
Sync Status:        OutOfSync from  (94ad32f)
Health Status:      Missing

GROUP  KIND        NAMESPACE        NAME          STATUS     HEALTH   HOOK  MESSAGE
       Service     argocd-examples  guestbook-ui  OutOfSync  Missing        
apps   Deployment  argocd-examples  guestbook-ui  OutOfSync  Missing 
```


## Sync your App

```bash
argocd app sync guestbook
```

# Enjoy!

Point your Browser to https://argocd-server-route-argoproj.apps-crc.testing and login with username `admin`and password `admin`

Hope this mini guide helps to get you started!