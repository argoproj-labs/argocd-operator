
# Development

### Requirements

The requirements for building the operator are fairly minimal.

 * Go 1.14+
 * Operator SDK 0.19+
 * Bash or equivalent 
 * Podman

By default, the project uses [Podman][podman_link] for building container images, this can be changed to `docker` or `buildah` by setting the `ARGOCD_OPERATOR_IMAGE_BUILDER` enviromnet variable to the tool of choice.

### Building from Source

There are several shell scripts provided in the `hack` directory to build and release the operator binaries from source.

#### Environment

There are environment variables defined in `hack/env.sh` that can be overridden as needed.

 * `ARGOCD_OPERATOR_REPO` is the container image repository path.
 * `ARGOCD_OPERATOR_TAG` is the container image version tag.
 * `ARGOCD_OPERATOR_IMAGE_HOST_ORG` is the container image registry host and organization/user. For example: "quay.io/jmckind"

Have a look at the scripts in the `hack` directory for all of the environment variables and how they are used.

#### Build

Run the provided shell script to build the operator. A container image wil be created locally.

``` bash
hack/build.sh
```

### Release

Push a locally created container image to a container registry for deployment.

``` bash
hack/push.sh
```

### Bundle

Bundle the operator for usage in OLM as a CatalogSource.

``` bash
hack/bundle.sh
```
[podman_link]:https://podman.io


### [WIP] Development Process

This is the basic process for development. First, create a branch for the new feature or bug fix.

``` bash
git checkout -b MY_BRANCH
```

Build the development container image. Remember that you can use the value in `ARGOCD_OPERATOR_IMAGE_HOST_ORG` for your image repo.

``` bash
hack/build.sh
```

Push the development container image.

``` bash
hack/push.sh
```

Tag the development container image as latest for testing in a remote cluster.

``` bash
hack/tag.sh
```

Run unit tests. Remember that you can modify `deploy/operator.yaml` to use the value in `ARGOCD_OPERATOR_IMAGE_HOST_ORG` for testing locally.

``` bash
hack/test-unit.sh
```

Run e2e tests and clean the cluster after complete.

``` bash
hack/test.sh
hack/cluster-clean.sh
```

Run scorecard tests.

``` bash
hack/scorecard.sh
```
