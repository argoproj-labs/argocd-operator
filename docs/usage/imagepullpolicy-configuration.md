# ImagePullPolicy Configuration

The ArgoCD Operator supports configurable `imagePullPolicy` settings at multiple levels, providing administrators with flexible control over how container images are pulled for ArgoCD components.

## Configuration Levels

The imagePullPolicy configuration follows a hierarchical precedence system:

1. **Global ArgoCD policy** 
2. **Subscription level environment variable** 
3. **Default value** (lowest priority)

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


## Configuration Precedence Examples

### Example 1: Operator Default
```yaml
# Operator env: IMAGE_PULL_POLICY=IfNotPresent
# ArgoCD CR: No imagePullPolicy specified
# Result: All components use IfNotPresent
```

### Example 2: Global Override
```yaml
# Operator env: ARGOCD_IMAGE_PULL_POLICY=IfNotPresent
# ArgoCD CR: spec.imagePullPolicy: Always
# Result: All components use Always
```

## Use Cases

### Development Environment
```yaml
# Use IfNotPresent to avoid unnecessary image pulls
spec:
  imagePullPolicy: IfNotPresent
```

### Production Environment
```yaml
# Use Always to ensure latest images
spec:
  imagePullPolicy: Always
```

### Air-Gapped Environment
```yaml
# Use Never for pre-loaded images
spec:
  imagePullPolicy: Never
```

## Migration from Hardcoded Values

The operator previously used hardcoded imagePullPolicy values:
- Most components: `PullAlways`
- Redis HA components: `PullIfNotPresent`

With the new configuration system, you can:
1. Set operator-level defaults to maintain current behavior
2. Gradually migrate to more appropriate policies per environment

## Validation

The imagePullPolicy values are validated using Kubernetes' built-in validation:
- `Always`
- `IfNotPresent` 
- `Never`

Invalid values will be rejected by the API server during resource creation/update.

