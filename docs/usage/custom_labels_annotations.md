# Custom Annotations and Labels

You can add labels and annotations to the pods of the server, repo, application set controller, application controller, dex, and redis.

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  server:
    labels:
      custom: label
      custom2: server
    annotations:
      custom: annotation
      custom2: server
  repo:
    labels:
      custom: label
      custom2: repo
    annotations:
      custom: annotation
      custom2: repo
  controller:
    labels:
      custom: label
      custom2: controller
    annotations:
      custom: annotation
      custom2: controller
  applicationSet:
    labels:
      custom: label
      custom2: applicationSet
    annotations:
      custom: annotation
      custom2: applicationSet
  redis:
    labels:
      custom: label
      custom2: redis
    annotations:
      custom: annotation
      custom2: redis
  sso:
    provider: dex
    dex:
      labels:
        custom: label
        custom2: dex
      annotations:
        custom: annotation
        custom2: dex
```