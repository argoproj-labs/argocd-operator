# Helm Chart for Argo CD Operator


## Prerequisites
* Helm
* Operator Lifecycle Manager

If not already installed... 

### Install Operator Lifecycle Manager

For Kubernetes installations you must have the Operator Lifecycle Manager up and running. Tested with Minikube and Google Cloud Platform Kubernetes Engine.

```bash
kubectl apply -f \
  https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.12.0/crds.yaml

kubectl apply -f \
  https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.12.0/olm.yaml

kubectl rollout status -w deployment/olm-operator -n olm
kubectl rollout status -w deployment/catalog-operator -n olm
kubectl rollout status -w deployment/packageserver -n olm
```


### Install Helm

For clusters with RBAC enabled:

```bash
kubectl --namespace kube-system create sa tiller

kubectl create clusterrolebinding tiller \
    --clusterrole cluster-admin \
    --serviceaccount=kube-system:tiller

helm init --service-account tiller

kubectl rollout status -w deployment/tiller-deploy -n kube-system
```

For clusters without RBAC enabled:
```bash
helm init
```

## Install Argo CD Operator (local directory)

Please Note! You MUST install everything in namespace `argproj`. This might change in the future!

Install:

```bash
cd helm/argocd-operator

helm install --namespace argoproj --name argocd-operator .
```


## Install Argo CD Operator (Helm Repository)

```bash
helm repo add argocd-operator https://disposab1e.github.io/helm-repo

helm repo list 

helm install  --namespace argoproj --name argocd-operator argocd-operator/argocd-operator --version 0.0.1
```


## Check Argo CD Operator installation

```bash
helm ls --all argocd-operator

kubectl rollout status -w deployment/argocd-operator -n argoproj

kubectl get crd | grep argoproj.io
```


## Install Argo CD

```bash
kubectl apply -f marketplace/openshift/deploy/argoproj_v1alpha1_argocd_cr.yaml -n argoproj
```


## Cleanup

### Delete Argo CD installation

```bash
kubectl delete ArgoCD argocd -n argoproj
kubectl delete AppProject default -n argoproj
```

### Delete Argo CD Operator installation

Please Note! You have to delete the crd's manually. This is not done with helm!

```bash
helm delete argocd-operator --purge

helm ls --all 

kubectl delete crd applications.argoproj.io
kubectl delete crd appprojects.argoproj.io 
kubectl delete crd argocds.argoproj.io 

kubectl delete ns argoproj

kubectl get crd | grep argoproj.io

kubectl get all --all-namespaces | grep argo
```
