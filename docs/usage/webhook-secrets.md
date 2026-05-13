# Webhook Secrets

- [Overview](#overview)
- [Configuring webhook secrets](#configuring-webhook-secrets)
  - [GitHub](#github)
  - [GitLab](#gitlab)
  - [Bitbucket Cloud](#bitbucket-cloud)
  - [Bitbucket Server](#bitbucket-server)
  - [Gogs](#gogs)
  - [Azure DevOps](#azure-devops)
- [Secret Management Integration](#secret-management-integration)
  - [External Secrets Operator](#external-secrets-operator)
  - [Sealed Secrets](#sealed-secrets)
- [Migration from Manual Configuration](#migration-from-manual-configuration)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)

## Overview

Argo CD uses webhook secrets to validate incoming webhook requests from Git providers. The Argo CD Operator now supports declarative management of these webhook secrets through the `spec.webhookSecrets` field in the Argo CD custom resource (`kind: ArgoCD`).

This approach provides several benefits:

* **GitOps-friendly**: Webhook secrets can be managed alongside the Argo CD configuration
* **Secret management integration**: Works seamlessly with Sealed Secrets, External Secrets Operator, and other Kubernetes secret management tools
* **Simplified operations**: No need to manually edit the `argocd-secret` Secret; the operator syncs values from the referenced Secrets (the Argo CD server may need to be restarted to reload configuration—see [Verification](#verification))
* **Multi-provider support**: Configure webhook secrets for multiple Git providers in a single resource

When `spec.webhookSecrets` is configured, the operator automatically populates the appropriate keys in the `argocd-secret` Secret that Argo CD uses internally—using the **same `argocd-secret` data key names** documented in Argo CD’s [Git Webhook Configuration](https://argo-cd.readthedocs.io/en/stable/operator-manual/webhook/) (for example `webhook.github.secret`, `webhook.bitbucket.uuid`).

A minimal GitHub-focused sample that can be adapted and applied lives in the operator repository:
[examples/argocd-webhook-secrets.yaml](https://github.com/argoproj-labs/argocd-operator/blob/master/examples/argocd-webhook-secrets.yaml).

## Configuring webhook secrets

Declarative webhook credentials use the same pattern as other operator features: **store the sensitive material in a Kubernetes `Secret`**, then **point the Argo CD Operator at that `Secret` from the Argo CD custom resource**.

1. **Create a `Secret`** in the **same namespace as the Argo CD instance** (the namespace of the `ArgoCD` CR). The examples below use the namespace **`argocd`**—replace it with the actual Argo CD namespace if different. Any object name and data keys can be chosen; the examples use names like `github-webhook-credentials` and keys like `token` or `password`.
2. **Set `spec.webhookSecrets`** on the Argo CD CR. Examples use `apiVersion: argoproj.io/v1beta1`; the same field exists on **`v1alpha1`**. Under each provider, use the appropriate `*SecretRef` field and set `name` to the `Secret` and `key` to the key holding that provider’s value.
3. **Let the operator reconcile.** It copies the referenced values into the `argocd-secret` keys that Argo CD reads. Confirm with the [Verification](#verification) steps.

For repository webhooks in the Git provider (payload URL `/api/webhook`, optional shared secret, events, etc.), follow [Argo CD’s Git Webhook Configuration](https://argo-cd.readthedocs.io/en/stable/operator-manual/webhook/)—start with **step 1 (create the webhook in the Git provider)**—then use this operator guide to supply those secrets declaratively.

!!! warning "Security"
    Do not commit plain-text secrets to Git. Use [Secret Management Integration](#secret-management-integration) (External Secrets Operator, Sealed Secrets, etc.) or keep raw `Secret` manifests with `stringData` out of version control.

| Provider | Field under `spec.webhookSecrets` | Reference on the CR | What the referenced key must contain |
|----------|-------------------------------------|----------------------|--------------------------------------|
| GitHub | `github` | `webhookSecretRef` | Shared webhook secret (→ `webhook.github.secret`) |
| GitLab | `gitlab` | `webhookSecretRef` | Shared webhook secret (→ `webhook.gitlab.secret`) |
| Bitbucket Cloud | `bitbucket` | `webhookUUIDSecretRef` | Webhook **UUID** (→ `webhook.bitbucket.uuid`; see *Special handling for Bitbucket Cloud* in [Git Webhook Configuration](https://argo-cd.readthedocs.io/en/stable/operator-manual/webhook/)) |
| Bitbucket Server | `bitbucketServer` | `webhookSecretRef` | Webhook secret (→ `webhook.bitbucketserver.secret`) |
| Gogs | `gogs` | `webhookSecretRef` | Webhook secret (→ `webhook.gogs.secret`) |
| Azure DevOps | `azureDevOps` | `usernameSecretRef` and `passwordSecretRef` (both required) | Basic-auth username and password or PAT (→ `webhook.azuredevops.username` / `webhook.azuredevops.password`) |

The **`argocd-secret` keys** below match Argo CD’s [Git Webhook Configuration](https://argo-cd.readthedocs.io/en/stable/operator-manual/webhook/) section **Configure Argo CD With The WebHook Secret**. The operator uses the same string constants as upstream Argo CD (see [`common/keys.go`](https://github.com/argoproj-labs/argocd-operator/blob/master/common/keys.go) for the exact key name constants).

| Provider | Key in `argocd-secret` |
|----------|-------------------------|
| GitHub | `webhook.github.secret` |
| GitLab | `webhook.gitlab.secret` |
| Bitbucket Cloud | `webhook.bitbucket.uuid` |
| Bitbucket Server | `webhook.bitbucketserver.secret` |
| Gogs | `webhook.gogs.secret` |
| Azure DevOps | `webhook.azuredevops.username`, `webhook.azuredevops.password` |

!!! note
    References use **`WebhookSecretKeySelector`**: only `name` and `key` are supported. There is **no** `namespace` field; the `Secret` must live in the **same namespace** as the Argo CD custom resource.

!!! note
    `spec.webhookSecrets` is optional. If left unset, existing `webhook.*` keys in `argocd-secret` are left unchanged. When set, the operator manages the providers declared; see [Migration](#migration-from-manual-configuration) and the **Declarative webhook secrets** section in [Upgrading](../upgrading.md).

!!! tip
    Labels such as `app.kubernetes.io/part-of: argocd` on the credential `Secret` are **optional**; they help organize resources and integrate with tooling. Only the GitHub example includes them—add the same labels to other manifests for consistency.

### GitHub

Create a `Secret` and reference it on the Argo CD CR (one manifest that can be saved and applied with `kubectl apply`):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: github-webhook-credentials
  namespace: argocd
  labels:
    app.kubernetes.io/part-of: argocd
    app.kubernetes.io/component: webhook
type: Opaque
stringData:
  token: "your-github-webhook-secret"
---
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
  namespace: argocd
spec:
  webhookSecrets:
    github:
      webhookSecretRef:
        name: github-webhook-credentials
        key: token
```

### GitLab

Configure GitLab webhook secrets similarly to GitHub:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gitlab-webhook-credentials
  namespace: argocd
type: Opaque
stringData:
  token: "your-gitlab-webhook-secret"
---
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
  namespace: argocd
spec:
  webhookSecrets:
    gitlab:
      webhookSecretRef:
        name: gitlab-webhook-credentials
        key: token
```

### Bitbucket Cloud

Configure Bitbucket Cloud webhook credentials. The value is the **UUID** verified from the `X-Hook-UUID` header (see Argo CD’s *Special handling for Bitbucket Cloud* in [Git Webhook Configuration](https://argo-cd.readthedocs.io/en/stable/operator-manual/webhook/)).

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: bitbucket-webhook-credentials
  namespace: argocd
type: Opaque
stringData:
  uuid: "your-bitbucket-webhook-uuid"
---
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
  namespace: argocd
spec:
  webhookSecrets:
    bitbucket:
      webhookUUIDSecretRef:
        name: bitbucket-webhook-credentials
        key: uuid
```

### Bitbucket Server

Configure Bitbucket Server webhook secrets:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: bitbucketserver-webhook-credentials
  namespace: argocd
type: Opaque
stringData:
  secret: "your-bitbucket-server-webhook-secret"
---
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
  namespace: argocd
spec:
  webhookSecrets:
    bitbucketServer:
      webhookSecretRef:
        name: bitbucketserver-webhook-credentials
        key: secret
```

### Gogs

Configure Gogs webhook secrets:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gogs-webhook-credentials
  namespace: argocd
type: Opaque
stringData:
  secret: "your-gogs-webhook-secret"
---
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
  namespace: argocd
spec:
  webhookSecrets:
    gogs:
      webhookSecretRef:
        name: gogs-webhook-credentials
        key: secret
```

### Azure DevOps

Azure DevOps requires both username and password for webhook authentication:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: azuredevops-webhook-credentials
  namespace: argocd
type: Opaque
stringData:
  username: "your-azuredevops-username"
  password: "your-azuredevops-password-or-pat"
---
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
  namespace: argocd
spec:
  webhookSecrets:
    azureDevOps:
      usernameSecretRef:
        name: azuredevops-webhook-credentials
        key: username
      passwordSecretRef:
        name: azuredevops-webhook-credentials
        key: password
```

!!! note
    Azure DevOps webhook secrets require both `usernameSecretRef` and `passwordSecretRef`. Both fields must be provided together. The value read from `passwordSecretRef` may be a user password or a **Personal Access Token (PAT)**, depending on the webhook configuration in Azure DevOps.

## Secret Management Integration

### External Secrets Operator

The External Secrets Operator can synchronize secrets from external secret management systems (AWS Secrets Manager, HashiCorp Vault, etc.) into Kubernetes Secrets that the Argo CD CR can reference.

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: github-webhook-credentials
  namespace: argocd
  labels:
    app.kubernetes.io/part-of: argocd
    app.kubernetes.io/component: webhook
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: cluster-secret-store
    kind: ClusterSecretStore
  target:
    name: github-webhook-credentials
    creationPolicy: Owner
    template:
      type: Opaque
      data:
        token: "{{ .token }}"
  data:
    - secretKey: token
      remoteRef:
        key: argocd/github-webhook
        property: token
```

Once the ExternalSecret reconciles and creates the Secret, reference it in the Argo CD CR:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
  namespace: argocd
spec:
  webhookSecrets:
    github:
      webhookSecretRef:
        name: github-webhook-credentials
        key: token
```

### Sealed Secrets

Sealed Secrets encrypt secrets so they can be safely stored in Git repositories. Generate a SealedSecret on the cluster:

```bash
kubectl create secret generic github-webhook-credentials \
  --namespace=argocd \
  --from-literal=token='your-github-webhook-secret' \
  --dry-run=client -o yaml | kubeseal -o yaml > github-webhook-sealed.yaml
```

The generated `github-webhook-sealed.yaml` can be committed to Git. Once applied to the cluster, the Sealed Secrets controller creates the underlying Secret, which can then be referenced in the Argo CD CR:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
  namespace: argocd
spec:
  webhookSecrets:
    github:
      webhookSecretRef:
        name: github-webhook-credentials
        key: token
```

## Migration from Manual Configuration

If webhook secrets are currently managed by manually editing the `argocd-secret`, follow these steps to migrate to declarative management:

!!! note
    References resolve Secrets in **the same namespace as the Argo CD instance**. `WebhookSecretKeySelector` only supports `name` and `key` (there is no `namespace` field on the CR).

!!! note
    When `spec.webhookSecrets` is **set**, the operator syncs webhook keys from the configured references into `argocd-secret` and **clears** `webhook.*` keys for providers that are **not** listed under `spec.webhookSecrets`. Omitting `spec.webhookSecrets` entirely leaves existing `webhook.*` entries in `argocd-secret` unchanged (manual or legacy values).

1. **Extract the current webhook secret value** from `argocd-secret`:

   ```bash
   kubectl get secret argocd-secret -n argocd -o jsonpath='{.data.webhook\.github\.secret}' | base64 -d
   ```

2. **Create a new Secret** with the extracted value:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: github-webhook-credentials
     namespace: argocd
   type: Opaque
   stringData:
     token: "extracted-value-from-step-1"
   ```

   Apply the Secret:

   ```bash
   kubectl apply -f github-webhook-credentials.yaml
   ```

3. **Update the Argo CD CR** to reference the new Secret:

   ```bash
   kubectl patch argocd example-argocd -n argocd --type merge --patch '
   spec:
     webhookSecrets:
       github:
         webhookSecretRef:
           name: github-webhook-credentials
           key: token
   '
   ```

4. **Verify** that the operator has updated `argocd-secret` (see [Verification](#verification) section).

5. **Remove manual processes** that previously managed the webhook secret in `argocd-secret`.

## Verification

After configuring webhook secrets through the Argo CD CR, verify that the operator has successfully updated the `argocd-secret`:

```bash
# For GitHub
kubectl get secret argocd-secret -n argocd -o jsonpath='{.data.webhook\.github\.secret}' | base64 -d

# For GitLab
kubectl get secret argocd-secret -n argocd -o jsonpath='{.data.webhook\.gitlab\.secret}' | base64 -d

# For Bitbucket Cloud (`webhook.bitbucket.uuid`)
kubectl get secret argocd-secret -n argocd -o jsonpath='{.data.webhook\.bitbucket\.uuid}' | base64 -d

# For Bitbucket Server (`webhook.bitbucketserver.secret`)
kubectl get secret argocd-secret -n argocd -o jsonpath='{.data.webhook\.bitbucketserver\.secret}' | base64 -d

# For Gogs
kubectl get secret argocd-secret -n argocd -o jsonpath='{.data.webhook\.gogs\.secret}' | base64 -d

# For Azure DevOps
kubectl get secret argocd-secret -n argocd -o jsonpath='{.data.webhook\.azuredevops\.username}' | base64 -d
kubectl get secret argocd-secret -n argocd -o jsonpath='{.data.webhook\.azuredevops\.password}' | base64 -d
```

The output should match the value in the referenced Secret.

If any value **does not** match:

1. Check the operator logs for reconciliation errors when reading the referenced `Secret` or applying `webhook.*` keys.
2. Confirm the source `Secret` exists, includes the expected `key`, and is in the same namespace as the Argo CD CR (`kubectl get secret ... -o yaml`).
3. Confirm the Argo CD custom resource has reconciled (`kubectl get argocd -n <namespace>` and operator logs).
4. Wait briefly and re-check `argocd-secret`; reconciliation is asynchronous.

!!! note
    The Argo CD server may need to be restarted to pick up new webhook secrets. If webhooks are still failing after verification, restart the server deployment:
    ```bash
    kubectl rollout restart deployment/example-argocd-server -n argocd
    ```

## Troubleshooting

### Operator Cannot Read the Secret

**Symptom**: Operator logs show errors about being unable to read the referenced Secret.

**Solution**: `webhookSecrets` references only resolve Secrets in **the same namespace as the Argo CD CR**. Ensure the referenced Secret exists there (copy or sync it from elsewhere if the secret tooling writes to a different namespace). The operator’s service account can read Secrets in that namespace during normal reconciliation; if using a custom operator deployment or restrictive RBAC, verify it can `get` the referenced Secret.

### Secret Key Not Found

**Symptom**: Operator logs show "key not found" errors, or the webhook secret in `argocd-secret` is empty.

**Solution**: Ensure each reference’s `key` field matches an actual key in the Secret (for example `webhookSecretRef.key` for GitHub, `webhookUUIDSecretRef.key` for Bitbucket Cloud):

```bash
kubectl get secret github-webhook-credentials -n argocd -o jsonpath='{.data}' | jq
```

Verify the key name and update the Argo CD CR if necessary.

### Webhooks Still Failing After Configuration

**Symptom**: Git webhooks return validation errors even though `argocd-secret` contains the correct value.

**Solution**: The Argo CD server component needs to reload its configuration to pick up new webhook secrets. Restart the server deployment:

```bash
kubectl rollout restart deployment/example-argocd-server -n argocd
```

### Multiple Providers Not Working

**Symptom**: Webhook secrets for some providers work but others do not.

**Solution**: Ensure each provider's configuration is complete and correct. For Azure DevOps, both `usernameSecretRef` and `passwordSecretRef` must be provided:

```yaml
spec:
  webhookSecrets:
    github:
      webhookSecretRef:
        name: github-webhook-credentials
        key: token
    azureDevOps:
      usernameSecretRef:
        name: azuredevops-webhook-credentials
        key: username
      passwordSecretRef:
        name: azuredevops-webhook-credentials
        key: password
```

Each provider configuration is independent and must reference valid Secrets.
