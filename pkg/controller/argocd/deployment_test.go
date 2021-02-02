package argocd

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/google/go-cmp/cmp"
)

const (
	testHTTPProxy  = "example.com:8888"
	testHTTPSProxy = "example.com:8443"
	testNoProxy    = ".example.com"
)

var (
	deploymentNames = []string{
		"argocd-repo-server",
		"argocd-application-controller",
		"argocd-dex-server",
		"argocd-grafana",
		"argocd-redis",
		"argocd-server"}
)

func TestReconcileArgoCD_reconcileApplicationControllerDeployment(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	assert.NilError(t, r.reconcileApplicationControllerDeployment(a))

	deployment := &appsv1.Deployment{}
	assert.NilError(t, r.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-application-controller",
			Namespace: a.Namespace,
		},
		deployment))
	command := deployment.Spec.Template.Spec.Containers[0].Command
	want := []string{
		"argocd-application-controller",
		"--operation-processors", "10",
		"--redis", "argocd-redis:6379",
		"--repo-server", "argocd-repo-server:8081",
		"--status-processors", "20"}
	if diff := cmp.Diff(want, command); diff != "" {
		t.Fatalf("reconciliation failed:\n%s", diff)
	}
}

func TestReconcileArgoCD_reconcileApplicationControllerDeployment_withUpdate(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	assert.NilError(t, r.reconcileApplicationControllerDeployment(a))

	a = makeTestArgoCD(controllerProcessors(30))
	assert.NilError(t, r.reconcileApplicationControllerDeployment(a))

	deployment := &appsv1.Deployment{}
	assert.NilError(t, r.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-application-controller",
			Namespace: a.Namespace,
		},
		deployment))
	command := deployment.Spec.Template.Spec.Containers[0].Command
	want := []string{
		"argocd-application-controller",
		"--operation-processors", "10",
		"--redis", "argocd-redis:6379",
		"--repo-server", "argocd-repo-server:8081",
		"--status-processors", "30"}
	if diff := cmp.Diff(want, command); diff != "" {
		t.Fatalf("reconciliation failed:\n%s", diff)
	}
}

func Test_getArgoApplicationControllerCommand(t *testing.T) {
	cmdTests := []struct {
		name string
		opts []argoCDOpt
		want []string
	}{
		{
			"defaults",
			[]argoCDOpt{},
			[]string{
				"argocd-application-controller",
				"--operation-processors",
				"10",
				"--redis",
				"argocd-redis:6379",
				"--repo-server",
				"argocd-repo-server:8081",
				"--status-processors",
				"20",
			},
		},
		{
			"configured status processors",
			[]argoCDOpt{controllerProcessors(30)},
			[]string{
				"argocd-application-controller",
				"--operation-processors",
				"10",
				"--redis",
				"argocd-redis:6379",
				"--repo-server",
				"argocd-repo-server:8081",
				"--status-processors",
				"30",
			},
		},
		{
			"configured operation processors",
			[]argoCDOpt{operationProcessors(15)},
			[]string{
				"argocd-application-controller",
				"--operation-processors",
				"15",
				"--redis",
				"argocd-redis:6379",
				"--repo-server",
				"argocd-repo-server:8081",
				"--status-processors",
				"20",
			},
		},
		{
			"configured appSync",
			[]argoCDOpt{appSync(time.Minute * 10)},
			[]string{
				"argocd-application-controller",
				"--operation-processors",
				"10",
				"--redis",
				"argocd-redis:6379",
				"--repo-server",
				"argocd-repo-server:8081",
				"--status-processors",
				"20",
				"--app-resync",
				"600",
			},
		},
	}

	for _, tt := range cmdTests {
		cr := makeTestArgoCD(tt.opts...)
		cmd := getArgoApplicationControllerCommand(cr)

		if !reflect.DeepEqual(cmd, tt.want) {
			t.Fatalf("got %#v, want %#v", cmd, tt.want)
		}
	}
}

// TODO: This needs more testing for the rest of the RepoDeployment container
// fields.

// reconcileRepoDeployment creates a Deployment with the correct volumes for the
// repo-server.
func TestReconcileArgoCD_reconcileRepoDeployment_volumes(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	err := r.reconcileRepoDeployment(a)
	assert.NilError(t, err)

	deployment := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-repo-server",
		Namespace: testNamespace,
	}, deployment)
	assert.NilError(t, err)

	want := []corev1.Volume{
		{
			Name: "ssh-known-hosts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDKnownHostsConfigMapName,
					},
				},
			},
		},
		{
			Name: "tls-certs",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDTLSCertsConfigMapName,
					},
				},
			},
		},
		{
			Name: "gpg-keyring",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	if diff := cmp.Diff(want, deployment.Spec.Template.Spec.Volumes); diff != "" {
		t.Fatalf("reconcileRepoDeployment failed:\n%s", diff)
	}
}

// reconcileRepoDeployment creates a Deployment with the correct mounts for the
// repo-server.
func TestReconcileArgoCD_reconcileRepoDeployment_mounts(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	err := r.reconcileRepoDeployment(a)
	assert.NilError(t, err)

	deployment := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-repo-server",
		Namespace: testNamespace,
	}, deployment)
	assert.NilError(t, err)

	want := []corev1.VolumeMount{
		{Name: "ssh-known-hosts", MountPath: "/app/config/ssh"},
		{Name: "tls-certs", MountPath: "/app/config/tls"},
		{Name: "gpg-keyring", MountPath: "/app/config/gpg/keys"},
	}

	if diff := cmp.Diff(want, deployment.Spec.Template.Spec.Containers[0].VolumeMounts); diff != "" {
		t.Fatalf("reconcileRepoDeployment failed:\n%s", diff)
	}
}

func TestReconcileArgoCD_reconcileDexDeployment_with_dex_disabled(t *testing.T) {
	restoreEnv(t)
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	os.Setenv("DISABLE_DEX", "true")
	assert.NilError(t, r.reconcileDexDeployment(a))

	deployment := &appsv1.Deployment{}
	assertNotFound(t, r.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-dex-server",
			Namespace: a.Namespace,
		},
		deployment))
}

// When Dex is disabled, the Dex Deployment should be removed.
func TestReconcileArgoCD_reconcileDexDeployment_removes_dex_when_disabled(t *testing.T) {
	restoreEnv(t)
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)
	os.Setenv("DISABLE_DEX", "true")

	assert.NilError(t, r.reconcileDexDeployment(a))

	a = makeTestArgoCD()
	assert.NilError(t, r.reconcileDexDeployment(a))

	deployment := &appsv1.Deployment{}
	assertNotFound(t, r.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-dex-server",
			Namespace: a.Namespace,
		},
		deployment))
}

// reconcileRepoDeployments creates a Deployment with the proxy settings from the
// environment propagated.
func TestReconcileArgoCD_reconcileDeployments_proxy(t *testing.T) {
	restoreEnv(t)
	os.Setenv("HTTP_PROXY", testHTTPProxy)
	os.Setenv("HTTPS_PROXY", testHTTPSProxy)
	os.Setenv("no_proxy", testNoProxy)

	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.Grafana.Enabled = true
	})
	r := makeTestReconciler(t, a)

	err := r.reconcileDeployments(a)
	assert.NilError(t, err)

	for _, v := range deploymentNames {
		assertDeploymentHasProxyVars(t, r.client, v)
	}
}

// reconcileRepoDeployments creates a Deployment with the proxy settings from the
// environment propagated.
//
// If the deployments already exist, they should be updated to reflect the new
// environment variables.
func TestReconcileArgoCD_reconcileDeployments_proxy_update_existing(t *testing.T) {
	restoreEnv(t)
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.Grafana.Enabled = true
	})
	r := makeTestReconciler(t, a)
	err := r.reconcileDeployments(a)
	assert.NilError(t, err)
	for _, v := range deploymentNames {
		refuteDeploymentHasProxyVars(t, r.client, v)
	}

	os.Setenv("HTTP_PROXY", testHTTPProxy)
	os.Setenv("HTTPS_PROXY", testHTTPSProxy)
	os.Setenv("no_proxy", testNoProxy)

	logf.SetLogger(logf.ZapLogger(true))

	err = r.reconcileDeployments(a)
	assert.NilError(t, err)

	for _, v := range deploymentNames {
		assertDeploymentHasProxyVars(t, r.client, v)
	}
}

// TODO: This should be subsumed into testing of the HA setup.
func TestReconcileArgoCD_reconcileDeployments_HA_proxy(t *testing.T) {
	restoreEnv(t)
	os.Setenv("HTTP_PROXY", testHTTPProxy)
	os.Setenv("HTTPS_PROXY", testHTTPSProxy)
	os.Setenv("no_proxy", testNoProxy)

	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.HA.Enabled = true
	})
	r := makeTestReconciler(t, a)

	err := r.reconcileDeployments(a)
	assert.NilError(t, err)

	assertDeploymentHasProxyVars(t, r.client, "argocd-redis-ha-haproxy")
}

func TestReconcileArgoCD_reconcileRepoDeployment_updatesVolumeMounts(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-repo-server",
			Namespace: testNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Command: []string{"testing"},
							Image:   "test-image",
						},
					},
				},
			},
		},
	}
	r := makeTestReconciler(t, a, d)

	err := r.reconcileRepoDeployment(a)
	assert.NilError(t, err)

	deployment := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-repo-server",
		Namespace: testNamespace,
	}, deployment)
	assert.NilError(t, err)

	if l := len(deployment.Spec.Template.Spec.Volumes); l != 3 {
		t.Fatalf("reconcileRepoDeployment volumes, got %d, want 3", l)
	}

	if l := len(deployment.Spec.Template.Spec.Containers[0].VolumeMounts); l != 3 {
		t.Fatalf("reconcileRepoDeployment mounts, got %d, want 3", l)
	}
}

func Test_proxyEnvVars(t *testing.T) {
	restoreEnv(t)
	os.Setenv("HTTP_PROXY", testHTTPProxy)
	os.Setenv("HTTPS_PROXY", testHTTPSProxy)
	os.Setenv("no_proxy", testNoProxy)
	envTests := []struct {
		vars []corev1.EnvVar
		want []corev1.EnvVar
	}{
		{
			vars: []corev1.EnvVar{},
			want: []corev1.EnvVar{
				{Name: "HTTP_PROXY", Value: "example.com:8888"},
				{Name: "HTTPS_PROXY", Value: "example.com:8443"},
				{Name: "no_proxy", Value: ".example.com"},
			},
		},
		{
			vars: []corev1.EnvVar{
				{Name: "TEST_VAR", Value: "testing"},
			},
			want: []corev1.EnvVar{
				{Name: "TEST_VAR", Value: "testing"},
				{Name: "HTTP_PROXY", Value: "example.com:8888"},
				{Name: "HTTPS_PROXY", Value: "example.com:8443"},
				{Name: "no_proxy", Value: ".example.com"},
			},
		},
	}

	for _, tt := range envTests {
		e := proxyEnvVars(tt.vars...)
		if diff := cmp.Diff(tt.want, e); diff != "" {
			t.Errorf("proxyEnvVars(%#v) diff = \n%s", tt.vars, diff)
		}
	}
}

func TestReconcileArgoCD_reconcileDexDeployment(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	assert.NilError(t, r.reconcileDexDeployment(a))

	deployment := &appsv1.Deployment{}
	assert.NilError(t, r.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-dex-server",
			Namespace: a.Namespace,
		},
		deployment))
	want := corev1.PodSpec{
		Volumes: []corev1.Volume{
			{
				Name: "static-files",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
		InitContainers: []corev1.Container{
			{
				Name:  "copyutil",
				Image: getArgoContainerImage(a),
				Command: []string{
					"cp",
					"/usr/local/bin/argocd-util",
					"/shared",
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "static-files",
						MountPath: "/shared",
					},
				},
				ImagePullPolicy: corev1.PullAlways,
			},
		},
		Containers: []corev1.Container{
			{
				Name:  "dex",
				Image: getDexContainerImage(a),
				Command: []string{
					"/shared/argocd-util",
					"rundex",
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "http",
						ContainerPort: 5556,
					},
					{
						Name:          "grpc",
						ContainerPort: 5557,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "static-files", MountPath: "/shared"}},
				ImagePullPolicy: corev1.PullAlways,
			},
		},
		ServiceAccountName: "argocd-argocd-dex-server",
	}

	if diff := cmp.Diff(want, deployment.Spec.Template.Spec); diff != "" {
		t.Fatalf("reconciliation failed:\n%s", diff)
	}
}

func TestReconcileArgoCD_reconcileDexDeployment_withUpdate(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	// Creates the deployment and then changes the CR and rereconciles.
	assert.NilError(t, r.reconcileDexDeployment(a))
	a.Spec.Image = "justatest"
	a.Spec.Version = "latest"
	a.Spec.Dex.Image = "testdex"
	a.Spec.Dex.Version = "v0.0.1"
	assert.NilError(t, r.reconcileDexDeployment(a))

	deployment := &appsv1.Deployment{}
	assert.NilError(t, r.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-dex-server",
			Namespace: a.Namespace,
		},
		deployment))
	want := corev1.PodSpec{
		Volumes: []corev1.Volume{
			{
				Name: "static-files",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
		InitContainers: []corev1.Container{
			{
				Name:  "copyutil",
				Image: "justatest:latest",
				Command: []string{
					"cp",
					"/usr/local/bin/argocd-util",
					"/shared",
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "static-files",
						MountPath: "/shared",
					},
				},
				ImagePullPolicy: corev1.PullAlways,
			},
		},
		Containers: []corev1.Container{
			{
				Name:  "dex",
				Image: "testdex:v0.0.1",
				Command: []string{
					"/shared/argocd-util",
					"rundex",
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "http",
						ContainerPort: 5556,
					},
					{
						Name:          "grpc",
						ContainerPort: 5557,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "static-files", MountPath: "/shared"}},
				ImagePullPolicy: corev1.PullAlways,
			},
		},
		ServiceAccountName: "argocd-argocd-dex-server",
	}

	if diff := cmp.Diff(want, deployment.Spec.Template.Spec); diff != "" {
		t.Fatalf("reconciliation failed:\n%s", diff)
	}
}

func TestReconcileArgoCD_reconcileServerDeployment(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	assert.NilError(t, r.reconcileServerDeployment(a))

	deployment := &appsv1.Deployment{}
	assert.NilError(t, r.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-server",
			Namespace: a.Namespace,
		},
		deployment))
	want := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            "argocd-server",
				Image:           getArgoContainerImage(a),
				ImagePullPolicy: corev1.PullAlways,
				Command: []string{
					"argocd-server",
					"--staticassets",
					"/shared/app",
					"--dex-server",
					"http://argocd-dex-server:5556",
					"--repo-server",
					"argocd-repo-server:8081",
					"--redis",
					"argocd-redis:6379",
				},
				Ports: []corev1.ContainerPort{
					{ContainerPort: 8080},
					{ContainerPort: 8083},
				},
				LivenessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 3,
					PeriodSeconds:       30,
				},
				ReadinessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 3,
					PeriodSeconds:       30,
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "ssh-known-hosts",
						MountPath: "/app/config/ssh",
					}, {
						Name:      "tls-certs",
						MountPath: "/app/config/tls",
					},
				},
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: "ssh-known-hosts",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: common.ArgoCDKnownHostsConfigMapName,
						},
					},
				},
			}, {
				Name: "tls-certs",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: common.ArgoCDTLSCertsConfigMapName,
						},
					},
				},
			},
		},
		ServiceAccountName: "argocd-argocd-server",
	}

	if diff := cmp.Diff(want, deployment.Spec.Template.Spec); diff != "" {
		t.Fatalf("failed to reconcile argocd-server deployment:\n%s", diff)
	}
}

func TestReconcileArgoCD_reconcileServerDeploymentWithInsecure(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.Server.Insecure = true
	})
	r := makeTestReconciler(t, a)

	assert.NilError(t, r.reconcileServerDeployment(a))

	deployment := &appsv1.Deployment{}
	assert.NilError(t, r.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-server",
			Namespace: a.Namespace,
		},
		deployment))
	want := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            "argocd-server",
				Image:           getArgoContainerImage(a),
				ImagePullPolicy: corev1.PullAlways,
				Command: []string{
					"argocd-server",
					"--insecure",
					"--staticassets",
					"/shared/app",
					"--dex-server",
					"http://argocd-dex-server:5556",
					"--repo-server",
					"argocd-repo-server:8081",
					"--redis",
					"argocd-redis:6379",
				},
				Ports: []corev1.ContainerPort{
					{ContainerPort: 8080},
					{ContainerPort: 8083},
				},
				LivenessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 3,
					PeriodSeconds:       30,
				},
				ReadinessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 3,
					PeriodSeconds:       30,
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "ssh-known-hosts",
						MountPath: "/app/config/ssh",
					}, {
						Name:      "tls-certs",
						MountPath: "/app/config/tls",
					},
				},
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: "ssh-known-hosts",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: common.ArgoCDKnownHostsConfigMapName,
						},
					},
				},
			}, {
				Name: "tls-certs",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: common.ArgoCDTLSCertsConfigMapName,
						},
					},
				},
			},
		},
		ServiceAccountName: "argocd-argocd-server",
	}

	if diff := cmp.Diff(want, deployment.Spec.Template.Spec); diff != "" {
		t.Fatalf("failed to reconcile argocd-server deployment:\n%s", diff)
	}
}

func TestReconcileArgoCD_reconcileServerDeploymentChangedToInsecure(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	assert.NilError(t, r.reconcileServerDeployment(a))

	a = makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.Server.Insecure = true
	})
	assert.NilError(t, r.reconcileServerDeployment(a))

	deployment := &appsv1.Deployment{}
	assert.NilError(t, r.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-server",
			Namespace: a.Namespace,
		},
		deployment))
	want := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            "argocd-server",
				Image:           getArgoContainerImage(a),
				ImagePullPolicy: corev1.PullAlways,
				Command: []string{
					"argocd-server",
					"--insecure",
					"--staticassets",
					"/shared/app",
					"--dex-server",
					"http://argocd-dex-server:5556",
					"--repo-server",
					"argocd-repo-server:8081",
					"--redis",
					"argocd-redis:6379",
				},
				Ports: []corev1.ContainerPort{
					{ContainerPort: 8080},
					{ContainerPort: 8083},
				},
				LivenessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 3,
					PeriodSeconds:       30,
				},
				ReadinessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 3,
					PeriodSeconds:       30,
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "ssh-known-hosts",
						MountPath: "/app/config/ssh",
					}, {
						Name:      "tls-certs",
						MountPath: "/app/config/tls",
					},
				},
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: "ssh-known-hosts",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: common.ArgoCDKnownHostsConfigMapName,
						},
					},
				},
			}, {
				Name: "tls-certs",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: common.ArgoCDTLSCertsConfigMapName,
						},
					},
				},
			},
		},
		ServiceAccountName: "argocd-argocd-server",
	}

	if diff := cmp.Diff(want, deployment.Spec.Template.Spec); diff != "" {
		t.Fatalf("failed to reconcile argocd-server deployment:\n%s", diff)
	}
}

func TestReconcileArgoCD_reconcileRedisDeployment(t *testing.T) {
	// tests reconciler hook for redis deployment
	cr := makeTestArgoCD()
	r := makeTestReconciler(t, cr)

	defer resetHooks()()
	Register(testDeploymentHook)

	assert.NilError(t, r.reconcileRedisDeployment(cr))
	d := &appsv1.Deployment{}
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-redis", Namespace: cr.Namespace}, d))
	assert.DeepEqual(t, int32(3), *d.Spec.Replicas)
}

func TestReconcileArgoCD_reconcileRedisDeployment_with_error(t *testing.T) {
	// tests reconciler hook for redis deployment
	cr := makeTestArgoCD()
	r := makeTestReconciler(t, cr)

	defer resetHooks()()
	Register(testErrorHook)

	assert.Error(t, r.reconcileRedisDeployment(cr), "this is a test error")
}

func restoreEnv(t *testing.T) {
	keys := []string{
		"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY",
		"http_proxy", "https_proxy", "no_proxy",
		"DISABLE_DEX"}
	env := map[string]string{}
	for _, v := range keys {
		env[v] = os.Getenv(v)
	}
	t.Cleanup(func() {
		for k, v := range env {
			os.Setenv(k, v)
		}
	})
}

func operationProcessors(n int32) argoCDOpt {
	return func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.Controller.Processors.Operation = n
	}
}

func appSync(d time.Duration) argoCDOpt {
	return func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.Controller.AppSync = &metav1.Duration{Duration: d}
	}
}

func assertDeploymentHasProxyVars(t *testing.T, c client.Client, name string) {
	t.Helper()
	deployment := &appsv1.Deployment{}
	err := c.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: testNamespace,
	}, deployment)
	assert.NilError(t, err)

	want := []corev1.EnvVar{
		{Name: "HTTP_PROXY", Value: testHTTPProxy},
		{Name: "HTTPS_PROXY", Value: testHTTPSProxy},
		{Name: "no_proxy", Value: testNoProxy},
	}
	for _, c := range deployment.Spec.Template.Spec.Containers {
		if diff := cmp.Diff(want, c.Env); diff != "" {
			t.Errorf("deployment proxy configuration failed for container %v in deployment %q:\n%s", c, name, diff)
		}
	}
	for _, c := range deployment.Spec.Template.Spec.InitContainers {
		if diff := cmp.Diff(want, c.Env); diff != "" {
			t.Errorf("deployment proxy configuration failed for init-container %v in deployment %q:\n%s", c, name, diff)
		}
	}
}

func refuteDeploymentHasProxyVars(t *testing.T, c client.Client, name string) {
	t.Helper()
	deployment := &appsv1.Deployment{}
	err := c.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: testNamespace,
	}, deployment)
	assert.NilError(t, err)

	names := []string{"http_proxy", "https_proxy", "no_proxy"}
	for _, name := range names {
		for _, c := range deployment.Spec.Template.Spec.Containers {
			for _, envVar := range c.Env {
				if strings.ToLower(envVar.Name) == name {
					t.Errorf("deployment proxy configuration failed for container %q, config var %q = %q", c.Name, envVar.Name, envVar.Value)
				}
			}
		}
		for _, c := range deployment.Spec.Template.Spec.InitContainers {
			for _, envVar := range c.Env {
				if strings.ToLower(envVar.Name) == name {
					t.Errorf("deployment proxy configuration failed for init-container %q, config var %q = %q", c.Name, envVar.Name, envVar.Value)
				}
			}
		}
	}
}

func assertNotFound(t *testing.T, err error) {
	t.Helper()
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected not found got %#v", err)
	}
}

func controllerProcessors(n int32) argoCDOpt {
	return func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.Controller.Processors.Status = n
	}
}
