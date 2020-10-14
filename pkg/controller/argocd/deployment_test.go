package argocd

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/argoproj-labs/argocd-operator/pkg/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/google/go-cmp/cmp"
)

func TestReconcileArgoCD_reconcileApplicationControllerDeployment(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	assertNoError(t, r.reconcileApplicationControllerDeployment(a))

	deployment := &appsv1.Deployment{}
	assertNoError(t, r.client.Get(
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

	assertNoError(t, r.reconcileApplicationControllerDeployment(a))

	a = makeTestArgoCD(controllerProcessors(30))
	assertNoError(t, r.reconcileApplicationControllerDeployment(a))

	deployment := &appsv1.Deployment{}
	assertNoError(t, r.client.Get(
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

func controllerProcessors(n int32) argoCDOpt {
	return func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.Controller.Processors.Status = n
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
	assertNoError(t, err)

	deployment := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-repo-server",
		Namespace: testNamespace,
	}, deployment)
	assertNoError(t, err)

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
	assertNoError(t, err)

	deployment := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-repo-server",
		Namespace: testNamespace,
	}, deployment)
	assertNoError(t, err)

	want := []corev1.VolumeMount{
		{Name: "ssh-known-hosts", MountPath: "/app/config/ssh"},
		{Name: "tls-certs", MountPath: "/app/config/tls"},
		{Name: "gpg-keyring", MountPath: "/app/config/gpg/keys"},
	}

	if diff := cmp.Diff(want, deployment.Spec.Template.Spec.Containers[0].VolumeMounts); diff != "" {
		t.Fatalf("reconcileRepoDeployment failed:\n%s", diff)
	}
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
	assertNoError(t, err)

	deployment := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-repo-server",
		Namespace: testNamespace,
	}, deployment)
	assertNoError(t, err)

	if l := len(deployment.Spec.Template.Spec.Volumes); l != 3 {
		t.Fatalf("reconcileRepoDeployment volumes, got %d, want 3", l)
	}

	if l := len(deployment.Spec.Template.Spec.Containers[0].VolumeMounts); l != 3 {
		t.Fatalf("reconcileRepoDeployment mounts, got %d, want 3", l)
	}
}
