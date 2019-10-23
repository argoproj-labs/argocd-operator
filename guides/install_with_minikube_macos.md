# Argo CD Operator with Minikube on macOS 

A quickstart guide to run `Argo CD` with the `guestbook` example on `Minikube` and `macOS Catalina`.

Features:
* Minikube (Dashboard, Registry)
* Operator Lifecycle Manager and Catalogue
* Grafana, Prometheus, Alert Manager
* Argo CD with Argo CD Operator


Please note: This is not a beginner guide. You should be familiar with docker, go, operator-sdk, minikube and argo-cd cli and everything must be properly installed and running.

With a few changes you can adopt it easily to your personal environment.


##  Prerequisites

  * Docker (19.03.2) other versions should work too
  * Go 1.12+
  * Operator SDK 0.10+
  * Minikube (1.4.0)
  * Argo CD CLI (v1.2.3)

## Prepare your Minikube installation

```bash
mkdir ~/my-argoproj && cd ~/my-argoproj

minikube profile argoproj

minikube start -p argoproj \
    --vm-driver=hyperkit \
    --memory=16384 \
    --kubernetes-version=v1.16.0 \
    --bootstrapper=kubeadm \
    --extra-config=kubelet.authentication-token-webhook=true \
    --extra-config=kubelet.authorization-mode=Webhook \
    --extra-config=scheduler.address=0.0.0.0 \
    --extra-config=controller-manager.address=0.0.0.0

minikube addons disable metrics-server
minikube addons enable registry
minikube addons enable dashboard
```

## Install Operator Lifecycle Manager

```bash
kubectl apply -f \
  https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.12.0/crds.yaml

kubectl apply -f \
  https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.12.0/olm.yaml


kubectl rollout status -w deployment/olm-operator -n olm
kubectl rollout status -w deployment/catalog-operator -n olm
kubectl rollout status -w deployment/packageserver -n olm
```

## Install Operator Lifecycle Manager Console and Catalogue (optional)

Please note! You need a local (not inside Minikube!) Docker or Podman installation. 

```bash
cd ~/my-argoproj && \
  git clone https://github.com/operator-framework/operator-lifecycle-manager.git && \
  cd operator-lifecycle-manager && \
  ./scripts/run_console_local.sh
```
Point your Browser to: http://localhost:9000 

## Start Minikube Dashboard (optional)

```bash
minikube dashboard -p argoproj
```

## Install Prometheus, Grafana and Alert Manager

```bash
cd ~/my-argoproj && \
  git clone https://github.com/coreos/kube-prometheus.git && \
  cd kube-prometheus && \
  kubectl apply -f manifests/
```

If you see something like this: `unable to recognize "manifests/....` run the following command again:

```bash
kubectl apply -f manifests/
```

```bash
kubectl rollout status -w deployment/prometheus-operator -n monitoring
kubectl rollout status -w deployment/grafana -n monitoring
kubectl rollout status -w deployment/kube-state-metrics -n monitoring
kubectl rollout status -w deployment/prometheus-adapter -n monitoring
```

## Open Prometheus in your Browser (optional)

```bash
kubectl port-forward svc/prometheus-k8s 9090 -n monitoring
```
Point your Browser to: http://localhost:9090

## Open Grafana in your Browser (optional)

```bash
kubectl port-forward svc/grafana 3000 -n monitoring
```
Point your Browser to: http://localhost:3000 and login with username `admin` and password `admin`

## Open Alert Manager in your Browser (optional)

```bash
kubectl port-forward svc/alertmanager-main 9093 -n monitoring
```
Point your Browser to: http://localhost:9093


## Create a new argoproj Namespace

```bash
kubectl create ns argoproj

kubectl config set-context --current --namespace=argoproj
```

## Clone argocd-operator Git Repository

```bash
cd ~/my-argoproj && \
  git clone https://github.com/jmckind/argocd-operator.git
```

## Build latest argocd-operator operator

```bash
cd ~/my-argoproj/argocd-operator && \
  operator-sdk build $(minikube ip -p argoproj):5000/argoproj/argocd-operator:latest
```

## Push Operator to Minikube Registry

```bash
docker push $(minikube ip -p argoproj):5000/argoproj/argocd-operator:latest
```

## Set up RBAC for the operator

```bash
kubectl create -f deploy/service_account.yaml
kubectl create -f deploy/role.yaml
kubectl create -f deploy/role_binding.yaml
```

## Add the CRDs to the cluster

```bash
kubectl create -f deploy/crds/argoproj_v1alpha1_application_crd.yaml
kubectl create -f deploy/crds/argoproj_v1alpha1_appproject_crd.yaml
kubectl create -f deploy/crds/argoproj_v1alpha1_argocd_crd.yaml
```

## Deploy the operator

```bash
kubectl create -f deploy/operator.yaml


kubectl wait pod -n argoproj -l name=argocd-operator --for=condition=ready
```

## Patch deplyoment to use your previously build operator

```bash
kubectl patch -n argoproj deployment argocd-operator \
--patch '{"spec": {"template": {"spec": {"containers": [{"name": "argocd-operator","image": "localhost:5000/argoproj/argocd-operator:latest"}]}}}}'


kubectl wait pod -n argoproj -l name=argocd-operator --for=condition=ready
```

## Create a new Argo CD custom resource

```bash
kubectl create -f deploy/crds/argoproj_v1alpha1_argocd_cr.yaml


kubectl wait pod -n argoproj -l app.kubernetes.io/name=argocd-server --for=condition=ready
```

## Get your initial Argo CD password

```bash
kubectl get pods -n argoproj -l app.kubernetes.io/name=argocd-server -o name | cut -d'/' -f 2
```
Example output: argocd-server-5689549cfd-c8ndr

## Expose Argo CD API

```bash
kubectl port-forward svc/argocd-server -n argoproj 5443:443
```

## Login to Argo CD the first time with CLI

```bash
argocd login --insecure --username admin \
  --password argocd-server-5689549cfd-c8ndr localhost:5443
```

## Change your Argo CD initial password to admin

```bash
argocd account update-password --insecure  \
  --current-password argocd-server-5689549cfd-c8ndr --new-password admin 
```

## Allow the Argo CD controller to act as cluster-admin

```bash
kubectl create clusterrolebinding argocd-clsuter-admin \
  --clusterrole=cluster-admin --serviceaccount=argoproj:argocd-application-controller
```

## Install the Argo CD guestbook example

```bash
kubectl create ns argoproj-examples

argocd app create guestbook \
  --repo https://github.com/argoproj/argocd-example-apps.git \
  --path guestbook \
  --dest-server https://kubernetes.default.svc \
  --dest-namespace argoproj-examples
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
Namespace:          argoproj-examples
URL:                https://localhost:5443/applications/guestbook
Repo:               https://github.com/argoproj/argocd-example-apps.git
Target:             
Path:               guestbook
Sync Policy:        <none>
Sync Status:        OutOfSync from  (94ad32f)
Health Status:      Missing

GROUP  KIND        NAMESPACE          NAME          STATUS     HEALTH   HOOK  MESSAGE
       Service     argoproj-examples  guestbook-ui  OutOfSync  Missing        
apps   Deployment  argoproj-examples  guestbook-ui  OutOfSync  Missing   
```

## Sync your guetbook app

```bash
argocd app sync guestbook
```

# View the guestbook app in Argo CD!

Point your Browser to: https://localhost:5443/applications/guestbook and login with username `admin`and password `admin`


# Install Grafana Dashboard for Argo CD

Please note! This is in an early preview and will change frequently!

If not already done:

```bash
kubectl port-forward svc/grafana 3000 -n monitoring
```

Minikube Prometheus default database ist named `prometheus` so we need to change it:

```bash
sed -i -e 's/Prometheus/prometheus/g' ~/my-argoproj/argocd-operator/grafana/dashbaords/argocd.json
```
Point your Browser to: http://localhost:3000 and login with username `admin` and password `admin`

Navigate to: `Dashboard / Manage / Import / Upload .json File` and import `~/argoproj/argocd-operator/grafana/dashboards/argocd.json` file.


Hope this mini guide helps to get started!
