package notifications

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/go-logr/logr"
	"golang.org/x/mod/semver"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetDefaultNotificationsConfig returns a map that contains default triggers and template configurations for argocd-notifications-cm
func GetDefaultNotificationsConfig() map[string]string {

	notificationsConfig := make(map[string]string)

	// configure default notifications templates

	notificationsConfig["template.app-created"] = `email:
  subject: Application {{.app.metadata.name}} has been created.
message: Application {{.app.metadata.name}} has been created.
teams:
  title: Application {{.app.metadata.name}} has been created.`

	notificationsConfig["template.app-deleted"] = `email:
  subject: Application {{.app.metadata.name}} has been deleted.
message: Application {{.app.metadata.name}} has been deleted.
teams:
  title: Application {{.app.metadata.name}} has been deleted.`

	notificationsConfig["template.app-deployed"] = `email:
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

	notificationsConfig["template.app-health-degraded"] = `email:
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

	notificationsConfig["template.app-sync-failed"] = `email:
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

	notificationsConfig["template.app-sync-running"] = `email:
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

	notificationsConfig["template.app-sync-status-unknown"] = `email:
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

	notificationsConfig["template.app-sync-succeeded"] = `email:
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

	// configure default notifications triggers

	notificationsConfig["trigger.on-created"] = `- description: Application is created.
  oncePer: app.metadata.name
  send:
  - app-created
  when: "true"`

	notificationsConfig["trigger.on-deleted"] = `- description: Application is deleted.
  oncePer: app.metadata.name
  send:
  - app-deleted
  when: app.metadata.deletionTimestamp != nil`

	notificationsConfig["trigger.on-deployed"] = `- description: Application is synced and healthy. Triggered once per commit.
  oncePer: app.status.operationState.syncResult.revision
  send:
  - app-deployed
  when: app.status.operationState.phase in ['Succeeded'] and app.status.health.status
      == 'Healthy'`

	notificationsConfig["trigger.on-health-degraded"] = `- description: Application has degraded
  send:
  - app-health-degraded
  when: app.status.health.status == 'Degraded'`

	notificationsConfig["trigger.on-sync-failed"] = `- description: Application syncing has failed
  send:
  - app-sync-failed
  when: app.status.operationState.phase in ['Error', 'Failed']`

	notificationsConfig["trigger.on-sync-running"] = `- description: Application is being synced
  send:
  - app-sync-running
  when: app.status.operationState.phase in ['Running']`

	notificationsConfig["trigger.on-sync-status-unknown"] = `- description: Application status is 'Unknown'
  send:
  - app-sync-status-unknown
  when: app.status.sync.status == 'Unknown'`

	notificationsConfig["trigger.on-sync-succeeded"] = `- description: Application syncing has succeeded
  send:
  - app-sync-succeeded
  when: app.status.operationState.phase in ['Succeeded']`

	return notificationsConfig
}

func GetNotificationsCommand(cr *argoprojv1a1.ArgoCD) []string {

	cmd := make([]string, 0)
	cmd = append(cmd, "argocd-notifications")

	cmd = append(cmd, "--loglevel")
	cmd = append(cmd, util.GetLogLevel(cr.Spec.Notifications.LogLevel))

	return cmd
}

// GetNotificationsResources will return the ResourceRequirements for the Notifications container.
func GetNotificationsResources(cr *argoprojv1a1.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Notifications.Resources != nil {
		resources = *cr.Spec.Notifications.Resources
	}

	return resources
}

func GetArgoCDNotificationsControllerReplicas(nr *NotificationsReconciler) *int32 {
	if nr.Instance.Spec.Notifications.Replicas != nil && *nr.Instance.Spec.Notifications.Replicas >= 0 {
		return nr.Instance.Spec.Notifications.Replicas
	}

	return nil
}

func UpdateIfChanged(existingVal, desiredVal interface{}, extraAction func(), changed *bool) {
	if !reflect.DeepEqual(existingVal, desiredVal) {
		reflect.ValueOf(existingVal).Elem().Set(reflect.ValueOf(desiredVal).Elem())
		if extraAction != nil {
			extraAction()
		}
		*changed = true
	}
}

// TODO: The below functions should be moved to some sharing util package.
func GetArgoContainerImage(cr *argoprojv1a1.ArgoCD) string {
	defaultTag, defaultImg := false, false
	img := cr.Spec.Image
	if img == "" {
		img = common.ArgoCDDefaultArgoImage
		defaultImg = true
	}

	tag := cr.Spec.Version
	if tag == "" {
		tag = common.ArgoCDDefaultArgoVersion
		defaultTag = true
	}
	if e := os.Getenv(common.ArgoCDImageEnvVar); e != "" && (defaultTag && defaultImg) {
		return e
	}

	return util.CombineImageTag(img, tag)
}

func ProxyEnvVars(vars ...corev1.EnvVar) []corev1.EnvVar {
	result := []corev1.EnvVar{}
	result = append(result, vars...)
	proxyKeys := []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"}
	for _, p := range proxyKeys {
		if k, v := caseInsensitiveGetenv(p); k != "" {
			result = append(result, corev1.EnvVar{Name: k, Value: v})
		}
	}
	return result
}

func caseInsensitiveGetenv(s string) (string, string) {
	if v := os.Getenv(s); v != "" {
		return s, v
	}
	ls := strings.ToLower(s)
	if v := os.Getenv(ls); v != "" {
		return ls, v
	}
	return "", ""
}

func AddSeccompProfileForOpenShift(log logr.Logger, client client.Client, podspec *corev1.PodSpec) {
	if !cluster.IsVersionAPIAvailable() {
		return
	}
	version, err := cluster.GetClusterVersion(client)
	if err != nil {
		log.Error(err, "couldn't get OpenShift version")
	}
	if version == "" || semver.Compare(fmt.Sprintf("v%s", version), "v4.10.999") > 0 {
		if podspec.SecurityContext == nil {
			podspec.SecurityContext = &corev1.PodSecurityContext{}
		}
		if podspec.SecurityContext.SeccompProfile == nil {
			podspec.SecurityContext.SeccompProfile = &corev1.SeccompProfile{}
		}
		if len(podspec.SecurityContext.SeccompProfile.Type) == 0 {
			podspec.SecurityContext.SeccompProfile.Type = corev1.SeccompProfileTypeRuntimeDefault
		}
	}
}
