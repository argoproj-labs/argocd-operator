package argocd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (

	// desired exposed metrics for argo-cd instance out of the box
	desiredDefaultMetricsResponse string = `# HELP argocd_application_controller_status Describes the status of the application controller workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']
# TYPE argocd_application_controller_status gauge
argocd_application_controller_status{namespace="argocd"} 3
# HELP argocd_applicationset_controller_status Describes the status of the applicationSet controller workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']
# TYPE argocd_applicationset_controller_status gauge
argocd_applicationset_controller_status{namespace="argocd"} 0
# HELP argocd_dex_status Describes the status of the dex workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']
# TYPE argocd_dex_status gauge
argocd_dex_status{namespace="argocd"} 0
# HELP argocd_phase Describes the phase of argo-cd instance [2='Pending', 4='Available']
# TYPE argocd_phase gauge
argocd_phase{namespace="argocd"} 4
# HELP argocd_redis_status Describes the status of the redis workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']
# TYPE argocd_redis_status gauge
argocd_redis_status{namespace="argocd"} 3
# HELP argocd_repo_server_status Describes the status of the repo server workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']
# TYPE argocd_repo_server_status gauge
argocd_repo_server_status{namespace="argocd"} 3
# HELP argocd_server_status Describes the status of the argo-cd server workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']
# TYPE argocd_server_status gauge
argocd_server_status{namespace="argocd"} 3`

	// desired exposed metrics when notifications-controller is enbaled and appset has invalid image
	desiredMetricsResponse = `# HELP argocd_application_controller_status Describes the status of the application controller workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']
# TYPE argocd_application_controller_status gauge
argocd_application_controller_status{namespace="argocd"} 3
# HELP argocd_applicationset_controller_status Describes the status of the applicationSet controller workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']
# TYPE argocd_applicationset_controller_status gauge
argocd_applicationset_controller_status{namespace="argocd"} 2
# HELP argocd_dex_status Describes the status of the dex workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']
# TYPE argocd_dex_status gauge
argocd_dex_status{namespace="argocd"} 0
# HELP argocd_notifications_controller_status Describes the status of the notifications controller workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']
# TYPE argocd_notifications_controller_status gauge
argocd_notifications_controller_status{namespace="argocd"} 3
# HELP argocd_phase Describes the phase of argo-cd instance [2='Pending', 4='Available']
# TYPE argocd_phase gauge
argocd_phase{namespace="argocd"} 4
# HELP argocd_redis_status Describes the status of the redis workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']
# TYPE argocd_redis_status gauge
argocd_redis_status{namespace="argocd"} 3
# HELP argocd_repo_server_status Describes the status of the repo server workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']
# TYPE argocd_repo_server_status gauge
argocd_repo_server_status{namespace="argocd"} 3
# HELP argocd_server_status Describes the status of the argo-cd server workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']
# TYPE argocd_server_status gauge
argocd_server_status{namespace="argocd"} 3`
)

var (
	desiredReplicas = int32(1)
)

type testDeploymentSetting struct {
	name             string
	deployedReplicas int32
}

func getTestDeployment(name string, deployedReplicas int32) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "argocd",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &desiredReplicas,
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: deployedReplicas,
		},
	}
	return deployment
}

func getTestStatefulSet(name string, deployedReplicas int32) *appsv1.StatefulSet {
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "argocd",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &desiredReplicas,
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas: deployedReplicas,
		},
	}
	return statefulSet
}

func TestInstanceStatusMetrics(t *testing.T) {

	tests := []struct {
		argocd            *argoprojv1alpha1.ArgoCD
		name              string
		description       string
		want              string
		deploymentSetting []testDeploymentSetting
	}{
		{
			name:        "default",
			argocd:      makeTestArgoCD(),
			description: "workload statuses for instance out of the box",
			want:        desiredDefaultMetricsResponse,
			deploymentSetting: []testDeploymentSetting{
				{
					name:             "argocd-application-controller",
					deployedReplicas: int32(1),
				},
				{
					name:             "argocd-redis",
					deployedReplicas: int32(1),
				},
				{
					name:             "argocd-repo-server",
					deployedReplicas: int32(1),
				},
				{
					name:             "argocd-server",
					deployedReplicas: int32(1),
				},
			},
		},
		{
			name: "modified",
			argocd: makeTestArgoCD(func(ac *argoprojv1alpha1.ArgoCD) {
				ac.Spec.Notifications.Enabled = true
			}),
			description: "workload statuses for modified instance: notifications-controller is enbaled and appset is pending",
			want:        desiredMetricsResponse,
			deploymentSetting: []testDeploymentSetting{
				{
					name:             "argocd-application-controller",
					deployedReplicas: int32(1),
				},
				{
					name:             "argocd-redis",
					deployedReplicas: int32(1),
				},
				{
					name:             "argocd-repo-server",
					deployedReplicas: int32(1),
				},
				{
					name:             "argocd-server",
					deployedReplicas: int32(1),
				},
				{
					name:             "argocd-notifications-controller",
					deployedReplicas: int32(1),
				},
				{
					name:             "argocd-applicationset-controller",
					deployedReplicas: int32(0),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := makeTestReconciler(t)

			r.Client.Create(context.TODO(), test.argocd)
			r.Client.Create(context.TODO(), getTestStatefulSet(test.deploymentSetting[0].name, test.deploymentSetting[0].deployedReplicas))

			for _, setting := range test.deploymentSetting[1:] {
				r.Client.Create(context.TODO(), getTestDeployment(setting.name, setting.deployedReplicas))
			}

			sm := http.NewServeMux()
			sm.Handle(MetricsPath, promhttp.Handler())

			err := r.reconcileStatusApplicationController(test.argocd)
			assert.NoError(t, err)
			err = r.reconcileStatusDex(test.argocd)
			assert.NoError(t, err)
			err = r.reconcileStatusRedis(test.argocd)
			assert.NoError(t, err)
			err = r.reconcileStatusRepo(test.argocd)
			assert.NoError(t, err)
			err = r.reconcileStatusServer(test.argocd)
			assert.NoError(t, err)
			err = r.reconcileStatusNotifications(test.argocd)
			assert.NoError(t, err)
			err = r.reconcileStatusApplicationSetController(test.argocd)
			assert.NoError(t, err)

			err = r.reconcileStatusPhase(test.argocd)
			assert.NoError(t, err)

			req, err := http.NewRequest("GET", "/metrics", nil)
			assert.NoError(t, err)
			rr := httptest.NewRecorder()

			testMetricsServer := &http.Server{
				Addr:    "localhost:8089",
				Handler: sm,
			}
			testMetricsServer.Handler.ServeHTTP(rr, req)
			assert.Equal(t, rr.Code, http.StatusOK)
			body := rr.Body.String()

			assert.True(t, strings.Contains(body, test.want))
		})
	}
}
