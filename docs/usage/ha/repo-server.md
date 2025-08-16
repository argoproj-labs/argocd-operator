# High Availability for Repo Server

The Argo CD operator supports scaling up the repo server to ensure high availability. This can be achieved using the mechanism described in the Argo CD [documentation][argocd_repo_scaling].

## Persistent Volumes for Repo Server Storage

> `argocd-repo-server` clones the repository into `/tmp` (or the path specified in the `TMPDIR` env variable). The Pod might run out of disk space if it has too many repositories or if the repositories have a lot of files. To avoid this problem mount a persistent volume.

To mount a Persistent Volume to the Repo Server, add the volume details in the `volumes` and `volumeMounts` fields under `.spec.repo` in the `ArgoCD` custom resource (CR).

!!! important
      The Argo CD operator does not create persistent volumes automatically. It is the user's responsibility to create and manage the required Persistent Volume (PV) and Persistent Volume Claim (PVC) resources. Make sure to provision these resources according to your storage needs and environment.

### Examples

Mount the persistent volume to the default repository storage path, i.e. `/tmp`:
```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd-sample
spec:
  repo:
    volumes:
    - name: repo-storage
      persistentVolumeClaim:
        claimName: <persistent-volume-claim>
    volumeMounts:
    - mountPath: /tmp
      name: repo-storage
```
Alternatively, mount the persistent volume to a custom repository storage path, e.g. `/manifests`.
```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd-sample
spec:
  repo:
    volumes:
    - name: repo-storage
      persistentVolumeClaim:
        claimName: <persistent-volume-claim>
    volumeMounts:
    - mountPath: /manifests
      name: repo-storage
    env:
    - name: TMPDIR
      value: "/manifests"
```

[argocd_repo_scaling]:https://argo-cd.readthedocs.io/en/stable/operator-manual/high_availability/#scaling-up
