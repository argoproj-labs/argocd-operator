
# Development

### Requirements

The requirements for building the operator are fairly minimal.

 * Go 1.12+
 * Operator SDK 0.10+

### Building from Source

Ensure Go module support is enabled in your environment.

```bash
export GO111MODULE=on

```

Run the build subcommand that is part of the Operator SDK to build the operator.

```bash
operator-sdk build <YOUR_IMAGE_REPO>/argocd-operator:latest
```

Push the image to a container registry for deployment.

```bash
docker push <YOUR_IMAGE_REPO>/argocd-operator:latest
```
