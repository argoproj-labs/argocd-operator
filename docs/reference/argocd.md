# ArgoCD

The `ArgoCD` resource is a Kubernetes Custom Resource (CRD) that describes the desired state for a given Argo CD
cluster and allows for the configuration of the components that make up an Argo CD cluster.

When the Argo CD Operator sees a new ArgoCD resource, the components are provisioned using Kubernetes resources and
managed by the operator. When something changes on an existing ArgoCD resource, the operator works to reconfigure the
cluster to ensure the actual state of the cluster matches the desired state.

The ArgoCD Custom Resource consists of the following properties.

Name | Default | Description
--- | --- | ---
[**ApplicationInstanceLabelKey**](#application-instance-label-key) | `mycompany.com/appname` |  The metadata.label key name where Argo CD injects the app name as a tracking label.
[**ConfigManagementPlugins**](#config-management-plugins) | [Empty] | Configuration to add a config management plugin.
[**Controller**](#controller-options) | [Object] | Argo CD Application Controller options.
[**Dex**](#dex-options) | [Object] | Dex configuration options.
[**GATrackingID**](#ga-tracking-id) | [Empty] | The google analytics tracking ID to use.
[**GAAnonymizeUsers**](#ga-anonymize-users) | `false` | Enable hashed usernames sent to google analytics.
[**Grafana**](#grafana-options) | [Object] | Grafana configuration options.
[**HA**](#ha-options) | [Object] | High Availability options.
[**HelpChatURL**](#help-chat-url) | `https://mycorp.slack.com/argo-cd` | URL for getting chat help, this will typically be your Slack channel for support.
[**HelpChatText**](#help-chat-text) | `Chat now!` | The text for getting chat help.
[**Image**](#image) | `argoproj/argocd` | The container image for all Argo CD components.
[**Import**](#import-options) | [Object] | Import configuration options.
[**Ingress**](#ingress-options) | [Object] | Ingress configuration options.
[**InitialRepositories**](#initial-repositories) | [Empty] | Initial git repositories to configure Argo CD to use upon creation of the cluster.
[**InitialSSHKnownHosts**](#initial-ssh-known-hosts) | [Default Argo CD Known Hosts] | Initial SSH Known Hosts for Argo CD to use upon creation of the cluster.
[**KustomizeBuildOptions**](#kustomize-build-options) | [Empty] | The build options/parameters to use with `kustomize build`.
[**OIDCConfig**](#oidc-config) | [Empty] | The OIDC configuration as an alternative to Dex.
[**Prometheus**](#prometheus-options) | [Object] | Prometheus configuration options.
[**RBAC**](#rbac-options) | [Object] | RBAC configuration options.
[**Redis**](#redis-options) | [Object] | Redis configuration options.
[**ResourceCustomizations**](#resource-customizations) | [Empty] | Customize resource behavior.
[**ResourceExclusions**](#resource-exclusions) | [Empty] | The configuration to completely ignore entire classes of resource group/kinds.
[**Server**](#server-options) | [Object] | Argo CD Server configuration options.
[**StatusBadgeEnabled**](#status-badge-enabled) | `true` | Enable application status badge feature.
[**TLS**](#tls-options) | [Object] | TLS configuration options.
[**UsersAnonymousEnabled**](#users-anonymous-enabled) | `true` | Enable anonymous user access.
[**Version**](#version) | v1.5.5 (SHA) | The tag to use with the container image for all Argo CD components.

## Application Instance Label Key

The metadata.label key name where Argo CD injects the app name as a tracking label (optional). Tracking labels are used to determine which resources need to be deleted when pruning. If omitted, Argo CD injects the app name into the label: 'app.kubernetes.io/instance'

This property maps directly to the `application.instanceLabelKey` field in the `argocd-cm` ConfigMap.

### Application Instance Label Key Example

The following example sets the default value in the `argocd-cm` ConfigMap using the `ApplicationInstanceLabelKey` property on the `ArgoCD` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: application-instance-label-key
spec:
  applicationInstanceLabelKey: mycompany.com/appname
```

## Config Management Plugins

Configuration to add a config management plugin. This property maps directly to the `configManagementPlugins` field in the `argocd-cm` ConfigMap.

### Config Management Plugins Example

The following example sets a value in the `argocd-cm` ConfigMap using the `ConfigManagementPlugins` property on the `ArgoCD` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: config-management-plugins
spec:
  configManagementPlugins: |
    - name: kasane
      init:
        command: [kasane, update]
      generate:
        command: [kasane, show]
```

## Controller Options

The following properties are available for configuring the Argo CD Application Controller component.

Name | Default | Description
--- | --- | ---
Processors.Operation | 10 | The number of operation processors.
Processors.Status | 20 | The number of status processors.
Resources | [Empty] | The container compute resources.

### Controller Example

The following example shows all properties set to the default values.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: controller
spec:
  controller:
    processors:
      operation: 10
      status: 20
    resources: {}
```

## Dex Options

The following properties are available for configuring the Dex component.

Name | Default | Description
--- | --- | ---
Config | [Empty] | The `dex.config` property in the `argocd-cm` ConfigMap.
Image | `quay.io/dexidp/dex` | The container image for Dex.
OpenShiftOAuth | false | Enable automatic configuration of OpenShift OAuth authentication for the Dex server. This is ignored if a value is presnt for `Dex.Config`.
Resources | [Empty] | The container compute resources.
Version | v2.21.0 (SHA) | The tag to use with the Dex container image.

### Dex Example

The following example shows all properties set to the default values.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: dex
spec:
  dex:
    config: ""
    image: quay.io/dexidp/dex
    openShiftOAuth: false
    resources: {}
    version: v2.21.0
```

### Dex OpenShift OAuth Example

The following example configures Dex to use the OAuth server built into OpenShift.

The `OpenShiftOAuth` property can be used to trigger the operator to auto configure the built-in OpenShift OAuth server. The RBAC `Policy` property is used to give the admin role in the Argo CD cluster to users in the OpenShift `cluster-admins` group.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: openshift-oauth
spec:
  dex:
    image: quay.io/redhat-cop/dex
    openShiftOAuth: true
    version: v2.22.0-openshift
  rbac:
    defaultPolicy: 'role:readonly'
    policy: |
      g, system:cluster-admins, role:admin
    scopes: '[groups]'
```

## GA Tracking ID

The google analytics tracking ID to use. This property maps directly to the `ga.trackingid` field in the `argocd-cm` ConfigMap.

### GA Tracking ID Example

The following example sets a value in the `argocd-cm` ConfigMap using the `GATrackingID` property on the `ArgoCD` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: ga-tracking-id
spec:
  gaTrackingID: UA-12345-1
```

## GA Anonymize Users

Enable hashed usernames sent to google analytics. This property maps directly to the `ga.anonymizeusers` field in the `argocd-cm` ConfigMap.

### GA Anonymize Users Example

The following example sets a value in the `argocd-cm` ConfigMap using the `GAAnonymizeUsers` property on the `ArgoCD` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: ga-anonymize-users
spec:
  gaAnonymizeUsers: true
```

## Grafana Options

The following properties are available for configuring the Grafana component.

Name | Default | Description
--- | --- | ---
Enabled | false | Toggle Grafana support globally for ArgoCD.
Host | `example-argocd-grafana` | The hostname to use for Ingress/Route resources.
Image | `grafana/grafana` | The container image for Grafana.
[Ingress](#grafana-ingress-options) | [Object] | Ingress configuration for Grafana.
Resources | [Empty] | The container compute resources.
[Route](#grafana-route-options) | [Object] | Route configuration options.
Size | 1 | The replica count for the Grafana Deployment.
Version | 6.7.1 (SHA) | The tag to use with the Grafana container image.

### Grafana Ingress Options

The following properties are available for configuring the Grafana Ingress.

Name | Default | Description
--- | --- | ---
Annotations | [Empty] | The map of annotations to use for the Ingress resource.
Enabled | `false` | Toggle creation of an Ingress resource.
Path | `/` | Path to use for Ingress resources.
TLS | [Empty] | TLS configuration for the Ingress.

### Grafana Route Options

The following properties are available to configure the Route for the Grafana component.

Name | Default | Description
--- | --- | ---
Annotations | [Empty] | The map of annotations to add to the Route.
Enabled | `false` | Toggles the creation of a Route for the Grafana component.
Path | `/` | The path for the Route.
TLS | [Object] | The TLSConfig for the Route.
WildcardPolicy| `None` | The wildcard policy for the Route. Can be one of `Subdomain` or `None`.

### Grafana Example

The following example shows all properties set to the default values.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: insights
spec:
  grafana:
    enabled: false
    host: example-argocd-grafana
    image: grafana/grafana
    ingress:
      enabled: false
    resources: {}
    route: false
    size: 1
    version: 6.7.1
```

## HA Options

The following properties are available for configuring High Availability for the Argo CD cluster.

Name | Default | Description
--- | --- | ---
Enabled | false | Toggle High Availability support globally for Argo CD.

### HA Example

The following example shows how to enable HA mode globally.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: ha
spec:
  ha:
    enabled: true
```

## Help Chat URL

URL for getting chat help, this will typically be your Slack channel for support. This property maps directly to the `help.chatUrl` field in the `argocd-cm` ConfigMap.

### Help Chat URL Example

The following example sets the default value in the `argocd-cm` ConfigMap using the `HelpChatURL` property on the `ArgoCD` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: help-chat-url
spec:
  helpChatURL: https://mycorp.slack.com/argo-cd
```

## Help Chat Text

The text for getting chat help. This property maps directly to the `help.chatText` field in the `argocd-cm` ConfigMap.

### Help Chat Text Example

The following example sets the default value in the `argocd-cm` ConfigMap using the `HelpChatText` property on the `ArgoCD` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: help-chat-text
spec:
  helpChatText: "Chat now!"
```

## Image

The container image for all Argo CD components.

### Image Example

The following example sets the default value using the `Image` property on the `ArgoCD` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: image
spec:
  image: argoproj/argocd
```

## Import Options

The `Import` property allows for the import of an existing `ArgoCDExport` resource. An ArgoCDExport object represents an Argo CD cluster at a point in time that was exported using the `argocd-util` export capability.

The following properties are available for configuring the import process.

Name | Default | Description
--- | --- | ---
Name | [Empty] | The name of an ArgoCDExport from which to import data.
Namespace | [ArgoCD Namepspace] |  The Namespace for the ArgoCDExport, defaults to the same namespace as the ArgoCD.

### Import Example

The following example shows the use of the `Import` properties to specify the name of an existing `ArgoCDExport` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: import
spec:
  import:
    name: example-argocdexport
    namespace: argocd
```

When `Import` properties are specified on the `ArgoCD` resource, the operator will create an init-container on the
Argo CD Application Controller Pod that will use the built-in Argo CD import command to create the resources defined
in an export YAML file that was generated by the referenced `ArgoCDExport` resource.

To aid in troubleshooting, view the logs from the init-container. Output similar to what is show below indicates a
successful import.

``` bash
importing argo-cd
decrypting argo-cd backup
loading argo-cd backup
/ConfigMap argocd-cm updated
/ConfigMap argocd-rbac-cm updated
/ConfigMap argocd-ssh-known-hosts-cm updated
/ConfigMap argocd-tls-certs-cm updated
/Secret argocd-secret updated
argoproj.io/AppProject default unchanged
argo-cd import complete
```

## Initial Repositories

Initial git repositories to configure Argo CD to use upon creation of the cluster.

This property maps directly to the `repositories` field in the `argocd-cm` ConfigMap. Updating this property after the cluster has been created has no affect and should be used only as a means to initialize the cluster with the value provided. Modifications to the `repositories` field should then be made through the Argo CD web UI or CLI.

### Initial Repositories Example

The following example sets a value in the `argocd-cm` ConfigMap using the `InitialRepositories` property on the `ArgoCD` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: initial-repositories
spec:
  initialRepositories: |
    - url: https://github.com/argoproj/my-private-repository
      passwordSecret:
        name: my-secret
        key: password
      usernameSecret:
        name: my-secret
        key: username
      sshPrivateKeySecret:
        name: my-secret
        key: sshPrivateKey
    - type: helm
      url: https://storage.googleapis.com/istio-prerelease/daily-build/master-latest-daily/charts
      name: istio.io
    - type: helm
      url: https://my-private-chart-repo.internal
      name: private-repo
      usernameSecret:
        name: my-secret
        key: username
      passwordSecret:
        name: my-secret
        key: password
    - type: git
      url: https://github.com/argoproj/argocd-example-apps.git
```

## Initial SSH Known Hosts

Initial SSH Known Hosts for Argo CD to use upon creation of the cluster.

This property maps directly to the `ssh_known_hosts` field in the `argocd-ssh-known-hosts-cm` ConfigMap. Updating this property after the cluster has been created has no affect and should be used only as a means to initialize the cluster with the value provided. Modifications to the `ssh_known_hosts` field should then be made through the Argo CD web UI or CLI.

### Initial SSH Known Hosts Example

The following example sets a value in the `argocd-ssh-known-hosts-cm` ConfigMap using the `InitialSSHKnownHosts` property on the `ArgoCD` resource. The example values have been truncated for clarity.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: initial-ssh-known-hosts
spec:
  initialSSHKnownHosts: |
    bitbucket.org ssh-rsa AAAAB3NzaC...
    github.com ssh-rsa AAAAB3NzaC...
```

## Kustomize Build Options

Build options/parameters to use with `kustomize build` (optional). This property maps directly to the `kustomize.buildOptions` field in the `argocd-cm` ConfigMap.

### Kustomize Build Options Example

The following example sets a value in the `argocd-cm` ConfigMap using the `KustomizeBuildOptions` property on the `ArgoCD` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: kustomize-build-options
spec:
  kustomizeBuildOptions: --load_restrictor none
```

## OIDC Config

OIDC configuration as an alternative to dex (optional). This property maps directly to the `oidc.config` field in the `argocd-cm` ConfigMap.

### OIDC Config Example

The following example sets a value in the `argocd-cm` ConfigMap using the `KustomizeBuildOptions` property on the `ArgoCD` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: oidc-config
spec:
  oidcConfig: |
    name: Okta
    issuer: https://dev-123456.oktapreview.com
    clientID: aaaabbbbccccddddeee
    clientSecret: $oidc.okta.clientSecret
    # Optional set of OIDC scopes to request. If omitted, defaults to: ["openid", "profile", "email", "groups"]
    requestedScopes: ["openid", "profile", "email"]
    # Optional set of OIDC claims to request on the ID token.
    requestedIDTokenClaims: {"groups": {"essential": true}}
```

## Prometheus Options

The following properties are available for configuring the Prometheus component.

Name | Default | Description
--- | --- | ---
Enabled | false | Toggle Prometheus support globally for ArgoCD.
Host | `example-argocd-prometheus` | The hostname to use for Ingress/Route resources.
Ingress | `false` | Toggles Ingress for Prometheus.
[Route](#prometheus-route-options) | [Object] | Route configuration options.
Size | 1 | The replica count for the Prometheus StatefulSet.

### Prometheus Ingress Options

The following properties are available for configuring the Prometheus Ingress.

Name | Default | Description
--- | --- | ---
Annotations | [Empty] | The map of annotations to use for the Ingress resource.
Enabled | `false` | Toggle creation of an Ingress resource.
Path | `/` | Path to use for Ingress resources.
TLS | [Empty] | TLS configuration for the Ingress.

### Prometheus Route Options

The following properties are available to configure the Route for the Prometheus component.

Name | Default | Description
--- | --- | ---
Annotations | [Empty] | The map of annotations to add to the Route.
Enabled | `false` | Toggles the creation of a Route for the Prometheus component.
Path | `/` | The path for the Route.
TLS | [Object] | The TLSConfig for the Route.
WildcardPolicy| `None` | The wildcard policy for the Route. Can be one of `Subdomain` or `None`.

### Prometheus Example

The following example shows all properties set to the default values.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: insights
spec:
  prometheus:
    enabled: false
    host: example-argocd-prometheus
    ingress:
      enabled: false
    route: false
    size: 1
```

## RBAC Options

The following properties are available for configuring RBAC for the Argo CD cluster.

Name | Default | Description
--- | --- | ---
DefaultPolicy | `role:readonly` | The `policy.default` property in the `argocd-rbac-cm` ConfigMap. The name of the default role which Argo CD will falls back to, when authorizing API requests.
Policy | [Empty] | The `policy.csv` property in the `argocd-rbac-cm` ConfigMap. CSV data containing user-defined RBAC policies and role definitions.
Scopes | `[groups]` | The `scopes` property in the `argocd-rbac-cm` ConfigMap.  Controls which OIDC scopes to examine during rbac enforcement (in addition to `sub` scope).

### RBAC Example

The following example shows all properties set to the default values.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: rbac
spec:
  rbac:
    defaultPolicy: 'role:readonly'
    policy: |
      g, system:cluster-admins, role:admin
    scopes: '[groups]'
```

## Redis Options

The following properties are available for configuring the Redis component.

Name | Default | Description
--- | --- | ---
Image | `redis` | The container image for Redis.
Resources | [Empty] | The container compute resources.
Version | 5.0.3 (SHA) | The tag to use with the Redis container image.

### Redis Example

The following example shows all properties set to the default values.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: redis
spec:
  redis:
    image: redis
    resources: {}
    version: "5.0.3"
```

## Repo Options

The following properties are available for configuring the Repo server component.

Name | Default | Description
--- | --- | ---
Resources | [Empty] | The container compute resources.
MountSAToken | false | Whether the ServiceAccount token should be mounted to the repo-server pod.
ServiceAccount | "" | The name of the ServiceAccount to use with the repo-server pod.

### Repo Example

The following example shows all properties set to the default values.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: repo
spec:
  repo:
    resources: {}
    mountsatoken: false
    serviceaccount: ""
```

## Resource Customizations

The configuration to customize resource behavior. This property maps directly to the `resource.customizations` field in the `argocd-cm` ConfigMap.

### Resource Customizations Example

The following example defines a custom PV health check in the `argocd-cm` ConfigMap using the `ResourceCustomizations` property on the `ArgoCD` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: resource-customizations
spec:
  resourceCustomizations: |
    PersistentVolumeClaim:
      health.lua: |
        hs = {}
        if obj.status ~= nil then
          if obj.status.phase ~= nil then
            if obj.status.phase == "Pending" then
              hs.status = "Healthy"
              hs.message = obj.status.phase
              return hs
            end
            if obj.status.phase == "Bound" then
              hs.status = "Healthy"
              hs.message = obj.status.phase
              return hs
            end
          end
        end
        hs.status = "Progressing"
        hs.message = "Waiting for certificate"
        return hs
```

## Resource Exclusions

Configuration to completely ignore entire classes of resource group/kinds (optional).
Excluding high-volume resources improves performance and memory usage, and reduces load and bandwidth to the Kubernetes API server.

These are globs, so a "*" will match all values. If you omit groups/kinds/clusters then they will match all groups/kind/clusters.

NOTE: events.k8s.io and metrics.k8s.io are excluded by default.

This property maps directly to the `resource.exclusions` field in the `argocd-cm` ConfigMap.

### Resource Exclusions Example

The following example sets a value in the `argocd-cm` ConfigMap using the `ResourceExclusions` property on the `ArgoCD` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: resource-exclusions
spec:
  resourceExclusions: |
    - apiGroups:
      - repositories.stash.appscode.com
      kinds:
      - Snapshot
      clusters:
      - "*.local"
```

## Server Options

The following properties are available for configuring the Argo CD Server component.

Name | Default | Description
--- | --- | ---
[Autoscale](#server-autoscale-options) | [Object] | Server autoscale configuration options.
[GRPC](#server-grpc-options) | [Object] | GRPC configuration options.
Host | example-argocd | The hostname to use for Ingress/Route resources.
[Ingress](#server-ingress-options) | [Object] | Ingress configuration for the Argo CD Server component.
Insecure | false | Toggles the insecure flag for Argo CD Server.
Resources | [Empty] | The container compute resources.
[Route](#server-route-options) | [Object] | Route configuration options.
Service.Type | ClusterIP | The ServiceType to use for the Service resource.

### Server Autoscale Options

The following properties are available to configure austoscaling for the Argo CD Server component.

Name | Default | Description
--- | --- | ---
Enabled | false | Toggle Autoscaling support globally for the Argo CD server component.
HPA | [Object] | HorizontalPodAutoscaler options for the Argo CD Server component.

### Server GRPC Options

The following properties are available to configure GRPC for the Argo CD Server component.

Name | Default | Description
--- | --- | ---
Host | `example-argocd-grpc` | The hostname to use for Ingress GRPC resources.
[Ingress](#server-grpc-ingress-options) | [Object] | Ingress configuration for the Argo CD GRPC Server component.

### Server GRPC Ingress Options

The following properties are available for configuring the Argo CD server GRP Ingress.

Name | Default | Description
--- | --- | ---
Annotations | [Empty] | The map of annotations to use for the Ingress resource.
Enabled | `false` | Toggle creation of an Ingress resource.
Path | `/` | Path to use for Ingress resources.
TLS | [Empty] | TLS configuration for the Ingress.

### Server Ingress Options

The following properties are available for configuring the Argo CD server Ingress.

Name | Default | Description
--- | --- | ---
Annotations | [Empty] | The map of annotations to use for the Ingress resource.
Enabled | `false` | Toggle creation of an Ingress resource.
Path | `/` | Path to use for Ingress resources.
TLS | [Empty] | TLS configuration for the Ingress.

### Server Route Options

The following properties are available to configure the Route for the Argo CD Server component.

Name | Default | Description
--- | --- | ---
Annotations | [Empty] | The map of annotations to add to the Route.
Enabled | `false` | Toggles the creation of a Route for the Argo CD Server component.
Path | `/` | The path for the Route.
TLS | [Object] | The TLSConfig for the Route.
WildcardPolicy| `None` | The wildcard policy for the Route. Can be one of `Subdomain` or `None`.

### Server Example

The following example shows all properties set to the default values.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: server
spec:
  server:
    autoscale:
      enabled: false
      hpa:
        maxReplicas: 3
        minReplicas: 1
        scaleTargetRef:
          apiVersion: extensions/v1beta1
          kind: Deployment
          name: example-argocd-server
        targetCPUUtilizationPercentage: 50
    grpc:
      host: example-argocd-grpc
      ingress: false
    host: example-argocd
    ingress:
      enabled: false
    insecure: false
    resources: {}
    route:
      annotations: {}
      enabled: false
      path: /
      TLS:
        insecureEdgeTerminationPolicy: Redirect
        termination: passthrough
      wildcardPolicy: None
    service:
      type: ClusterIP
```

## Status Badge Enabled

Enable application status badge feature. This property maps directly to the `statusbadge.enabled` field in the `argocd-cm` ConfigMap.

### Status Badge Enabled Example

The following example sets the default value in the `argocd-cm` ConfigMap using the `StatusBadgeEnabled` property on the `ArgoCD` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: status-badge-enabled
spec:
  statusBadgeEnabled: true
```

## TLS Options

The following properties are available for configuring the Grafana component.

Name | Default | Description
--- | --- | ---
CA.ConfigMapName | `example-argocd-ca` | The name of the ConfigMap containing the CA Certificate.
CA.SecretName | `example-argocd-ca` | The name of the Secret containing the CA Certificate and Key.
InitialCerts | [Empty] | Initial set of certificates in the `argocd-tls-certs-cm` ConfigMap for connecting Git repositories via HTTPS.

### TLS Example

The following example shows all properties set to the default values.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: tls
spec:
  tls:
    ca:
      configMapName: example-argocd-ca
      secretName: example-argocd-ca
    initialCerts: []
```

## Users Anonymous Enabled

Enables anonymous user access. The anonymous users get default role permissions specified `argocd-rbac-cm`.

This property maps directly to the `users.anonymous.enabled` field in the `argocd-cm` ConfigMap.

### Users Anonymous Enabled Example

The following example sets the default value in the `argocd-cm` ConfigMap using the `UsersAnonymousEnabled` property on the `ArgoCD` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: users-anonymous-enabled
spec:
  usersAnonymousEnabled: false
```

## Version

The tag to use with the container image for all Argo CD components.

### Version Example

The following example sets the default value using the `Version` property on the `ArgoCD` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: version
spec:
  version: v1.5.4
```
