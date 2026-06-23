## Overview

[Argo CD Notifications](https://blog.argoproj.io/notifications-for-argo-bb7338231604) was merged into core Argo CD codebase as a part of the v2.3 release. The notifications controller is now available as an optional workload that can be configured through the Argo CD operator. 

## Installation

Argo CD Notifications controller can be enabled/disabled using a new toggle within the Argo CD CR with default specs as follows:

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  notifications:
    enabled: True
```

Notifications are disabled by default. Enabling notifications results in the operator creating the following resources on the cluster:

*  `<argocd-instance-name>-notifications-controller` deployment
*  `<argocd-instance-name>-argocd-notifications-controller` serviceAccount
*  `<argocd-instance-name>-argocd-notifications-controller` role
*  `<argocd-instance-name>-argocd-notifications-controller` roleBinding
*  `<argocd-instance-name>-argocd-notifications-cm` configmap
*  `<argocd-instance-name>-argocd-notifications-secret` secret

The operator creates the `argocd-notifications-cm` configmap which is populated with a set of default templates and triggers out of the box, in line with what is provided by the upstream Argo CD project. `argocd-notifications-cm` is editable to users, and will not be reconciled/overwritten by the operator. The `argocd-notifications-secret` is an empty secret that can be used to configure credentials for the supported notifications services.

Instructions for appropriate configuration of these resources can be found within [upstream documentation](https://argo-cd.readthedocs.io/en/stable/operator-manual/notifications/).

## Usage

The `notifications` field in the `ArgoCD` custom resource allows you to configure the notifications controller:

Name | Default | Description
--- | --- | ---
`enabled` | `false` | Whether to enable deployment of the notifications controller.
`replicas` |  | Number of controller replicas.
`sourceNamespaces` |  | List of namespaces where notifications should watch for Argo CD Application resources.
`env` |  | Additional environment variables for the notifications pods (array of `EnvVar`).
`image` |  | Container image for the notifications controller.
`version` |  | The tag/version of the controller image to use.
`resources` |  | Compute resources required by the notifications controller pod (Kubernetes resource requirements).
`logLevel` |  | Log level for the notifications controller (`debug`, `info`, `warn`, `error`).
`logFormat` |  | Log output format (`text` or `json`).
`metrics` |  | Configuration for Prometheus ServiceMonitor.
`metrics.interval` |  | Prometheus scrape interval.
`metrics.scrapeTimeout` |  | Prometheus scrape timeout.

Example configuration snippet:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  notifications:
    enabled: true
    sourceNamespaces:
      - my-team
      - dev
    image: quay.io/acme-corp/argocd-notification
    version: stable
    resources:
      limits:
        cpu: 100m
        memory: 256Mi
      requests:
        cpu: 50m
        memory: 128Mi
    logLevel: info
    logFormat: json
    metrics:
      interval: 30s
      scrapeTimeout: 10s
```

For most users, enabling `notifications` with default options is sufficient, but these fields provide flexibility for advanced deployments.


## Notifications in Any Namespace

By default, Argo CD Notifications uses a centralized configuration model where all notification settings are managed in the main Argo CD namespace (typically `argocd`). The operator supports delegating notification configuration to specific namespaces, allowing teams to manage their own notification settings.

For detailed information on enabling and using notifications in any namespace, see the [Notifications in Any Namespace](./notifications-in-any-namespace.md) documentation.

## Uninstallation

Argo CD Notifications controller can be disabled by setting `.spec.notifications.enabled` to `false` :

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  notifications:
    enabled: false
```
This will clean up all the aforementioned notifications-controller resources that were created by the operator.
