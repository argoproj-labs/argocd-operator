# Argo CD Operator Bundle for OpenShift Marketplace

Create Argo CD Operator Bundle for quay.io.

## Note

This directory contains everything needed to create a Bundle for quay.io. All files (instead of argo.png) have to be in the same directory to create and push the bundle.

argoproj_v1alpha1_argocd_crd.yaml (This file is untouched and copied from deploy/crds to have it available in this directory.)

argoproj_v1alpha1_appproject_crd.yaml (added: `spec.version: v1alpha1` Without this the Bundle is not valid.)

argoproj_v1alpha1_application_crd.yaml (added: `spec.version: v1alpha1` Without this the Bundle is not valid.)

## Create and Push the Bundle to quay.io

To create the Bundle you need:

```bash
pip3 install operator-courier
```

And if you want help to create a token from quay.io:

```bash
git clone https://github.com/operator-framework/operator-courier
```

### Create a new `Application Repository` at quay.io

```bash
Create New Repository -> Application Repository -> Repository Name: argocd-operators -> Public
```



### Get an auth token from quay.io

```bash
./operator-courier/scripts/get-quay-token
Username: <your username>
Password: <your password>
{"token":"basic xyzxyzxyzxyzxyzxyzxyzxyzxyzxyzxyz"}
```

### Apply for you!

Change in file: 

```bash
argocd-operator.v0.0.1.clusterserviceversion.yaml

search for : disposab1e

and replace with: john_mckenzie

```

And change whatever you need to....


### Verify Bundle

```bash
operator-courier verify --ui_validate_io marketplace/openshift/bundle
```

### Push Bundle

```bash
operator-courier \
    push "marketplace/openshift/bundle" \
    "john_mckenzie" "argocd-operators" "0.0.1" "basic xyzxyzxyzxyzxyzxyzxyzxyzxyzxyzxyz=="
```

