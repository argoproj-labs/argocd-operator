# NotificationsConfiguration

The `NotificationsConfiguration` resource is a Kubernetes Custom Resource (CRD) that allows users to add the triggers, templates, services and subscriptions to the Argo CD Notifications Configmap.

A `NotificationsConfiguration` custom resource with name `default-notifications-configuration` is created **OOTB** with default configuration. Users should update this custom resource with their templates, triggers, services, subscriptios or any other configuration.

**Note:** 
- Any configuration changes should be made to the `default-notifications-configuration` only. At this point, we do not support any custom resources of kind `NotificationsConfiguration` created by the users.
- Any modifications to the `argocd-notifications-cm` will be reconciled back by the `NotificationsConfiguration` controller of the Argo CD operator instance.

The `NotificationsConfiguration` Custom Resource consists of the following properties.

Name | Default | Description
--- | --- | ---
**Templates** | [Empty] | Triggers define the condition when the notification should be sent and list of templates required to generate the message.
**Triggers** | [Empty] | Templates are used to generate the notification template message.
**Services** | [Empty] | Services are used to deliver message.
**Subscriptions** | [Empty] | Subscriptions contain centrally managed global application subscriptions.

## Templates Example

The following example shows how to add templates to the `argocd-notification-cm` using the `default-notifications-configuration` custom resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: NotificationsConfiguration
metadata:
 name: default-notifications-configuration
spec:
 templates:
  template.my-custom-template: |
    message: |
      Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.

```

## Triggers Example

The following example shows how to add Triggers to the `argocd-notification-cm` using the `default-notifications-configuration` custom resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: NotificationsConfiguration
metadata:
 name: default-notifications-configuration
spec:
 triggers:
  trigger.on-sync-status-unknown: |
    - when: app.status.sync.status == 'Unknown'
      send: [my-custom-template]
```

## Services Example

The following example shows how to add Services to the `argocd-notification-cm` using the `default-notifications-configuration` custom resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: NotificationsConfiguration
metadata:
 name: default-notifications-configuration
spec:
 Services:
  service.slack: |
    token: $slack-token
    username: <override-username> # optional username
    icon: <override-icon> # optional icon for the message (supports both emoij and url notation)
```

## Subscriptions Example

The following example shows how to add Subscriptions to the `argocd-notification-cm` using the `default-notifications-configuration` custom resource.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: NotificationsConfiguration
metadata:
 name: default-notifications-configuration
spec:
 Subscriptions: |
  subscriptions: |
    # subscription for on-sync-status-unknown trigger notifications
    - recipients:
      - slack:test2
      - email:test@gmail.com
      triggers:
      - on-sync-status-unknown
    # subscription restricted to applications with matching labels only
    - recipients:
      - slack:test3
      selector: test=true
      triggers:
      - on-sync-status-unknown
```
