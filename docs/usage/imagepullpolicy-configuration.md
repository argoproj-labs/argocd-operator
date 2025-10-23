# ImagePullPolicy Configuration

The ArgoCD Operator supports configurable `imagePullPolicy` settings at multiple levels, providing administrators with flexible control over how container images are pulled for ArgoCD components.

## Configuration Levels

The imagePullPolicy configuration follows a hierarchical precedence system:

1. **Instance-level policy** - specified in the ArgoCD CR via `.spec.imagePullPolicy`.
2. **Global-level policy** - defined through the `IMAGE_PULL_POLICY` environment variable in the Operator’s Subscription.
3. **Default policy** - IfNotPresent (used when neither of the above are specified).

Valid values:
- `Always` - Always pull the image
- `IfNotPresent` - Pull the image only if not present locally
- `Never` - Never pull the image

## Operator Level Configuration

Use this configuration to set a global image pull policy for all Argo CD instances managed by the operator. The value is defined as an environment variable in the operator’s deployment/subscription and applies to all Argo CD instances.

### Environment Variable

Set the global default imagePullPolicy for all ArgoCD instances managed by the operator:

```bash
export IMAGE_PULL_POLICY=IfNotPresent
```

### Subscription Example

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: argocd-operator
spec:
  config:
   env: 
    - name: IMAGE_PULL_POLICY
      value: "IfNotPresent"
  channel: alpha
  name: argocd-operator
  source: argocd-catalog
  sourceNamespace: olm
```

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

This value overrides the operator-level policy and applies only to the specific Argo CD instance defined in the CR.

Set imagePullPolicy for all components in an ArgoCD instance:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  # Instance level imagePullPolicy for all components
  imagePullPolicy: IfNotPresent
```
