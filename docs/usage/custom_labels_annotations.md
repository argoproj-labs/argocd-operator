# Custom Annotations and Labels

You can easily add labels and annotations to the pods of the server, repo, application set controller, and application controller.

```[yaml]
spec:
  server:
    customPodLabels:
      custom: label
      custom2: server
    customPodAnnotations:
      custom: annotation
      custom2: server
  repo:
    customPodLabels:
      custom: label
      custom2: repo
    customPodAnnotations:
      custom: annotation
      custom2: repo
  controller:
    customPodLabels:
      custom: label
      custom2: controller
    customPodAnnotations:
      custom: annotation
      custom2: controller
  applicationSet:
    customPodLabels:
      custom: label
      custom2: applicationSet
    customPodAnnotations:
      custom: annotation
      custom2: applicationSet
```