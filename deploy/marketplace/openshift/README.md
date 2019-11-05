# Argo CD Operator for OpenShift Marketplace

Install Argo CD Operator and Argo CD from OperatorHub.

## Note

Initial work to make the operator available through OperatorHub Console in OpenShift 4.2.

In the moment it is hosted at quay.io in namespace `disposab1e` but with a few changes you can host it under your namespace `john_mckenzie`.

## Resources

Argo CD Operator (latest version from master)

https://quay.io/repository/disposab1e/argocd-operator

Argo CD Operators (Operator Registry)

https://quay.io/application/disposab1e/argocd-operators


## Prerequisites

CodeReady Containers is installed with NO manual deployment of the operator and crd's, cr's. 

## Installation

Cause the operator is namespace scope it will be installed in the namesapce `argoproj`.

```bash

oc login -u kubeadmin -p <password> https://api.crc.testing:6443

kubectl create ns argoproj
kubectl apply -f marketplace/openshift/deploy/operator-group.yaml -n argoproj
kubectl apply -f marketplace/openshift/deploy/operator-source.yaml -n openshift-marketplace

kubectl rollout status -w deployment/argocd-operators -n openshift-marketplace

kubectl apply -f marketplace/openshift/deploy/operator-subscription.yaml -n argoproj

kubectl apply -f marketplace/openshift/deploy/argoproj_v1alpha1_argocd_cr.yaml -n argoproj

````

## View Console

Operator in OperatorHub:
https://console-openshift-console.apps-crc.testing/operatorhub/ns/argoproj?providerType=%5B%22Custom%22%5D

Installed Operator:
https://console-openshift-console.apps-crc.testing/k8s/ns/argoproj/clusterserviceversions

Deployed Argo CD:
https://console-openshift-console.apps-crc.testing/k8s/ns/argoproj/pods

## Cleanup

```bash
kubectl delete ArgoCD argocd -n argoproj
kubectl delete AppProject default -n argoproj
kubectl delete csv argocd-operator.v0.0.1 -n argoproj
kubectl delete subscription argocd-operators -n argoproj
kubectl delete operatorgroup argocd-operators -n argoproj
kubectl delete operatorsource argocd-operators -n openshift-marketplace
kubectl delete -f marketplace/openshift/bundle/argoproj_v1alpha1_argocd_crd.yaml
kubectl delete -f marketplace/openshift/bundle/argoproj_v1alpha1_appproject_crd.yaml
kubectl delete -f marketplace/openshift/bundle/argoproj_v1alpha1_application_crd.yaml
kubectl delete ns argoproj
````
