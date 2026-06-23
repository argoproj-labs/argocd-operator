# Upgrading

This page contains upgrade instructions and migration guides for the Argo CD Operator.
## Upgrading from Operator ≤0.18 to Operator 0.19+

### ApplicationSet tokenRef strict mode
If you're upgrading to an operator version that defaults ApplicationSet tokenRef strict mode when ApplicationSets in any namespace are configured, note the following changes:

1. When `.spec.applicationSet.sourceNamespaces` expands to at least one cluster namespace, the operator sets `applicationsetcontroller.enable.tokenref.strict.mode` to `"true"` in `argocd-cmd-params-cm`
2. The ApplicationSet controller requires Secrets referenced by SCM Provider and Pull Request generators via `tokenRef` to be labeled `argocd.argoproj.io/secret-type: scm-creds`
3. Manual edits to this key in `argocd-cmd-params-cm` are corrected on reconcile; use the ArgoCD CR (`.spec.cmdParams`) to change behavior
4. You may opt out via `.spec.cmdParams`, but this is not recommended; see [ApplicationSets in Any Namespace](./usage/appsets-in-any-namespace.md#tokenref-strict-mode)

### Detection

The following users are **unaffected** by this change:
- Users who do not configure `.spec.applicationSet.sourceNamespaces` on their ArgoCD CR
- Users whose `.spec.applicationSet.sourceNamespaces` patterns match no cluster namespace
- Users who do not have ApplicationSets that use an SCM Provider or Pull Request generator with a `tokenRef` pointing at a Secret for API authentication
- Users whose SCM `tokenRef` Secrets already have `argocd.argoproj.io/secret-type: scm-creds`

The following users are **affected** and should perform remediation:
- Users with a non-empty expanded `.spec.applicationSet.sourceNamespaces` list whose ApplicationSets use an **SCM Provider** or **Pull Request** generator with a **`tokenRef`** pointing at a Secret for API authentication
- Users whose referenced Secrets are missing the `argocd.argoproj.io/secret-type: scm-creds` label

### Remediation Steps

1. **Find ApplicationSets using `tokenRef`:**

```bash
kubectl get applicationsets -A -o yaml | grep -B5 -A3 'tokenRef:'
```

Review SCM Provider and Pull Request generator blocks in each ApplicationSet.

2. **Identify referenced Secrets:**

For each `tokenRef`, note `secretName` and namespace (often the Argo CD namespace or the ApplicationSet namespace).

```bash
kubectl get secret -n <namespace> <secretName> -o yaml
```

3. **Label SCM credential Secrets:**

```bash
kubectl label secret -n <namespace> <secretName> \
  argocd.argoproj.io/secret-type=scm-creds
```

Or in Git or your secret management workflow:

```yaml
metadata:
  labels:
    argocd.argoproj.io/secret-type: scm-creds
```

4. **Verify after upgrade or reconcile:**

```bash
kubectl get cm -n <argocd-namespace> argocd-cmd-params-cm -o yaml
kubectl logs -n <argocd-namespace> deploy/<instance>-applicationset-controller --tail=50
```

Confirm `applicationsetcontroller.enable.tokenref.strict.mode` is `"true"` when source namespaces are configured, and that ApplicationSets reconcile without tokenRef or secret errors.

5. **Temporary opt-out (migration only):**

If you need time to label Secrets, set `.spec.cmdParams` on the ArgoCD CR:

```yaml
spec:
  cmdParams:
    applicationsetcontroller.enable.tokenref.strict.mode: "false"
```

This removes the `scm-creds` label requirement and is **not recommended** for production. Prefer labeling Secrets and keeping strict mode enabled. See [ApplicationSets in Any Namespace](./usage/appsets-in-any-namespace.md#tokenref-strict-mode) for details.

## Upgrading from Operator ≤0.14 (Argo CD ≤2.14) to Operator 0.15+ (Argo CD 3.0+)

### Logs RBAC Enforcement

If you're upgrading from Argo CD 2.x to 3.0+, note the following changes:

1. The `server.rbac.log.enforce.enable` flag is no longer supported
2. Logs RBAC is now enforced by default
3. Users with existing policies need to explicitly add logs permissions
4. The operator does not provide default RBAC policies - you must define your own

### Detection

The following users are **unaffected** by this change:
- Users who have `server.rbac.log.enforce.enable: "true"` in their `argocd-cm` ConfigMap
- Users who have `policy.default: role:readonly` or `policy.default: role:admin` in their `argocd-rbac-cm` ConfigMap

The following users are **affected** and should perform remediation:
- Users who don't have a `policy.default` in their `argocd-rbac-cm` ConfigMap
- Users who have `server.rbac.log.enforce.enable` set to `false` or don't have this setting at all in their `argocd-cm` ConfigMap

### Remediation Steps

1. **Quick Remediation:**
   - Add logs permissions to existing roles
   - Example: `p, role:existing-role, logs, get, */*, allow`

2. **Recommended Remediation:**
   - Review existing roles and their permissions
   - Add logs permissions only to roles that need them
   - Consider creating a dedicated log viewer role
   - Define your own RBAC policies as the operator does not provide defaults
   - Remove the `server.rbac.log.enforce.enable` setting from `argocd-cm` ConfigMap if it was present before the upgrade

### RBAC with Dex SSO Authentication

If you're upgrading from Argo CD 2.x to 3.0+ and using Dex SSO, you need to update your RBAC policies to maintain the same access levels.

#### Detection

The following users are **affected** by this change:
- Users who have Dex SSO configured with custom RBAC policies
- Users who reference Dex `sub` claims in their RBAC policies
- Users who have user-specific permissions based on Dex authentication

The following users are **unaffected** by this change:
- Users who don't use Dex SSO
- Users who only use group-based RBAC policies
- Users who use other SSO providers (Keycloak, OIDC, etc.)

#### Remediation Steps

1. **Quick Remediation:**
   - Decode existing `sub` claims in your policies
   - Replace `sub` claim values with the decoded `user_id`
   - Example: Replace `ChdleGFtcGxlQGFyZ29wcm9qLmlvEgJkZXhfY29ubl9pZA` with `example@argoproj.io`

2. **Recommended Remediation:**
   - Audit all existing RBAC policies for Dex `sub` claim references
   - Decode each `sub` claim to identify the actual user ID
   - Update policies to use the `federated_claims.user_id` format
   - Test authentication and authorization after the changes
   - Consider using group-based policies instead of user-specific ones for better maintainability

#### CLI Authentication

If you're using the Argo CD CLI with Dex authentication, make sure to use the new Argo CD version to obtain an authentication token with the appropriate claims. The CLI will automatically handle the new authentication flow.

#### Best Practices

1. **Use Group-Based Policies**: Instead of user-specific policies, consider using group-based policies for better maintainability
2. **Document User Mappings**: Keep a record of the decoded user IDs for future reference
3. **Test Thoroughly**: Verify that all users maintain their expected access levels after the migration
4. **Monitor Authentication**: Watch for authentication issues during and after the migration

#### Example Migration Workflow

1. **Identify affected policies:**
```bash
kubectl get cm argocd-rbac-cm -n argocd -o=jsonpath='{.data.policy\.csv}'
```

2. **Decode sub claims:**
```bash
echo "YOUR_SUB_CLAIM_HERE" | base64 -d
```

3. **Update policies:**
```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: rbac-migration
spec:
  rbac:
    policy: |
      # Old: g, ChdleGFtcGxlQGFyZ29wcm9qLmlvEgJkZXhfY29ubl9pZA, role:example
      # New: g, example@argoproj.io, role:example
      g, example@argoproj.io, role:example
```

4. **Test authentication:**
```bash
argocd login <your-argocd-server> --sso
argocd app list
```

## Declarative webhook secrets (`spec.webhookSecrets`)

The Argo CD Operator can populate Argo CD’s Git webhook credentials from Kubernetes `Secret`
references declared on **`spec.webhookSecrets`** in the **ArgoCD** CR (`v1beta1`). This is optional and **backward compatible**:

- **If you do not set** `spec.webhookSecrets`, the operator continues to omit declarative webhook management; **`webhook.*` keys already present in `argocd-secret` are left as-is**, including values you patched in manually before this feature existed.
- **If you set** `spec.webhookSecrets`, the operator syncs the providers you declare into `argocd-secret`. Providers not listed while management is enabled can have their **`webhook.*` keys cleared** on reconcile—see [Configuring webhook secrets](./usage/webhook-secrets.md) for exact semantics.

For migration from manual edits, verification, integrations (External Secrets, Sealed Secrets), and troubleshooting, use the **[Configuring webhook secrets](./usage/webhook-secrets.md)** guide. A runnable example can be found at <https://github.com/argoproj-labs/argocd-operator/blob/master/examples/argocd-webhook-secrets.yaml>.