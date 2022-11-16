# ArgoRollouts

The `ArgoRollouts` resource is a Kubernetes Custom Resource (CRD) that describes the desired state for the Argo Rollouts Controller.

When the Operator sees a new `ArgoRollouts` resource, the operator creates the Argo Rollouts controller and the associated resources.

The `ArgoRollouts` Custom Resource consists of the following properties.

Name | Default | Description
--- | --- | ---
[**Image**](#image) | `quay.io/argoproj/argo-rollouts` | The container image for the rollouts controller.
[**Env**](#env) | `nil` | Environment Variables for Rollouts.
[**ExtraCommandArgs**](#extraCommandArgs) | `nil` | Additional Arguments to the rollouts container.
[**NodePlacement**](#nodePlacement) | `none` | Allow users to run rollouts controller on the provided node.
[**Resources**](#resources) | `nil` | CPU and Memory requests and limits for the rollouts controller.
[**Version**](#version) | v1.3.1 (SHA) | The tag to use with the container image for the rollouts controller.

## ArgoRollouts Example

The following is an example to use rollouts with the default values.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoRollouts
metadata:
  name: example-rollouts
  labels:
    example: basic
spec: {}
```

## Image

The container image for the rollouts.

### Image Example

The following example sets the default value using the `Image` property.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoRollouts
metadata:
  name: example-rollouts
  labels:
    example: basic
spec: 
  image: <your custom rollouts image>
```

## Version

The tag to use with the rollouts container image.

### Version Example

The following example sets the default value using the `Version` property.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoRollouts
metadata:
  name: example-rollouts
  labels:
    example: basic
spec: 
  version: v1.3.0
```
