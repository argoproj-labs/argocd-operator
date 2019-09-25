# Argo CD Operator

A Kubernetes operator for managing Argo CD deployments.

## Usage

Set up RBAC for the operator

```bash
kubectl create -f deploy/service_account.yaml
kubectl create -f deploy/role.yaml
kubectl create -f deploy/role_binding.yaml
```

Add the CRDs to the cluster

```bash
kubectl create -f deploy/crds/argoproj_v1alpha1_application_crd.yaml
kubectl create -f deploy/crds/argoproj_v1alpha1_appproject_crd.yaml
kubectl create -f deploy/crds/argoproj_v1alpha1_argocd_crd.yaml
```

Deploy the operator

```bash
kubectl create -f deploy/operator.yaml
```

Once the operator is deployed, create a new Argo CD custom resource.

```bash
kubectl create -f deploy/crds/argoproj_v1alpha1_argocd_cr.yaml
```

## Development

The requirements for building the operator are fairly minimal.

 * Go 1.12+
 * Operator SDK 0.10+

Ensure Go module support is enabled in your environment.

```bash
export GO111MODULE=on

```

Run the build subcommand that is part of the Operator SDK to build the operator.

```bash
operator-sdk build <YOUR_IMAGE_REPO>/argocd-operator
```
