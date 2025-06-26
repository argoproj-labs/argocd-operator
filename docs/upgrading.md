# Upgrading

This page contains upgrade instructions and migration guides for the Argo CD Operator.

#### Upgrading from Operator ≤0.14 (Argo CD ≤2.14) to Operator 0.15+ (Argo CD 3.0+)

## Logs RBAC Enforcement

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

## RBAC with Dex SSO Authentication

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