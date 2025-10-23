# ImagePullPolicy Configuration

The ArgoCD Operator supports configurable `imagePullPolicy` settings at multiple levels, providing administrators with flexible control over how container images are pulled for ArgoCD components.

## Configuration Levels

The imagePullPolicy configuration follows a hierarchical precedence system:

1. **Instance-level policy** - specified in the ArgoCD CR via `.spec.imagePullPolicy`.
2. **Global-level policy** - defined through the `IMAGE_PULL_POLICY` environment variable in the Operator’s Subscription.
3. **Default policy** - IfNotPresent (used when neither of the above are specified).

## Operator Level Configuration

### Environment Variable

Set the global default imagePullPolicy for all ArgoCD instances managed by the operator:

```bash
export IMAGE_PULL_POLICY=IfNotPresent
```

Valid values:
- `Always` - Always pull the image
- `IfNotPresent` - Pull the image only if not present locally
- `Never` - Never pull the image

### Operator Deployment Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-operator-controller-manager
  namespace: argocd-operator-system
spec:
  template:
    spec:
      containers:
      - name: manager
        env:
        - name: IMAGE_PULL_POLICY
          value: "IfNotPresent"
```

## ArgoCD Instance Level Configuration

### Global Policy

Set a global imagePullPolicy for all components in an ArgoCD instance:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  # Global imagePullPolicy for all components
  imagePullPolicy: IfNotPresent
```



## Migration from Hardcoded Values

The operator previously used hardcoded imagePullPolicy values:
- Most components: `PullAlways`
- Redis HA components: `PullIfNotPresent`

With the new configuration system, you can:
1. Set operator-level defaults to maintain current behavior
2. Gradually migrate to more appropriate policies per environment

