
# Development

### Requirements

The requirements for building the operator are fairly minimal.

 * Bash or equivalent 
 * Go 1.12+
 * Operator SDK 0.10+

### Building from Source

There are several shell scripts provided in the `hack` directory to build and release the operator binaries from source.

#### Environment

There are environment variables defined in `hack/env.sh` that can be overridden as needed.

 * `ARGOCD_OPERATOR_REPO` is the container image repository path.
 * `ARGOCD_OPERATOR_TAG` is the container image version tag.

Have a look at the scripts in the `hack` directory for all of the environment variables and how they are used.

#### Build

Run the provided shell script to build the operator. A container image wil be created locally.

```bash
hack/build.sh
```

### Release

Push a locally created container image to a container registry for deployment.

```bash
hack/push.sh
```

### Bundle

Bundle the operator for usage in OLM as a CatalogSource.

```bash
hack/bundle.sh
```
