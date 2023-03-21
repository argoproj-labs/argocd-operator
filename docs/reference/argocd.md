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
[**ApplicationSet**](#applicationset-controller-options) | [Object] | ApplicationSet controller configuration options.
[**ConfigManagementPlugins**](#config-management-plugins) | [Empty] | Configuration to add a config management plugin.
[**Controller**](#controller-options) | [Object] | Argo CD Application Controller options.
[**Dex**](#dex-options) | [Object] | Dex configuration options.
[**DisableAdmin**](#disable-admin) | `false` | Disable the admin user.
[**GATrackingID**](#ga-tracking-id) | [Empty] | The google analytics tracking ID to use.
[**GAAnonymizeUsers**](#ga-anonymize-users) | `false` | Enable hashed usernames sent to google analytics.
[**Grafana**](#grafana-options) | [Object] | Grafana configuration options.
[**HA**](#ha-options) | [Object] | High Availability options.
[**HelpChatURL**](#help-chat-url) | `https://mycorp.slack.com/argo-cd` | URL for getting chat help, this will typically be your Slack channel for support.
[**HelpChatText**](#help-chat-text) | `Chat now!` | The text for getting chat help.
[**Image**](#image) | `argoproj/argocd` | The container image for all Argo CD components. This overrides the `ARGOCD_IMAGE` environment variable.
[**Import**](#import-options) | [Object] | Import configuration options.
[**Ingress**](#ingress-options) | [Object] | Ingress configuration options.
[**InitialRepositories**](#initial-repositories) | [Empty] | Initial git repositories to configure Argo CD to use upon creation of the cluster.
[**Notifications**](#notifications-controller-options) | [Object] | Notifications controller configuration options.
[**RepositoryCredentials**](#repository-credentials) | [Empty] | Git repository credential templates to configure Argo CD to use upon creation of the cluster.
[**InitialSSHKnownHosts**](#initial-ssh-known-hosts) | [Default Argo CD Known Hosts] | Initial SSH Known Hosts for Argo CD to use upon creation of the cluster.
[**KustomizeBuildOptions**](#kustomize-build-options) | [Empty] | The build options/parameters to use with `kustomize build`.
[**OIDCConfig**](#oidc-config) | [Empty] | The OIDC configuration as an alternative to Dex.
[**NodePlacement**](#nodeplacement-option) | [Empty] | The NodePlacement configuration can be used to add nodeSelector and tolerations.
[**Prometheus**](#prometheus-options) | [Object] | Prometheus configuration options.
[**RBAC**](#rbac-options) | [Object] | RBAC configuration options.
[**Redis**](#redis-options) | [Object] | Redis configuration options.
[**ResourceCustomizations**](#resource-customizations) | [Empty] | Customize resource behavior.
[**ResourceExclusions**](#resource-exclusions) | [Empty] | The configuration to completely ignore entire classes of resource group/kinds.
[**ResourceInclusions**](#resource-inclusions) | [Empty] | The configuration to configure which resource group/kinds are applied.
[**ResourceTrackingMethod**](#resource-tracking-method) | `label` | The resource tracking method Argo CD should use.
[**Server**](#server-options) | [Object] | Argo CD Server configuration options.
[**SSO**](#single-sign-on-options) | [Object] | Single sign-on options.
[**StatusBadgeEnabled**](#status-badge-enabled) | `true` | Enable application status badge feature.
[**TLS**](#tls-options) | [Object] | TLS configuration options.
[**UsersAnonymousEnabled**](#users-anonymous-enabled) | `true` | Enable anonymous user access.
[**Version**](#version) | v2.4.0 (SHA) | The tag to use with the container image for all Argo CD components.
[**Banner**](#banner) | [Object] | Add a UI banner message.

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

## ApplicationSet Controller Options

The following properties are available for configuring the ApplicationSet controller component.

Name | Default | Description
--- | --- | ---
Env | [Empty] | Environment to set for the applicationSet controller workloads
[ExtraCommandArgs](#add-command-arguments-to-applicationsets-controller) | [Empty] | Extra Command arguments allows users to pass command line arguments to applicationSet workload. They get added to default command line arguments provided by the operator.
Image | `quay.io/argoproj/argocd-applicationset` | The container image for the ApplicationSet controller. This overrides the `ARGOCD_APPLICATIONSET_IMAGE` environment variable.
Version | *(recent ApplicationSet version)* | The tag to use with the ApplicationSet container image.
Resources | [Empty] | The container compute resources.
LogLevel | info | The log level to be used by the ArgoCD Application Controller component. Valid options are debug, info, error, and warn.
LogFormat | text | The log format to be used by the ArgoCD Application Controller component. Valid options are text or json.
ParallelismLimit | 10 | The kubectl parallelism limit to set for the controller (`--kubectl-parallelism-limit` flag)

### ApplicationSet Controller Example

The following example shows all properties set to the default values.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: applicationset
spec:
  applicationSet: {}
```

### Add Command Arguments to ApplicationSets Controller

Below example shows how a user can add command arguments to the ApplicationSet controller. 

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: applicationset
spec:
  applicationSet:
    extraCommandArgs:
      - --foo
      - bar
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
LogLevel | info | The log level to be used by the ArgoCD Application Controller component. Valid options are debug, info, error, and warn.
AppSync | 3m | AppSync is used to control the sync frequency of ArgoCD Applications
Sharding.enabled | false | Whether to enable sharding on the ArgoCD Application Controller component. Useful when managing a large number of clusters to relieve memory pressure on the controller component.
Sharding.replicas | 1 | The number of replicas that will be used to support sharding of the ArgoCD Application Controller.
Env | [Empty] | Environment to set for the application controller workloads

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

The following example shows how to set command line parameters using the env variable 

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: controller
spec:
  controller:
    env:
    - name: ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_TIMEOUT_SECONDS
      value: '120'    
```

## Dex Options

!!! warning 
    `.spec.dex` is deprecated and support will be removed in Argo CD operator v0.6.0. Please use `.spec.sso.dex` to configure Dex.

The following properties are available for configuring the Dex component.

Name | Default | Description
--- | --- | ---
Config | [Empty] | The `dex.config` property in the `argocd-cm` ConfigMap.
Groups | [Empty] | Optional list of required groups a user must be a member of
Image | `quay.io/dexidp/dex` | The container image for Dex. This overrides the `ARGOCD_DEX_IMAGE` environment variable.
OpenShiftOAuth | false | Enable automatic configuration of OpenShift OAuth authentication for the Dex server. This is ignored if a value is present for `Dex.Config`.
Resources | [Empty] | The container compute resources.
Version | v2.21.0 (SHA) | The tag to use with the Dex container image.

### Dex Example

The following examples show all properties set to the default values. Both configuration methods will be supported until v0.6.0

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
    groups:
      - default
    image: quay.io/dexidp/dex
    openShiftOAuth: false
    resources: {}
    version: v2.21.0
```
OR

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: dex
spec:
  sso:
    provider: dex
    dex:
      config: ""
      groups:
        - default
      image: quay.io/dexidp/dex
      openShiftOAuth: false
      resources: {}
      version: v2.21.0
```

Please refer to the [dex user guide](../usage/dex.md) to learn more about configuring dex as a Single sign-on provider.


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
    openShiftOAuth: true
  rbac:
    defaultPolicy: 'role:readonly'
    policy: |
      g, cluster-admins, role:admin
    scopes: '[groups]'
```

### Important Note regarding Role Mappings:

To have a specific user be properly atrributed with the `role:admin` upon SSO through Openshift, the user needs to be in a **group** with the `cluster-admin` role added. If the user only has a direct `ClusterRoleBinding` to the Openshift role for `cluster-admin`, the ArgoCD role will not map. 

A quick fix will be to create an `cluster-admins` group, add the user to the group and then apply the `cluster-admin` ClusterRole to the group.

```
oc adm groups new cluster-admins
oc adm groups add-users cluster-admins USER
oc adm policy add-cluster-role-to-group cluster-admin cluster-admins
```

## Disable Admin

Disable the admin user. This property maps directly to the `admin.enabled` field in the `argocd-cm` ConfigMap.

### Disable Admin Example

The following example disables the admin user using the `DisableAdmin` property on the `ArgoCD` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: disable-admin
spec:
  disableAdmin: true
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
Image | `grafana/grafana` | The container image for Grafana. This overrides the `ARGOCD_GRAFANA_IMAGE` environment variable.
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
IngressClassName | [Empty] | IngressClass to use for the Ingress resource.
Path | `/` | Path to use for Ingress resources.
TLS | [Empty] | TLS configuration for the Ingress.

### Grafana Route Options

The following properties are available to configure the Route for the Grafana component.

Name | Default | Description
--- | --- | ---
Annotations | [Empty] | The map of annotations to add to the Route.
Enabled | `false` | Toggles the creation of a Route for the Grafana component.
Labels | [Empty] | The map of labels to add to the Route.
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
Enabled | `false` | Toggle High Availability support globally for Argo CD.
RedisProxyImage | `haproxy` | The Redis HAProxy container image. This overrides the `ARGOCD_REDIS_HA_PROXY_IMAGE`environment variable.
RedisProxyVersion | `2.0.4` | The tag to use for the Redis HAProxy container image.

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
    redisProxyImage: haproxy
    redisProxyVersion: "2.0.4"
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

## Notifications Controller Options

The following properties are available for configuring the Notifications controller component.

Name | Default | Description
--- | --- | ---
Enabled | `false` | The toggle that determines whether notifications-controller should be started or not.
Env | [Empty] | Environment to set for the notifications workloads.
Image | `argoproj/argocd` | The container image for all Argo CD components. This overrides the `ARGOCD_IMAGE` environment variable.
Version | *(recent Argo CD version)* | The tag to use with the Notifications container image.
Resources | [Empty] | The container compute resources.
LogLevel | info | The log level to be used by the ArgoCD Application Controller component. Valid options are debug, info, error, and warn.

### Notifications Controller Example

The following example shows all properties set to the default values.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  notifications:
    enabled: true
```

## Repository Credentials

Git repository credential templates to configure Argo CD to use upon creation of the cluster.

This property maps directly to the `repository.credentials` field in the `argocd-cm` ConfigMap.

### Repository Credentials Example

The following example sets a value in the `argocd-cm` ConfigMap using the `RepositoryCredentials` property on the `ArgoCD` resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: repository-credentials
spec:
  repositoryCredentials: |
    - sshPrivateKeySecret:
        key: sshPrivateKey
        name: my-ssh-secret
      type: git
      url: ssh://git@gitlab.com/my-org/
```

## Initial SSH Known Hosts

Initial SSH Known Hosts for Argo CD to use upon creation of the cluster.

This property maps directly to the `ssh_known_hosts` field in the `argocd-ssh-known-hosts-cm` ConfigMap. Updating this property after the cluster has been created has no affect and should be used only as a means to initialize the cluster with the value provided. Modifications to the `ssh_known_hosts` field should then be made through the Argo CD web UI or CLI.

The following properties are available for configuring the import process.

Name | Default | Description
--- | --- | ---
ExcludeDefaultHosts | false | Whether you would like to exclude the default SSH Hosts entries that ArgoCD provides
Keys | "" | Additional SSH Hosts entries that you would like to include with ArgoCD

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
  initialSSHKnownHosts:
    excludedefaulthosts: false
    keys: |
      my-git.org ssh-rsa AAAAB3NzaC...
      my-git.com ssh-rsa AAAAB3NzaC...
```

## Keycloak Options

The following properties are available for configuring Keycloak Single sign-on provider.

Name | Default | Description
--- | --- | ---
Image | OpenShift - `registry.redhat.io/rh-sso-7/sso75-openshift-rhel8` <br/> Kuberentes - `quay.io/keycloak/keycloak` | The container image for keycloak. This overrides the `ARGOCD_KEYCLOAK_IMAGE` environment variable.
Resources | `Requests`: CPU=500m, Mem=512Mi, `Limits`: CPU=1000m, Mem=1024Mi | The container compute resources.
RootCA | "" | root CA certificate for communicating with the OIDC provider
VerifyTLS | true | Whether to enforce strict TLS checking when communicating with Keycloak service.
Version | OpenShift - `sha256:720a7e4c4926c41c1219a90daaea3b971a3d0da5a152a96fed4fb544d80f52e3` (7.5.1) <br/> Kubernetes - `sha256:64fb81886fde61dee55091e6033481fa5ccdac62ae30a4fd29b54eb5e97df6a9` (15.0.2) | The tag to use with the keycloak container image.

### Keycloak Single sign-on Example

The following example uses keycloak as Single sign-on option for Argo CD.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: status-badge-enabled
spec:
  sso:
    provider: keycloak
```

Please refer to the [keycloak user guide](../usage/keycloak/kubernetes.md) to learn more about configuring keycloak as a Single sign-on provider.

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

## KustomizeVersions Options

A list of configured Kustomize versions within your ArgoCD Repo Server Container Image. For each version, this generates the `kustomize.version.vX.Y.Z` field in the `argocd-cm` ConfigMap.

The following properties are available for each item in the KustomizeVersions list.

Name | Default | Description
--- | --- | ---
Version | "" | The Kustomize version in the format vX.Y.Z that is configured in your ArgoCD Repo Server container image.
Path | "" | The path to the specified kustomize version on the file system within your ArgoCD Repo Server container image.

## KustomizeVersions Example

The following example configures additional Kustomize versions that are available within the ArgoCD Repo Server container image. These versions already need to be made available via a custom image. Only setting these properties in your ConfigMap does not automatically make them available if they are already not there.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: kustomize-versions
spec:
  kustomizeVersions:
    - version: v4.1.0
      path: /path/to/kustomize-4.1
    - version: v3.5.4
      path: /path/to/kustomize-3.5.4
```

## OIDC Config

OIDC configuration as an alternative to dex (optional). This property maps directly to the `oidc.config` field in the `argocd-cm` ConfigMap.

### OIDC Config Example

The following example sets a value in the `argocd-cm` ConfigMap using the `oidcConfig` property on the `ArgoCD` resource.

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

## NodePlacement Option

The following properties are available for configuring the NodePlacement component.

Name | Default | Description
--- | --- | ---
NodeSelector | [Empty] | A map of key value pairs for node selection.
Tolerations | [Empty] | Tolerations allow pods to schedule on nodes with matching taints.

### NodePlacement Example

The following example sets a NodeSelector and tolerations using NodePlacement property in the ArgoCD CR

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: nodeplacement-example
spec:
  nodePlacement: 
    nodeSelector: 
      key1: value1
    tolerations: 
    - key: key1
      operator: Equal
      value: value1
      effect: NoSchedule
    - key: key1
      operator: Equal
      value: value1
      effect: NoExecute   
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
IngressClassName | [Empty] | IngressClass to use for the Ingress resource.
Path | `/` | Path to use for Ingress resources.
TLS | [Empty] | TLS configuration for the Ingress.

### Prometheus Route Options

The following properties are available to configure the Route for the Prometheus component.

Name | Default | Description
--- | --- | ---
Annotations | [Empty] | The map of annotations to add to the Route.
Enabled | `false` | Toggles the creation of a Route for the Prometheus component.
Labels | [Empty] | The map of labels to add to the Route.
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
PolicyMatcherMode | `glob` | The `policy.matchMode` property in the `argocd-rbac-cm` ConfigMap. There are two options for this, 'glob' for glob matcher and 'regex' for regex matcher.
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
    policyMatcherMode: 'glob'
    policy: |
      g, system:cluster-admins, role:admin
    scopes: '[groups]'
```

## Redis Options

The following properties are available for configuring the Redis component.

Name | Default | Description
--- | --- | ---
AutoTLS | "" | Provider to use for creating the redis server's TLS certificate (one of: `openshift`). Currently only available for OpenShift.
DisableTLSVerification | false | defines whether the redis server should be accessed using strict TLS validation
Image | `redis` | The container image for Redis. This overrides the `ARGOCD_REDIS_IMAGE` environment variable.
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
    disableTLSVerification: false
    autotls: ""
```

## Repo Options

The following properties are available for configuring the Repo server component.

Name | Default | Description
--- | --- | ---
[ExtraRepoCommandArgs](#pass-command-arguments-to-repo-server) | [Empty] | Extra Command arguments allows users to pass command line arguments to repo server workload. They get added to default command line arguments provided by the operator.
Resources | [Empty] | The container compute resources.
MountSAToken | false | Whether the ServiceAccount token should be mounted to the repo-server pod.
ServiceAccount | "" | The name of the ServiceAccount to use with the repo-server pod.
VerifyTLS | false | Whether to enforce strict TLS checking on all components when communicating with repo server
AutoTLS | "" | Provider to use for setting up TLS the repo-server's gRPC TLS certificate (one of: `openshift`). Currently only available for OpenShift.
Image | `argoproj/argocd` | The container image for ArgoCD Repo Server. This overrides the `ARGOCD_REPOSERVER_IMAGE` environment variable.
Version | same as `.spec.Version` | The tag to use with the ArgoCD Repo Server.
LogLevel | info | The log level to be used by the ArgoCD Repo Server. Valid options are debug, info, error, and warn.
LogFormat | text | The log format to be used by the ArgoCD Repo Server. Valid options are text or json.
ExecTimeout | 180 | Execution timeout in seconds for rendering tools (e.g. Helm, Kustomize)
Env | [Empty] | Environment to set for the repository server workloads
Replicas | [Empty] | The number of replicas for the ArgoCD Repo Server. Must be greater than or equal to 0.

### Pass Command Arguments To Repo Server

Allows a user to pass additional arguments to Argo CD Repo Server command.

Name | Default | Description
--- | --- | ---
ExtraCommandArgs | [Empty] | Extra Command arguments allows users to pass command line arguments to repo server workload. They get added to default command line arguments
provided by the operator.

!!! note
    The command line arguments provided as part of ExtraRepoCommandArgs will not overwrite the default command line arguments created by the operator.

### Repo Server Example

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
    verifytls: false
    autotls: ""
    replicas: 1
```

### Repo Server Command Arguments Example

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: server
spec:
  repo:
    extraRepoCommandArgs:
      - --reposerver.max.combined.directory.manifests.size
      - 10M
```

## Resource Customizations

There are two ways to customize resource behavior- the first way, only available with release v0.5.0+, is with subkeys (`resourceHealthChecks`, `resourceIgnoreDifferences`, and `resourceActions`), the second is without subkeys (`resourceCustomizations`). `resourceCustomizations` maps directly to the `resource.customizations` field in the `argocd-cm` ConfigMap, while each of the subkeys maps directly to their own field in the `argocd-cm`. `resourceHealthChecks` will map to `resource.customizations.health`, `resourceIgnoreDifferences` to `resource.customizations.ignoreDifferences`, and `resourceActions` to `resource.customizations.actions`.

!!! warning 
    `resourceCustomizations` is being deprecated so is encouraged to use `resourceHealthChecks`, `resourceIgnoreDifferences`, and `resourceActions` instead. It is the user's responsibility to not provide conflicting resources if they choose to use both methods of resource customizations. 

### Resource Customizations (with subkeys) Example

Keys for `resourceHealthChecks`, `resourceIgnoreDifferences`, and `resourceActions` are in the form (respectively): `resource.customizations.health.<group_kind>`, `resource.customizations.ignoreDifferences.<group_kind>`, and `resource.customizations.actions.<group_kind>`. The following example defines a custom health check, custom action, and an ignoreDifferences config in the `argocd-cm` ConfigMap. Additionally, `.spec.resourceIgnoreDifferences.all` allows you to apply these specified settings to all resources managed by this Argo CD instance.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: argocd
spec:
  resourceIgnoreDifferences:
    all:
      jsonPointers:
        - /spec/replicas
      managedFieldsManagers:
        - kube-controller-manager
    resourceIdentifiers:
      - group: admissionregistration.k8s.io
        kind: MutatingWebhookConfiguration
        customization:
          jqPathExpressions:
            - '.webhooks[]?.clientConfig.caBundle'
      - group: apps
        kind: Deployment
        customization:
          managedFieldsManagers:
            - kube-controller-manager
          jsonPointers:
            - /spec/replicas
  resourceHealthChecks:
    - group: certmanager.k8s.io
      kind: Certificate
      check: |
        hs = {}
        if obj.status ~= nil then
          if obj.status.conditions ~= nil then
            for i, condition in ipairs(obj.status.conditions) do
              if condition.type == "Ready" and condition.status == "False" then
                hs.status = "Degraded"
                hs.message = condition.message
                return hs
              end
              if condition.type == "Ready" and condition.status == "True" then
                hs.status = "Healthy"
                hs.message = condition.message
                return hs
              end
            end
          end
        end
        hs.status = "Progressing"
        hs.message = "Waiting for certificate"
        return hs
  resourceActions:
    - group: apps
      kind: Deployment
      action: |
        discovery.lua: |
        actions = {}
        actions["restart"] = {}
        return actions
        definitions:
        - name: restart
          # Lua Script to modify the obj
          action.lua: |
            local os = require("os")
            if obj.spec.template.metadata == nil then
                obj.spec.template.metadata = {}
            end
            if obj.spec.template.metadata.annotations == nil then
                obj.spec.template.metadata.annotations = {}
            end
            obj.spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"] = os.date("!%Y-%m-%dT%XZ")
            return obj
```
 After applying these changes your `argocd-cm` Configmap should contain the following fields: 

```
resource.customizations.ignoreDifferences.all: |
  jsonPointers:
  - /spec/replicas
  managedFieldsManagers:
  - kube-controller-manager

resource.customizations.ignoreDifferences.admissionregistration.k8s.io_MutatingWebhookConfiguration: |
  jqpathexpressions:
  - '.webhooks[]?.clientConfig.caBundle'

resource.customizations.ignoreDifferences.apps_deployments: |
  managedFieldsManagers:
  - kube-controller-manager
  jsonPointers:
  - /spec/replicas

resource.customizations.health.certmanager.k8s.io_Certificate: |
  hs = {}
  if obj.status ~= nil then
    if obj.status.conditions ~= nil then
      for i, condition in ipairs(obj.status.conditions) do
        if condition.type == "Ready" and condition.status == "False" then
          hs.status = "Degraded"
          hs.message = condition.message
          return hs
        end
        if condition.type == "Ready" and condition.status == "True" then
          hs.status = "Healthy"
          hs.message = condition.message
          return hs
        end
      end
    end
  end
  hs.status = "Progressing"
  hs.message = "Waiting for certificate"
  return hs

resource.customizations.actions.apps_Deployment: |
  discovery.lua: |
  actions = {}
  actions["restart"] = {}
  return actions
  definitions:
  - name: restart
    # Lua Script to modify the obj
    action.lua: |
      local os = require("os")
      if obj.spec.template.metadata == nil then
          obj.spec.template.metadata = {}
      end
      if obj.spec.template.metadata.annotations == nil then
          obj.spec.template.metadata.annotations = {}
      end
      obj.spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"] = os.date("!%Y-%m-%dT%XZ")
      return obj
```

### Resource Customizations Example

!!! warning 
    `resourceCustomizations` is being deprecated in favor of `resourceHealthChecks`, `resourceIgnoreDifferences`, and `resourceActions`.

The following example defines a custom PVC health check in the `argocd-cm` ConfigMap using the `ResourceCustomizations` property on the `ArgoCD` resource.

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

## Resource Inclusions

In addition to exclusions, you might configure the list of included resources using the resourceInclusions setting.

By default, all resource group/kinds are included. The resourceInclusions setting allows customizing the list of included group/kinds.

### Resource Inclusions Example

The following example sets a value in the `argocd-cm` ConfigMap using the `ResourceInclusions` property on the `ArgoCD` resource.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: resource-inclusion
spec:
  resourceInclusions: |
    - apiGroups:
      - "*"
      kinds:
      - Deployment
      clusters:
      - https://192.168.0.20
```

## Resource Tracking Method

You can configure which 
[resource tracking method](https://argo-cd.readthedocs.io/en/stable/user-guide/resource_tracking/#choosing-a-tracking-method)
Argo CD should use to keep track of the resources it manages.

Valid values are:

* `label` - Track resources using a label
* `annotation` - Track resources using an annotation
* `annotation+label` - Track resources using both, an annotation and a label

The default is to use `label` as tracking method.

When this value is changed, existing managed resources will re-sync to apply the new tracking method.

### Resource Tracking Method

The following example sets the resource tracking method to `annotation+label`

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: resource-tracking-method
spec:
  resourceTrackingMethod: annotation+label
```

## Server Options

The following properties are available for configuring the Argo CD Server component.

Name | Default | Description
--- | --- | ---
[Autoscale](#server-autoscale-options) | [Object] | Server autoscale configuration options.
[ExtraCommandArgs](#server-command-arguments) | [Empty] | List of arguments that will be added to the existing arguments set by the operator.
[GRPC](#server-grpc-options) | [Object] | GRPC configuration options.
Host | example-argocd | The hostname to use for Ingress/Route resources.
[Ingress](#server-ingress-options) | [Object] | Ingress configuration for the Argo CD Server component.
Insecure | false | Toggles the insecure flag for Argo CD Server.
Resources | [Empty] | The container compute resources.
Replicas | [Empty] | The number of replicas for the ArgoCD Server. Must be greater than equal to 0. If Autoscale is enabled, Replicas is ignored.
[Route](#server-route-options) | [Object] | Route configuration options.
Service.Type | ClusterIP | The ServiceType to use for the Service resource.
LogLevel | info | The log level to be used by the ArgoCD Server component. Valid options are debug, info, error, and warn.
LogFormat | text | The log format to be used by the ArgoCD Server component. Valid options are text or json.
Env | [Empty] | Environment to set for the server workloads

### Server Autoscale Options

The following properties are available to configure austoscaling for the Argo CD Server component.

Name | Default | Description
--- | --- | ---
Enabled | false | Toggle Autoscaling support globally for the Argo CD server component.
HPA | [Object] | HorizontalPodAutoscaler options for the Argo CD Server component.

!!! note
    When `.spec.server.autoscale.enabled` is set to `true`, the number of required replicas (if set) in `.spec.server.replicas` will be ignored. The final replica count on the server deployment will be controlled by the Horizontal Pod Autoscaler instead. 

### Server Command Arguments

Allows a user to pass arguments to Argo CD Server command.

Name | Default | Description
--- | --- | ---
ExtraCommandArgs | [Empty] | List of arguments that will be added to the existing arguments set by the operator.

!!! note
    ExtraCommandArgs will not be added, if one of these commands is already part of the server command with same or different value.

### Server Command Arguments Example

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: server
spec:
  server:
    extraCommandArgs:
      - --rootpath
      - /argocd
```

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
IngressClassName | [Empty] | IngressClass to use for the Ingress resource.
Path | `/` | Path to use for Ingress resources.
TLS | [Empty] | TLS configuration for the Ingress.

### Server Ingress Options

The following properties are available for configuring the Argo CD server Ingress.

Name | Default | Description
--- | --- | ---
Annotations | [Empty] | The map of annotations to use for the Ingress resource.
Enabled | `false` | Toggle creation of an Ingress resource.
IngressClassName | [Empty] | IngressClass to use for the Ingress resource.
Path | `/` | Path to use for Ingress resources.
TLS | [Empty] | TLS configuration for the Ingress.

### Server Route Options

The following properties are available to configure the Route for the Argo CD Server component.

Name | Default | Description
--- | --- | ---
Annotations | [Empty] | The map of annotations to add to the Route.
Enabled | `false` | Toggles the creation of a Route for the Argo CD Server component.
Labels | [Empty] | The map of labels to add to the Route.
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
    extraCommandArgs:
      - --rootpath
      - /argocd
    grpc:
      host: example-argocd-grpc
      ingress: false
    host: example-argocd
    ingress:
      enabled: false
    insecure: false
    replicas: 1
    resources: {}
    route:
      annotations: {}
      enabled: false
      path: /
      tls:
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

## Single sign-on Options

!!! warning
    `.spec.sso.Image`, `.spec.sso.Version`, `.spec.sso.Resources` and `.spec.sso.verifyTLS` are deprecated and support will be removed in Argo CD operator v0.6.0. Please use equivalent fields under `.spec.sso.keycloak` to configure your keycloak instance.

The following properties are available for configuring the Single sign-on component.

Name | Default | Description
--- | --- | ---
Image | OpenShift - `registry.redhat.io/rh-sso-7/sso75-openshift-rhel8` <br/> Kuberentes - `quay.io/keycloak/keycloak` | The container image for keycloak. This overrides the `ARGOCD_KEYCLOAK_IMAGE` environment variable.
[Keycloak](#keycloak-options) | [Object] | Configuration options for Keycloak SSO provider
[Dex](#dex-options) | [Object] | Configuration options for Dex SSO provider
Provider | [Empty] | The name of the provider used to configure Single sign-on. For now the supported options are Dex and keycloak.
Resources | `Requests`: CPU=500m, Mem=512Mi, `Limits`: CPU=1000m, Mem=1024Mi | The container compute resources.
VerifyTLS | true | Whether to enforce strict TLS checking when communicating with Keycloak service.
Version | OpenShift - `sha256:720a7e4c4926c41c1219a90daaea3b971a3d0da5a152a96fed4fb544d80f52e3` (7.5.1) <br/> Kubernetes - `sha256:64fb81886fde61dee55091e6033481fa5ccdac62ae30a4fd29b54eb5e97df6a9` (15.0.2) | The tag to use with the keycloak container image.

## TLS Options

The following properties are available for configuring the TLS settings.

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

### IntialCerts Example

Initial set of repository certificates to be configured in Argo CD upon creation of the cluster.

This property maps directly to the data field in the argocd-tls-certs-cm ConfigMap. Updating this property after the cluster has been created has no affect and should be used only as a means to initialize the cluster with the value provided. Updating new certificates should then be made through the Argo CD web UI or CLI.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: intialCerts
spec:
  tls:
    ca: {}
    initialCerts:
      test.example.com: |
        -----BEGIN CERTIFICATE-----
        -----END CERTIFICATE-----
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
  version: v1.7.7
```

## Banner

The following properties are available for configuring a [UI banner message](https://argo-cd.readthedocs.io/en/stable/operator-manual/custom-styles/#banners). 

Name | Default | Description
--- | --- | ---
Banner.Content | [Empty] | The banner message content (required if a banner should be displayed).
Banner.URL | [Empty] | The banner message link URL (optional).

### Banner Example
The following example enables a UI banner with message content and URL.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: version
spec:
  banner:
    content: "Custom Styles - Banners"
    url: "https://argo-cd.readthedocs.io/en/stable/operator-manual/custom-styles/#banners"
```
