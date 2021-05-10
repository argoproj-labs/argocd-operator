# Usage

Note: This feature is currently supported only in openshift and is currently work-in-progress.

The following example shows the most minimal valid manifest to create a new Argo CD cluster with keycloak as Single sign-on provider.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: basic
spec:
  sso:
    provider: keycloak
```

## Create

Create a new Argo CD cluster in the `argocd` namespace using the provided basic example.

```bash
kubectl create -n argocd -f examples/argocd-basic.yaml
```

The above configuration creates a keycloak instance and its relevant resources along with the Argo CD resources. Users can login into the keycloak console using the below commands.

Get the Keycloak Route URL for Login.

```bash
kubectl -n argocd get route sso
```

```bash
NAME   HOST/PORT                                                                PATH   SERVICES   PORT    TERMINATION   WILDCARD
sso    sso-default.apps.ci-ln-rh7g922-d5d6b.origin-ci-int-aws.dev.rhcloud.com          sso        <all>   reencrypt     None
```

Get the Keycloak Credentials which are stored as environment variables in the keycloak pod.  

Get the Keycloak Pod name.

```bash
kubectl -n argocd get pods
```

```bash
NAME                                         READY   STATUS             RESTARTS   AGE
sso-1-2sjcl                                  1/1     Running            0          45m
```

Get the Keycloak Username.

```bash
kubectl -n argocd exec sso-1-2sjcl -- "env" | grep SSO_ADMIN_USERNAME
```

```bash
SSO_ADMIN_USERNAME=Cqid54Ih
```

Get the Keycloak Password.

```bash
kubectl -n argocd exec sso-1-2sjcl -- "env" | grep SSO_ADMIN_PASSWORD
```

```bash
SSO_ADMIN_PASSWORD=GVXxHifH
```