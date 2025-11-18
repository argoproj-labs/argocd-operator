# Notifications in Any Namespace

Argo CD supports managing notification configurations in namespaces other than the control plane's namespace (typically `argocd`). This allows teams to manage their own notification settings for their Argo CD applications without requiring administrator intervention.

To manage notification configurations in non-control plane namespaces, you must satisfy the following prerequisites:

1. The Argo CD instance should be cluster-scoped
2. [Apps in Any Namespace](./apps-in-any-namespace.md) should be enabled on target namespaces
3. Notifications controller must be enabled

## Enable Notifications in a namespace

To enable this feature in a namespace, add the namespace name under `.spec.notifications.sourceNamespaces` field in ArgoCD CR.

For example, following configuration will allow `example` Argo CD instance to manage notification configurations in `foo` namespace.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  sourceNamespaces:
    - foo
  notifications:
    enabled: true
    sourceNamespaces:
      - foo
```

As of now, wildcards are not supported in `.spec.notifications.sourceNamespaces`.

!!! important
Ensure that [Apps in Any Namespace](./apps-in-any-namespace.md) is enabled on target namespace i.e., the target namespace name is part of `.spec.sourceNamespaces` field in ArgoCD CR.

For notifications to work correctly in a specific namespace, you **must** add that namespace to **both**:
    
- `spec.sourceNamespaces` (for applications)
- `spec.notifications.sourceNamespaces` (for notifications)

If either is missing, notifications will not function as expected in that namespace. The operator will skip creating notification resources for any namespace in `spec.notifications.sourceNamespaces` that is not also present in `spec.sourceNamespaces`.

The Operator creates/modifies below RBAC resources when Notifications in Any Namespace is enabled:

|Name|Kind|Purpose|
|:-|:-|:-|
|`<argoCDName-argoCDNamespace>-notifications`|Role & RoleBinding|For notifications controller to read ConfigMaps and Secrets in target namespace|

Additionally, it adds `argocd.argoproj.io/notifications-managed-by-cluster-argocd` label to the target namespace.

The operator also automatically creates a `NotificationsConfiguration` custom resource named `default-notifications-configuration` in each delegated namespace. This provides a consistent UX where users manage the `argocd-notifications-cm` ConfigMap through the NotificationsConfiguration CR instead of editing the ConfigMap directly, just like in the main Argo CD namespace.

## Configuration Resolution

The notifications controller uses a fallback mechanism to resolve notification configuration:

1. **First**: It checks the application's own namespace for notification settings (`argocd-notifications-cm` ConfigMap and `argocd-notifications-secret` Secret).
2. **Fallback**: If not found, it falls back to the central configuration in the main Argo CD namespace.

This allows teams to override global notification settings with namespace-specific configurations when needed, while still benefiting from centralized defaults.

## Managing Notification Configuration

The operator automatically creates a `NotificationsConfiguration` custom resource named `default-notifications-configuration` in each delegated namespace. Teams should update this resource to manage their notification configuration:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: NotificationsConfiguration
metadata:
  name: default-notifications-configuration
  namespace: foo
spec:
  templates:
    template.app-sync-status: |
      email:
        subject: Application {{.app.metadata.name}} sync status
      message: |
        Application {{.app.metadata.name}} is now {{.app.status.sync.status}}.
        Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.
  triggers:
    trigger.on-sync-status-unknown: |
      - description: Application {{.app.metadata.name}} sync status is 'Unknown'
        send:
        - app-sync-status
        when: app.status.sync.status == 'Unknown'
  services:
    service.email.example: |
      username: $email-username
      password: $email-password
      host: smtp.gmail.com
      port: 465
      from: noreply@example.com
```

The NotificationsConfiguration controller will automatically reconcile the `argocd-notifications-cm` ConfigMap based on the NotificationsConfiguration resource.

For more details on using NotificationsConfiguration, see the [NotificationsConfiguration reference](../reference/notificationsconfiguration.md).

### Things to consider

Only one of either `managed-by` or `notifications-managed-by-cluster-argocd` labels can be applied to a given namespace. We will be prioritizing `managed-by` label in case of a conflict as this feature is currently in beta, so the new roles/rolebindings will not be created if namespace is already labelled with `managed-by` label, and they will be deleted if a namespace is first added to the `.spec.notifications.sourceNamespaces` list and is later also labelled with `managed-by` label.

