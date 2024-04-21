package notifications

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
)

// getDefaultNotificationsContext returns an empty map for context
func getDefaultNotificationsContext() map[string]string {
	notificationContext := make(map[string]string)
	return notificationContext
}

// getDefaultNotificationsTemplates returns a map that contains default template configurations
func getDefaultNotificationsTemplates() map[string]string {

	notificationsTempaltes := make(map[string]string)

	// configure default notifications templates

	notificationsTempaltes["template.app-created"] = `email:
  subject: Application {{.app.metadata.name}} has been created.
message: Application {{.app.metadata.name}} has been created.
teams:
  title: Application {{.app.metadata.name}} has been created.`

	notificationsTempaltes["template.app-deleted"] = `email:
  subject: Application {{.app.metadata.name}} has been deleted.
message: Application {{.app.metadata.name}} has been deleted.
teams:
  title: Application {{.app.metadata.name}} has been deleted.`

	notificationsTempaltes["template.app-deployed"] = `email:
  subject: New version of an application {{.app.metadata.name}} is up and running.
message: |
  {{if eq .serviceType "slack"}}:white_check_mark:{{end}} Application {{.app.metadata.name}} is now running new version of deployments manifests.
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#18be52",
      "fields": [
      {
        "title": "Sync Status",
        "value": "{{.app.status.sync.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      },
      {
        "title": "Revision",
        "value": "{{.app.status.sync.revision}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Sync Status",
      "value": "{{.app.status.sync.status}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    },
    {
      "name": "Revision",
      "value": "{{.app.status.sync.revision}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |-
    [{
      "@type":"OpenUri",
      "name":"Operation Application",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  themeColor: '#000080'
  title: New version of an application {{.app.metadata.name}} is up and running.`

	notificationsTempaltes["template.app-health-degraded"] = `email:
  subject: Application {{.app.metadata.name}} has degraded.
message: |
  {{if eq .serviceType "slack"}}:exclamation:{{end}} Application {{.app.metadata.name}} has degraded.
  Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link": "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#f4c030",
      "fields": [
      {
        "title": "Health Status",
        "value": "{{.app.status.health.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Health Status",
      "value": "{{.app.status.health.status}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |
    [{
      "@type":"OpenUri",
      "name":"Open Application",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  themeColor: '#FF0000'
  title: Application {{.app.metadata.name}} has degraded.`

	notificationsTempaltes["template.app-sync-failed"] = `email:
  subject: Failed to sync application {{.app.metadata.name}}.
message: |
  {{if eq .serviceType "slack"}}:exclamation:{{end}}  The sync operation of application {{.app.metadata.name}} has failed at {{.app.status.operationState.finishedAt}} with the following error: {{.app.status.operationState.message}}
  Sync operation details are available at: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true .
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#E96D76",
      "fields": [
      {
        "title": "Sync Status",
        "value": "{{.app.status.sync.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Sync Status",
      "value": "{{.app.status.sync.status}}"
    },
    {
      "name": "Failed at",
      "value": "{{.app.status.operationState.finishedAt}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |-
    [{
      "@type":"OpenUri",
      "name":"Open Operation",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  themeColor: '#FF0000'
  title: Failed to sync application {{.app.metadata.name}}.`

	notificationsTempaltes["template.app-sync-running"] = `email:
  subject: Start syncing application {{.app.metadata.name}}.
message: |
  The sync operation of application {{.app.metadata.name}} has started at {{.app.status.operationState.startedAt}}.
  Sync operation details are available at: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true .
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#0DADEA",
      "fields": [
      {
        "title": "Sync Status",
        "value": "{{.app.status.sync.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Sync Status",
      "value": "{{.app.status.sync.status}}"
    },
    {
      "name": "Started at",
      "value": "{{.app.status.operationState.startedAt}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |-
    [{
      "@type":"OpenUri",
      "name":"Open Operation",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  title: Start syncing application {{.app.metadata.name}}.`

	notificationsTempaltes["template.app-sync-status-unknown"] = `email:
  subject: Application {{.app.metadata.name}} sync status is 'Unknown'
message: |
  {{if eq .serviceType "slack"}}:exclamation:{{end}} Application {{.app.metadata.name}} sync is 'Unknown'.
  Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.
  {{if ne .serviceType "slack"}}
  {{range $c := .app.status.conditions}}
      * {{$c.message}}
  {{end}}
  {{end}}
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#E96D76",
      "fields": [
      {
        "title": "Sync Status",
        "value": "{{.app.status.sync.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Sync Status",
      "value": "{{.app.status.sync.status}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |-
    [{
      "@type":"OpenUri",
      "name":"Open Application",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  title: Application {{.app.metadata.name}} sync status is 'Unknown'`

	notificationsTempaltes["template.app-sync-succeeded"] = `email:
  subject: Application {{.app.metadata.name}} has been successfully synced.
message: |
  {{if eq .serviceType "slack"}}:white_check_mark:{{end}} Application {{.app.metadata.name}} has been successfully synced at {{.app.status.operationState.finishedAt}}.
  Sync operation details are available at: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true .
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#18be52",
      "fields": [
      {
        "title": "Sync Status",
        "value": "{{.app.status.sync.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Sync Status",
      "value": "{{.app.status.sync.status}}"
    },
    {
      "name": "Synced at",
      "value": "{{.app.status.operationState.finishedAt}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |-
    [{
      "@type":"OpenUri",
      "name":"Operation Details",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  themeColor: '#000080'
  title: Application {{.app.metadata.name}} has been successfully synced`

	return notificationsTempaltes
}

// getDefaultNotificationsTriggers returns a map that contains default triggers configurations
func getDefaultNotificationsTriggers() map[string]string {

	notificationsTriggers := make(map[string]string)

	// configure default notifications triggers

	notificationsTriggers["trigger.on-created"] = `- description: Application is created.
  oncePer: app.metadata.name
  send:
  - app-created
  when: "true"`

	notificationsTriggers["trigger.on-deleted"] = `- description: Application is deleted.
  oncePer: app.metadata.name
  send:
  - app-deleted
  when: app.metadata.deletionTimestamp != nil`

	notificationsTriggers["trigger.on-deployed"] = `- description: Application is synced and healthy. Triggered once per commit.
  oncePer: app.status.operationState.syncResult.revision
  send:
  - app-deployed
  when: app.status.operationState.phase in ['Succeeded'] and app.status.health.status
      == 'Healthy'`

	notificationsTriggers["trigger.on-health-degraded"] = `- description: Application has degraded
  send:
  - app-health-degraded
  when: app.status.health.status == 'Degraded'`

	notificationsTriggers["trigger.on-sync-failed"] = `- description: Application syncing has failed
  send:
  - app-sync-failed
  when: app.status.operationState.phase in ['Error', 'Failed']`

	notificationsTriggers["trigger.on-sync-running"] = `- description: Application is being synced
  send:
  - app-sync-running
  when: app.status.operationState.phase in ['Running']`

	notificationsTriggers["trigger.on-sync-status-unknown"] = `- description: Application status is 'Unknown'
  send:
  - app-sync-status-unknown
  when: app.status.sync.status == 'Unknown'`

	notificationsTriggers["trigger.on-sync-succeeded"] = `- description: Application syncing has succeeded
  send:
  - app-sync-succeeded
  when: app.status.operationState.phase in ['Succeeded']`

	return notificationsTriggers

}

func (nr *NotificationsReconciler) GetNotificationsCommand() []string {

	cmd := make([]string, 0)
	cmd = append(cmd, "argocd-notifications")

	cmd = append(cmd, "--loglevel")
	cmd = append(cmd, argoutil.GetLogLevel(nr.Instance.Spec.Notifications.LogLevel))

	// TO DO: fix this after repo-server is implemented w/ interfaces

	// if nr.Instance.Spec.Repo.IsEnabled() {
	// 	cmd = append(cmd, "--argocd-repo-server", getRepoServerAddress(cr))
	// } else {
	// 	log.Info("Repo Server is disabled. This would affect the functioning of Notification Controller.")
	// }

	return cmd
}

// GetNotificationsResources will return the ResourceRequirements for the Notifications container.
func (nr *NotificationsReconciler) GetNotificationsResources() corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if nr.Instance.Spec.Notifications.Resources != nil {
		resources = *nr.Instance.Spec.Notifications.Resources
	}

	return resources
}

func (nr *NotificationsReconciler) GetArgoCDNotificationsControllerReplicas() *int32 {
	if nr.Instance.Spec.Notifications.Replicas != nil && *nr.Instance.Spec.Notifications.Replicas >= 0 {
		return nr.Instance.Spec.Notifications.Replicas
	}

	return nil
}
