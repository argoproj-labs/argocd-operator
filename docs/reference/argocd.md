# ArgoCD

The `ArgoCD` resource is a Kubernetes Custom Resource (CRD) that describes the desired state for a given Argo CD 
cluster and allows for the configuration of the components that make up an Argo CD cluster.

When the Argo CD Operator sees a new ArgoCD resource, the components are provisioned using Kubernetes resources and 
managed by the operator. When something changes on an existing ArgoCD resource, the operator works to reconfigure the 
cluster to ensure the actual state of the cluster matches the desired state. 

The ArgoCD Custom Resource consists of the following properties.

Name | Default | Description
--- | --- | ---
**ApplicationInstanceLabelKey** | `mycompany.com/appname` |  The `application.instanceLabelKey` property in the `argocd-cm` ConfigMap. The metadata.label key name where Argo CD injects the app name as a tracking label (optional).
**ConfigManagementPlugins** | [Empty] | The `configManagementPlugins` property in the `argocd-cm` ConfigMap. Configuration to add a config management plugin.
**Controller** | [Object](#controller-options) | Argo CD Application Controller options. See [below](#controller-options) for more detail.
**Dex** | [Object](#dex-options) | Dex options. See [below](#dex-options) for more detail.
**GATrackingID** | [Empty] | The `ga.trackingid` property in the `argocd-cm` ConfigMap. The google analytics tracking ID to use.
**GAAnonymizeUsers** | false | The `ga.anonymizeusers` property in the `argocd-cm` ConfigMap. Enable hashed usernames sent to google analytics.
**Grafana** | [Object](#grafana-options) | Grafana options. See [below](#grafana-options) for more detail.
**HA** | [Object]#ha-options) | High Availability options. See [below](#ha-options) for more detail.
**HelpChatURL** | `https://mycorp.slack.com/argo-cd` | The `help.chatUrl` property in the `argocd-cm` ConfigMap. URL for getting chat help, this will typically be your Slack channel for support.
**HelpChatText** | `Chat now!` | The `help.chatText` property in the `argocd-cm` ConfigMap. The text for getting chat help.
**Image** | `argoproj/argocd` | The container image for all Argo CD components.
**Import** | [Object](#import-options) | Import options. See [below](#import-options) for more detail.
**Ingress** | [Object](#ingress-options) | Ingress options. See [below](#ingress-options) for more detail.
**KustomizeBuildOptions** | [Empty] | The `kustomize.buildOptions` property in the `argocd-cm` ConfigMap. The build options/parameters to use with `kustomize build`.
**OIDCConfig** | [Empty] | The `oidc.config` property in the `argocd-cm` ConfigMap. The OIDC configuration as an alternative to Dex.
**Prometheus** | [Object](#prometheus-options) | Prometheus options. See [below](#prometheus-options) for more detail.
**RBAC** | [Object](#rbac-options) | RBAC options. See [below](#rbac-options) for more detail. 
**Redis** | [Object](#redis-options) | Redis options. See [below](#redis-options) for more detail.
**Repositories** | [Empty] | The `repositories` property in the `argocd-cm` ConfigMap. Git repositories configure Argo CD with initially.
**ResourceCustomizations** | [Empty] | The `resource.customizations` property in the `argocd-cm` ConfigMap. The configuration to customize resource behavior.
**ResourceExclusions** | [Empty] | The `resource.exclusions` property in the `argocd-cm` ConfigMap. The configuration to completely ignore entire classes of resource group/kinds
**Server** | [Object](#server-options) | Argo CD Server options. See [below](#server-options) for more detail.
**SSHKnownHosts** | Default Argo CD Known Hosts | The `ssh_known_hosts` property in the `argocd-ssh-known-hosts-cm` ConfigMap. 
**StatusBadgeEnabled** | true | The `statusbadge.enabled` property in the `argocd-cm` ConfigMap. Enable application status badge feature.
**TLS** | [Object](#tls-options) | TLS options. See [below](#tls-options) for more detail.
**UsersAnonymousEnabled** | true | The `users.anonymous.enabled` property in the `argocd-cm` ConfigMap. Enable anonymous user access.
**Version** | v1.4.2 (SHA) | The tag to use with the container image for all Argo CD components.

## Controller Options

The following properties are available for configuring the Argo CD Application Controller component. 

Name | Default | Description
--- | --- | ---
Processors.Operation | 10 | The number of operation processors.
Processors.Status | 20 | The number of status processors.
Resources | [Empty] | The container compute resources.

### Controller Example

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
      operation: 15
      status: 25
    resources:
      requests:
        memory: "64Mi"
        cpu: "500m"
      limits:
        memory: "128Mi"
        cpu: "2000m"
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

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: dex
spec:
  dex:
    image: quay.io/ablock/dex
    openShiftOAuth: true
    resources:
      requests:
        memory: "32Mi"
        cpu: "250m"
      limits:
        memory: "64Mi"
        cpu: "1000m"
    version: openshift-connector
```

## Grafana Options

The following properties are available for configuring the Grafana component.

Name | Default | Description
--- | --- | ---
Enabled | false | Toggle Grafana support globally for ArgoCD.
Host | `example-argocd-grafana` | The hostname to use for Ingress/Route resources.
Image | `grafana/grafana` | The container image for Grafana.
Resources | [Empty] | The container compute resources.
Size | 1 | The replica count for the Grafana Deployment.
Version | 6.7.1 (SHA) | The tag to use with the Grafana container image.

### Grafana Example

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: insights
spec:
  grafana:
    enabled: true
    host: grafana.example.com
    resources:
      requests:
        memory: "32Mi"
        cpu: "250m"
      limits:
        memory: "64Mi"
        cpu: "1000m"
    size: 3
```

## HA Options

The following properties are available for configuring High Availability for the Argo CD cluster.

Name | Default | Description
--- | --- | ---
Enabled | false | Toggle High Availability support globally for Argo CD.

### HA Example

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

## Import Options

The `Import` property allows for the import of an existing `ArgoCDExport` resource. An ArgoCDExport object represents an Argo CD cluster at a point in time that was exported using the `argocd-util` export capability.

The following properties are available for configuring the import process.

Name | Default | Description
--- | --- | ---
Name | [Empty] | The name of an ArgoCDExport from which to import data.
Namespace | [ArgoCD Namepspace] |  The Namespace for the ArgoCDExport, defaults to the same namespace as the ArgoCD.

## Ingress Options

The following properties are available for configuring the Ingress for the cluster.

Name | Default | Description
--- | --- | ---
Annotations | [Empty] | The map of annotations to use for the Ingress resource.
Enabled | false | Toggle Ingress support globally for ArgoCD.
Path | / | Path to use for the Ingress resource.

### Ingress Example

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: ingress
spec:
  ingress:
    annotations:
      kubernetes.io/ingress.class: nginx
      nginx.ingress.kubernetes.io/rewrite-target: /static/$2
      cert-manager.io/cluster-issuer: letsencrypt
    enabled: true
    path: /testpath
  server:
    insecure: true
```

## Prometheus Options

The following properties are available for configuring the Prometheus component.

Name | Default | Description
--- | --- | ---
Enabled | false | Toggle Prometheus support globally for ArgoCD.
Host | `example-argocd-prometheus` | The hostname to use for Ingress/Route resources.
Size | 1 | The replica count for the Prometheus StatefulSet.

### Prometheus Example

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: insights
spec:
  prometheus:
    enabled: true
    host: prometheus.example.com
    size: 2
```

## RBAC Options

The following properties are available for configuring RBAC for the Argo CD cluster.

Name | Default | Description
--- | --- | ---
DefaultPolicy | `role:readonly` | The `policy.default` property in the `argocd-rbac-cm` ConfigMap. The name of the default role which Argo CD will falls back to, when authorizing API requests.
Policy | [Empty] | The `policy.csv` property in the `argocd-rbac-cm` ConfigMap. CSV data containing user-defined RBAC policies and role definitions.
Scopes | `[groups]` | The `scopes` property in the `argocd-rbac-cm` ConfigMap.  Controls which OIDC scopes to examine during rbac enforcement (in addition to `sub` scope).

## Redis Options

The following properties are available for configuring the Redis component.

Name | Default | Description
--- | --- | ---
Image | `redis` | The container image for Redis.
Resources | [Empty] | The container compute resources.
Version | 5.0.3 (SHA) | The tag to use with the Redis container image.

### Redis Example

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: redis
spec:
  redis:
    resources:
      requests:
        memory: "32Mi"
        cpu: "250m"
      limits:
        memory: "64Mi"
        cpu: "1000m"
```

## Repo Options

The following properties are available for configuring the Repo server component.

Name | Default | Description
--- | --- | ---
Resources | [Empty] | The container compute resources.

### Repo Example

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: repo
spec:
  repo:
    resources:
      requests:
        memory: "32Mi"
        cpu: "250m"
      limits:
        memory: "64Mi"
        cpu: "1000m"
```

## Server Options

The following properties are available for configuring the Argo CD Server component.

Name | Default | Description
--- | --- | ---
Autoscale | [Object](#server-autoscale-options) | Autoscale options. See [below](#server-autoscale-options) for more detail.
GRPC.Host | example-argocd-grpc | The hostname to use for Ingress/Route GRPC resources.
Host | example-argocd | The hostname to use for Ingress/Route resources.
Insecure | false | Toggles the insecure flag for Argo CD Server.
Resources | [Empty] | The container compute resources.
Service.Type | ClusterIP | The ServiceType to use for the Service resource.

### Server Autoscale Options

The following properties are available to configure austoscaling for the Argo CD Server component.

Name | Default | Description
--- | --- | ---
Enabled | false | Toggle Autoscaling support globally for the Argo CD server component.
HPA | [Object] | HorizontalPodAutoscaler options for the Argo CD Server component.

### Server Example

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
    host: example-argocd
    insecure: false
    resources:
      requests:
        memory: "32Mi"
        cpu: "250m"
      limits:
        memory: "64Mi"
        cpu: "1000m"
    service:
      type: ClusterIP
```

## TLS Options

The following properties are available for configuring the Grafana component.

Name | Default | Description
--- | --- | ---
CA.ConfigMapName | example-argocd-ca | The name of the ConfigMap containing the CA Certificate.
CA.SecretName | example-argocd-ca | The name of the Secret containing the CA Certificate and Key.
Certs | [Empty] | Properties in the `argocd-tls-certs-cm` ConfigMap. Define custom TLS certificates for connecting Git repositories via HTTPS.
