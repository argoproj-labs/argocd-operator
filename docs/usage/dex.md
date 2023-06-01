- [Overview](#overview)
- [Installing & Configuring Dex](#installing--configuring-dex)
    - [Using `.spec.sso.provider`](#using-specssoprovider)
    - [Using the DISABLE_DEX environment variable](#using-the-disable_dex-environment-variable)
- [Dex OpenShift OAuth Connector](#dex-openshift-oauth-connector)
    - [Role Mappings](#role-mappings)
- [Dex GitHub Connector](#dex-github-connector)
- [Uninstalling Dex](#uninstalling-dex)
    - [Using `.spec.sso`](#using-specsso)
    - [Using the DISABLE_DEX environment variable](#using-the-disable_dex-environment-variable-1)
    - [Using `.spec.dex`](#using-specdex)

## Overview

Dex can be used to delegate authentication to external identity providers like GitHub, SAML and others. SSO configuration of Argo CD requires updating the Argo CD CR with [Dex connector](https://dexidp.io/docs/connectors/) settings.


## Installing & Configuring Dex

#### Using `.spec.sso.provider`

Dex configuration has moved to `.spec.sso` in release v0.4.0. Dex can be enabled by setting `.spec.sso.provider` to `dex` in the Argo CD CR. 

!!! note
    It is now mandatory to specify `.spec.sso.dex` either with OpenShift configuration through `openShiftOAuth: true` or valid custom configuration supplied through `.spec.sso.dex.config`. Absence of either will result in an error due to failing health checks on Dex. 

!!! note
    Specifying `.spec.sso.dex` without setting dex as the provider will result in an error. 

An example of correctly configured dex would look as follows:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  sso:
    provider: dex
    dex:
      openShiftOAuth: true
```

#### Using the DISABLE_DEX environment variable 

!!! warning 
    `DISABLE_DEX` is deprecated and support will be removed in Argo CD operator v0.8.0. Please use `.spec.sso.provider` to enable/disable Dex.

!!! note
    `DISABLE_DEX` environment variable was earlier scheduled for removal in Argo CD operator v0.7.0, but has been extended to Argo CD operator v0.8.0.

Until release v0.4.0 of Argo CD operator, Dex resources were created by default unless the `DISABLE_DEX` environment variable was explicitly set to `true`. However, v0.4.0 onward, `DISBALE_DEX` being either unset, or set to `false` will not trigger creation of Dex resources, unless there is valid Dex configuration expressed through `.spec.dex`. Users can continue setting `DISABLE_DEX` to `true` to uninstall dex resources until v0.8.0. 

!!! warning 
    `.spec.dex` is deprecated and support will be removed in Argo CD operator v0.8.0. Please use `.spec.sso.dex` to configure Dex.

!!! note
    `.spec.dex` field was earlier scheduled for removal in Argo CD operator v0.7.0, but has been extended to Argo CD operator v0.8.0.

An example of correctly configured dex would look as follows:

Set the `DISABLE_DEX` to `false` in the Subscription resource of the operator.

```yaml
spec:
  config:
    env:
    - name: DISABLE_DEX
      value: "false"
```

and supply `.spec.dex` with valid configuration

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  dex:
    openShiftOAuth: true
```

## Dex OpenShift OAuth Connector

The below section describes how to configure Argo CD SSO using OpenShift connector as an example. Dex makes use of the users and groups defined within OpenShift by querying the platform provided OAuth server.

The `openShiftOAuth` property can be used to trigger the operator to auto configure the built-in OpenShift OAuth server. The `groups` property is used to mandate users to be part of one or all the groups in the groups list. The RBAC `Policy` property is used to give the admin role in the Argo CD cluster to users in the OpenShift `cluster-admins` group.

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
    groups:
     - default
  rbac:
    defaultPolicy: 'role:readonly'
    policy: |
      g, cluster-admins, role:admin
    scopes: '[groups]'
```

#### Role Mappings

To have a specific user be properly atrributed with the `role:admin` upon SSO through Openshift, the user needs to be in a **group** with the `cluster-admin` role added. If the user only has a direct `ClusterRoleBinding` to the Openshift role for `cluster-admin`, the Argo CD role will not map.

A quick fix will be to create a group named `cluster-admins` group, add the user to the group and then apply the `cluster-admin` ClusterRole to the group.

```txt
oc adm groups new cluster-admins
oc adm groups add-users cluster-admins USER
oc adm policy add-cluster-role-to-group cluster-admin cluster-admins
```

## Dex GitHub Connector

The below section describes how to configure Argo CD SSO using GitHub (OAuth2) as an example, but the steps should be similar for other identity providers.

1. Register the application in the identity provider as explained [here](https://argoproj.github.io/argo-cd/operator-manual/user-management/#1-register-the-application-in-the-identity-provider).

2. Update the Argo CD CR.

In the `dex.config` key, add the github connector to the connectors sub field. See the Dex [GitHub connector documentation](https://github.com/dexidp/website/blob/main/content/docs/connectors/github.md) for explanation of the fields. A minimal config should populate the clientID, clientSecret generated in Step 1.
You will very likely want to restrict logins to one or more GitHub organization. In the
`connectors.config.orgs` list, add one or more GitHub organizations. Any member of the org will then be able to login to Argo CD to perform management tasks.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: openshift-oauth
spec:
  dex:
    config: |
      connectors:
        # GitHub example
        - type: github
          id: github
          name: GitHub
          config:
            clientID: xxxxxxxxxxxxxx
            clientSecret: $dex.github.clientSecret # Alternatively $<some_K8S_secret>:dex.github.clientSecret
            orgs:
            - name: dummy-org
```

## Uninstalling Dex 

#### Using `.spec.sso`

Dex can be uninstalled either by removing `.spec.sso` from the Argo CD CR, or switching to a different SSO provider. 

#### Using the DISABLE_DEX environment variable

Dex can be uninstalled by setting `DISABLE_DEX` to `true` in the Subscription resource of the operator.

```yaml
spec:
  config:
    env:
    - name: DISABLE_DEX
      value: "true"
```

!!! warning 
    `DISABLE_DEX` is deprecated and support will be removed in Argo CD operator v0.8.0. Please use `.spec.sso.provider` to enable/disable Dex.

!!! note
    `DISABLE_DEX` environment variable was earlier scheduled for removal in Argo CD operator v0.7.0, but has been extended to Argo CD operator v0.8.0.

#### Using `.spec.dex`

Dex can be uninstalled by either removing `.spec.dex` from the Argo CD CR, or ensuring `.spec.dex.config` is empty and `.spec.dex.openShiftOAuth` is set to `false`.

!!! warning 
    `.spec.dex` is deprecated and support will be removed in Argo CD operator v0.8.0. Please use `.spec.sso.dex` to configure Dex.

!!! note
    `.spec.dex` field was earlier scheduled for removal in Argo CD operator v0.7.0, but has been extended to Argo CD operator v0.8.0.