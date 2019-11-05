# Argo CD Operator

A Kubernetes operator for managing Argo CD deployments.

## Basic Usage

Set up RBAC for the operator

```bash
kubectl create -f deploy/service_account.yaml
kubectl create -f deploy/role.yaml
kubectl create -f deploy/role_binding.yaml
```

Add the Argo CD server CRDs to the cluster.

```bash
kubectl create -f deploy/argo-cd
```

Add the ArgoCD Operator CRD to the cluster

```bash
kubectl create -f deploy/crds/argoproj_v1alpha1_argocd_crd.yaml
```

Deploy the operator

```bash
kubectl create -f deploy/operator.yaml
```

Once the operator is deployed, create a new ArgoCD custom resource.

```bash
kubectl create -f examples/argocd-minimal.yaml
```

## Deployment Guides

See the [deployment guides][deploy_guides] for different platforms.

## Contributing

See the [development documentation][dev_docs] for information on how to contribute!

## License

ArgoCD Operator is released under the Apache 2.0 license. See the [LICENSE][license_file] file for details.

[deploy_guides]:./docs/guides/
[dev_docs]:./docs/development.md
[license_file]:./LICENSE
