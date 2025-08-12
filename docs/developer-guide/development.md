
# Development

### Requirements

The requirements for building the operator are fairly minimal.

 * Go 1.16+
 * Operator SDK 1.11.0+
 * Bash or equivalent
 * Docker
 
### Building from Source

The `Makefile` in the root directory contains several targets to build and release the operator binaries from source.

#### Environment

The `Makefile` defines several variables to control the names of the images to build and push.
These variables can either be set as environment variables, or specified when invoking `make`.

 * `IMG` is the image URL to use all building/pushing image targets.
 * `BUNDLE_IMG` defines the image:tag used for the bundle.

Have a look `Makefile` for all of the variables and how they are used.

### Build

Use the following make target to build the operator. A container image wil be created locally. The name of the image is specified by the `IMG` variable defined in the `Makefile`.

``` bash
make docker-build
```

### Release

Push a locally created container image to a container registry for deployment.  The name of the image is specified by the `IMG` variable defined in the `Makefile`.

``` bash
make docker-push
```

### Bundle

Create and push the bundle image for to use the operator in OLM as a CatalogSource. 

``` bash
make bundle-build bundle-push
```
To override the name of the bundle image, specify the `BUNDLE_IMG` tag, for example

``` bash
make bundle-build bundle-push BUNDLE_IMG=quay.io/my-org/argocd-operator-bundle:latest
```

### [WIP] Development Process

This is the basic process for development. First, create a branch for the new feature or bug fix.

``` bash
git switch -c MY_BRANCH
```

#### Building and testing locally

To run the operator locally on your machine (outside a container), invoke the following make target:

``` bash
make install run
```

This will install the CRDs into your cluster, then run the operator on your machine.

To run the unit tests, invoke the following make target:

``` bash
make test
```

Run the e2e tests.

Refer E2E test [guide](../e2e-test-guide.md) for the setup and execution.

```bash
k3d cluster create --servers 3
```

#### Building the operator images to test on a cluster

Build the development container image.
Override the name of the image to build by specifying the `IMG` variable.

``` bash
make docker-build IMG=quay.io/my-org/argocd-operator:latest
```

Push the development container image.
Override the name of the image to push by specifying the `IMG` variable.

``` bash
make docker-push IMG=quay.io/my-org/argocd-operator:latest
```

Generate the bundle artifacts.
Override the name of the development image by specifying the `IMG` variable.

``` bash
rm -fr bundle/
make bundle IMG=quay.io/my-org/argocd-operator:latest
```

Build and push the development bundle image.
Override the name of the bundle image by specifying the `BUNDLE_IMG` variable.

``` bash
make bundle-build BUNDLE_IMG=quay.io/my-org/argocd-operator-bundle:latest
make bundle-push BUNDLE_IMG=quay.io/my-org/argocd-operator-bundle:latest
```

Build and push the development catalog image.
Override the name of the catalog image by specifying the `CATALOG_IMG` variable.
Specify the bundle image to include using the `BUNDLE_IMG` variable

``` bash
make catalog-build BUNDLE_IMG=quay.io/my-org/argocd-operator-bundle:latest CATALOG_IMG=quay.io/my-org/argocd-operator-index:latest
make catalog-push CATALOG_IMG=quay.io/my-org/argocd-operator-index:latest
```
For more infomation see [build operator images to test on a cluster.](https://argocd-operator.readthedocs.io/en/latest/developer-guide/development/#building-the-operator-images-to-test-on-a-cluster)

Once the operator is installed, you would need to configure an ArgoCD instance that the operator would manage. The sample instance configuration is below:
``` yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: basic
spec: {}
```
Store your ArgoCD configuration in a yaml file and execute below command to configure the ArgoCD instance.

``` bash
kubectl apply -f <path_to_yaml_file>
```
For configuring specific properties based on the use case, you can look at the entire list of the configurable properties [here.](https://argocd-operator.readthedocs.io/en/latest/reference/argocd/)

### Build and Verify Argo CD Operator Docs

**Note**: Please note that you need to have `Python3` Installed as a prerequisite.

Create a Python Virtual Environment. This is not mandatory, you can continue without creating a Virtual Environment as well.

```bash
python3 -m venv doc
```

Get into the virtual environment, if you have created one using the above step.

```bash
source doc/bin/activate
```

Install the required Python libraries

```bash
pip3 install mkdocs
pip3 install mkdocs-material
```

Start the `mkdocs` server locally to verify the UI changes.

```bash
mkdocs serve
```

### Default Argo CD Version

There are several steps required to update the default version of Argo CD that is installed by the operator.

#### CRDs

The operator bundles and provides the CRDs that are used by Argo CD to ensure that they are present in the cluster.

Update the [CRDs][argocd_upstream_crds] from the upstream Argo CD project in the `config/crd/bases` directory to ensure they match the version of Argo CD that will be used as the default.

[podman_link]:https://podman.io
[argocd_upstream_crds]:https://github.com/argoproj/argo-cd/tree/master/manifests/crds

#### Container Image

Update the constant that contains the hash that corresponds to the version of Argo CD that should be deployed by default. This can be found in the `common/defaults.go` file.

```go
ArgoCDDefaultArgoVersion = "sha256:abc123..."
```
