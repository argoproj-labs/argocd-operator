# Resource Requirements

This page covers the steps to create, update and delete resource requests and limits for Argo CD workloads.

The Argo CD Custom Resource allows you to create the workloads with desired resource requests and limits. This is required when a user/admin wishes to deploy his Argo CD instance in a namespace that is set with [Resource Quota](https://kubernetes.io/docs/concepts/policy/resource-quotas/).

For example, the below Argo CD instance deploys the Argo CD workloads such as Application Controller, ApplicationSet Controller, Dex, Redis, Repo Server and Server with resource requests and limits. Similarly you can also create the other workloads with resource requirements.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example
spec:
  server:
    resources:
      limits:
        cpu: 500m
        memory: 256Mi
      requests:
        cpu: 125m
        memory: 128Mi
    route:
      enabled: true
  applicationSet:
    resources:
      limits:
        cpu: '2'
        memory: 1Gi
      requests:
        cpu: 250m
        memory: 512Mi
  repo:
    resources:
      limits:
        cpu: '1'
        memory: 512Mi
      requests:
        cpu: 250m
        memory: 256Mi
  sso:
    dex:
      resources:
        limits:
          cpu: 500m
          memory: 256Mi
        requests:
          cpu: 250m
          memory: 128Mi
  redis:
    resources:
      limits:
        cpu: 500m
        memory: 256Mi
      requests:
        cpu: 250m
        memory: 128Mi
  controller:
    resources:
      limits:
        cpu: '2'
        memory: 2Gi
      requests:
        cpu: 250m
        memory: 1Gi
```

!!! note
    The above mentioned resource requirements for the workloads are not the recommended values. Please do not consider them as defaults for your instance.

## Patch the Argo CD instance to update the resource requirements

A User can update the resource requirements for all or any of your workloads post installation.

For example, a user can update the Application Controller resource requests of `example` Argo CD instance in `argocd` namespace using the below commands.

```sh
kubectl -n argocd patch argocd example --type='json' -p='[{"op": "replace", "path": "/spec/controller/resources/requests/cpu", "value":"1"}]'
```

```sh
kubectl -n argocd patch argocd example --type='json' -p='[{"op": "replace", "path": "/spec/controller/resources/requests/memory", "value":"512Mi"}]'
```

Similarly, A user can update the Application Controller resource limits of `example` Argo CD instance in `argocd` namespace using the below commands.

```sh
kubectl -n argocd patch argocd example --type='json' -p='[{"op": "replace", "path": "/spec/controller/resources/limits/cpu", "value":"4"}]'
```

```sh
kubectl -n argocd patch argocd example --type='json' -p='[{"op": "replace", "path": "/spec/controller/resources/limits/memory", "value":"2048Mi"}]'
```

The above commands can be modified to replace controller with any other Argo CD workloads such as ApplicationSet Controller, Dex, Redis, Repo Server, Server and others.

## Remove the resource requirements for Argo CD workloads

A User can also remove resource requirements for all or any of your workloads post installation.

For example, A user can remove the Application Controller resource requests of `example` ArgoCD instance in `argocd` namespace using the below command.

```sh
kubectl -n argocd patch argocd example --type='json' -p='[{"op": "remove", "path": "/spec/controller/resources/requests/cpu"}]'
```

```sh
kubectl -n argocd patch argocd example --type='json' -p='[{"op": "remove", "path": "/spec/controller/resources/requests/memory"}]'
```

Similarly, A user can remove the Application Controller resource limits of `example` Argo CD instance in `argocd` namespace using the below command.

```sh
kubectl -n argocd patch argocd example --type='json' -p='[{"op": "remove", "path": "/spec/controller/resources/limits/cpu"}]'
```

```sh
kubectl -n argocd patch argocd example --type='json' -p='[{"op": "remove", "path": "/spec/controller/resources/limits/memory"}]'
```

The above commands can be modified to replace controller with any other Argo CD workloads such as ApplicationSet Controller, Dex, Redis, Repo Server, Server and others.
