# keycloak

## Synopsis

This is a test to verify the Single Sign-on with keycloak for Argo CD.

## Success criteria

Verify all the keycloak resources(deployment, service, ingress and secret) are created by adding `.sso.provider: keycloak` in the Argo CD CR.
Verify OIDC configuration is updated for keycloak in argocd-cm configmap.
Verify Keycloak resources and configuration is deleted by removing `.sso.provider: keycloak` in the Argo CD CR.

## Remarks

TODO: Verify Argo CD login using a Keycloak User.
