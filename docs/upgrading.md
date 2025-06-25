# Upgrading

This page contains upgrade instructions and migration guides for the Argo CD Operator.

## Upgrading from Operator ≤0.14 (Argo CD ≤2.14) to Operator 0.15+ (Argo CD 3.0+)

If you're upgrading from Argo CD 2.14 to 3.0, note the following changes:

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

