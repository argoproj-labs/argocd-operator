# Custom Tooling

See [upstream documentation](https://argo-cd.readthedocs.io/en/stable/operator-manual/custom_tools/) for more information

## Adding Tools Via Volume Mounts

Both init containers and volumes can be added to the repo server using the `ArgoCD` custom resource

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd-sample
spec:
  repo:
    # 1. Define an emptyDir volume which will hold the custom binaries
    volumes:
    - name: custom-tools
      emptyDir: {}
    # 2. Use an init container to download/copy custom binaries into the emptyDir
    initContainers:
    - name: download-tools
      image: alpine:3.8
      command: [sh, -c]
      args:
      - wget -qO- https://storage.googleapis.com/kubernetes-helm/helm-v2.12.3-linux-amd64.tar.gz | tar -xvzf - &&
        mv linux-amd64/helm /custom-tools/
      volumeMounts:
      - mountPath: /custom-tools
        name: custom-tools
    # 3. Volume mount the custom binary to the bin directory (overriding the existing version)
    volumeMounts:
    - mountPath: /usr/local/bin/helm
      name: custom-tools
      subPath: helm
```
