- [Overview](#overview)
- [Dex OpenShift OAuth Connector](#dex-openshift-oauth-connector)
    - [Role Mappings](#role-mappings)
- [Dex GitHub Connector](#dex-github-connector)
- [Disable DEX](#disable-dex)

## Overview

Dex can be used to delegate authentication to external identity providers like GitHub, SAML and others. SSO configuration of Argo CD requires updating the Argo CD CR with [Dex connector](https://dexidp.io/docs/connectors/) settings.

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

## Disable DEX

Dex is installed by default for all the Argo CD instances created by the operator. You can disable this behavior using the environmental variable `DISABLE_DEX` on the operator.

Set the `DISABLE_DEX` to `true` in the Subscription resource of the operator.

```yaml
spec:
  config:
    env:
    - name: DISABLE_DEX
      value: "true"
```
