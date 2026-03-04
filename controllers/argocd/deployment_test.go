package argocd

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

const (
	testHTTPProxy  = "example.com:8888"
	testHTTPSProxy = "example.com:8443"
	testNoProxy    = ".example.com"
)

var (
	deploymentNames = []string{
		"argocd-repo-server",
		"argocd-dex-server",
		"argocd-redis",
		"argocd-server"}
)

type MockTrueFipsChecker struct{}

func (a *MockTrueFipsChecker) IsFipsEnabled() (bool, error) {
	return true, nil
}

type MockFalseFipsChecker struct{}

func (a *MockFalseFipsChecker) IsFipsEnabled() (bool, error) {
	return false, nil
}

func TestReconcileArgoCD_reconcileRepoDeployment_replicas(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name          string
		replicas      int32
		expectedNil   bool
		expectedValue int32
	}{
		{
			name:          "replicas field in the spec should reflect the number of replicas on the cluster",
			replicas:      5,
			expectedNil:   false,
			expectedValue: 5,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.Repo.Replicas = &test.replicas
			})

			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			err := r.reconcileRepoDeployment(a, false)
			assert.NoError(t, err)

			deployment := &appsv1.Deployment{}
			err = r.Get(context.TODO(), types.NamespacedName{
				Name:      "argocd-repo-server",
				Namespace: testNamespace,
			}, deployment)
			assert.NoError(t, err)
			assert.Equal(t, test.expectedNil, deployment.Spec.Replicas == nil)
			if deployment.Spec.Replicas != nil {
				assert.Equal(t, test.expectedValue, *deployment.Spec.Replicas)
			}
		})
	}
}

func TestReconcileArgoCD_reconcile_ServerDeployment_replicas(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	var (
		initalReplicas  int32 = 4
		updatedReplicas int32 = 5
	)

	tests := []struct {
		name              string
		initialReplicas   *int32
		updatedReplicas   *int32
		autoscale         bool
		wantFinalReplicas *int32
	}{
		{
			name:              "deployment spec replicas initially nil, updated by operator, no autoscale",
			initialReplicas:   nil,
			updatedReplicas:   &updatedReplicas,
			autoscale:         false,
			wantFinalReplicas: &updatedReplicas,
		},
		{
			name:              "deployment spec replicas initially not nil, updated by operator, no autoscale",
			initialReplicas:   &initalReplicas,
			updatedReplicas:   &updatedReplicas,
			autoscale:         false,
			wantFinalReplicas: &updatedReplicas,
		},
		{
			name:              "deployment spec replicas initially nil, ignored by operator with autoscale",
			initialReplicas:   nil,
			updatedReplicas:   &updatedReplicas,
			autoscale:         true,
			wantFinalReplicas: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.Server.Replicas = test.initialReplicas
				a.Spec.Server.Autoscale.Enabled = test.autoscale
			})

			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			err := r.reconcileServerDeployment(a, false)
			assert.NoError(t, err)

			deployment := &appsv1.Deployment{}
			err = r.Get(context.TODO(), types.NamespacedName{
				Name:      "argocd-server",
				Namespace: testNamespace,
			}, deployment)
			assert.NoError(t, err)
			assert.Equal(t, test.initialReplicas, deployment.Spec.Replicas)

			a.Spec.Server.Replicas = test.updatedReplicas
			err = r.reconcileServerDeployment(a, false)
			assert.NoError(t, err)

			deployment = &appsv1.Deployment{}
			err = r.Get(context.TODO(), types.NamespacedName{
				Name:      "argocd-server",
				Namespace: testNamespace,
			}, deployment)
			assert.NoError(t, err)
			assert.Equal(t, test.wantFinalReplicas, deployment.Spec.Replicas)
		})
	}
}

func TestReconcileArgoCD_reconcileRepoDeployment_loglevel(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	repoDeps := []*argoproj.ArgoCD{
		makeTestArgoCD(func(a *argoproj.ArgoCD) {
			a.Spec.Repo.LogLevel = "warn"
		}),
		makeTestArgoCD(func(a *argoproj.ArgoCD) {
			a.Spec.Repo.LogLevel = "error"
		}),
		makeTestArgoCD(),
	}

	for _, lglv := range repoDeps {

		var ll string
		if lglv.Spec.Repo.LogLevel == "" {
			ll = "info"
		} else {
			ll = lglv.Spec.Repo.LogLevel
		}

		resObjs := []client.Object{lglv}
		subresObjs := []client.Object{lglv}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		err := r.reconcileRepoDeployment(lglv, false)
		assert.NoError(t, err)
		deployment := &appsv1.Deployment{}
		err = r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-repo-server",
			Namespace: testNamespace,
		}, deployment)
		assert.NoError(t, err)

		for _, con := range deployment.Spec.Template.Spec.Containers {
			if con.Name == "argocd-repo-server" {
				for cmdKey, cmd := range con.Command {
					if cmd == "--loglevel" {
						if diff := cmp.Diff(ll, con.Command[cmdKey+1]); diff != "" {
							t.Fatalf("reconcileRepoDeployment failed:\n%s", diff)
						}
					}
				}
			}
		}
	}
}

// TODO: This needs more testing for the rest of the RepoDeployment container
// fields.

// reconcileRepoDeployment creates a Deployment with the correct volumes for the
// repo-server.
func TestReconcileArgoCD_reconcileRepoDeployment_volumes(t *testing.T) {
	t.Run("create default volumes", func(t *testing.T) {
		logf.SetLogger(ZapLogger(true))
		a := makeTestArgoCD()

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		err := r.reconcileRepoDeployment(a, false)
		assert.NoError(t, err)
		deployment := &appsv1.Deployment{}
		err = r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-repo-server",
			Namespace: testNamespace,
		}, deployment)
		assert.NoError(t, err)
		assert.Equal(t, repoServerDefaultVolumes(), deployment.Spec.Template.Spec.Volumes)
	})

	t.Run("create extra volumes", func(t *testing.T) {
		customVolume := corev1.Volume{
			Name: "custom-volume",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}

		logf.SetLogger(ZapLogger(true))
		a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
			a.Spec.Repo.Volumes = []corev1.Volume{customVolume}
		})

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		err := r.reconcileRepoDeployment(a, false)
		assert.NoError(t, err)
		deployment := &appsv1.Deployment{}
		err = r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-repo-server",
			Namespace: testNamespace,
		}, deployment)
		assert.NoError(t, err)
		assert.Contains(t, deployment.Spec.Template.Spec.Volumes, customVolume)
	})
}

func TestReconcileArgoCD_reconcile_ServerDeployment_env(t *testing.T) {
	t.Run("Test some env set in argocd-server", func(t *testing.T) {
		logf.SetLogger(ZapLogger(true))
		a := makeTestArgoCD()
		a.Spec.Server.Env = []corev1.EnvVar{
			{
				Name:  "FOO",
				Value: "BAR",
			},
			{
				Name:  "BAR",
				Value: "FOO",
			},
		}
		timeout := 600
		a.Spec.Repo.ExecTimeout = &timeout

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		err := r.reconcileServerDeployment(a, false)
		assert.NoError(t, err)
		deployment := &appsv1.Deployment{}
		err = r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-server",
			Namespace: testNamespace,
		}, deployment)
		assert.NoError(t, err)

		// Check that the env vars are set, Count is 3 because of the default REDIS_PASSWORD env var
		assert.Len(t, deployment.Spec.Template.Spec.Containers[0].Env, 3)
		assert.Contains(t, deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "FOO", Value: "BAR"})
		assert.Contains(t, deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "BAR", Value: "FOO"})
	})

}

func TestReconcileArgoCD_reconcileRepoDeployment_env(t *testing.T) {
	t.Run("Test some env set in argocd-repo-server", func(t *testing.T) {
		logf.SetLogger(ZapLogger(true))
		a := makeTestArgoCD()
		a.Spec.Repo.Env = []corev1.EnvVar{
			{
				Name:  "FOO",
				Value: "BAR",
			},
			{
				Name:  "BAR",
				Value: "FOO",
			},
		}
		timeout := 600
		a.Spec.Repo.ExecTimeout = &timeout

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		err := r.reconcileRepoDeployment(a, false)
		assert.NoError(t, err)
		deployment := &appsv1.Deployment{}
		err = r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-repo-server",
			Namespace: testNamespace,
		}, deployment)
		assert.NoError(t, err)

		// Check that the env vars are set, Count is 4 because of the default REDIS_PASSWORD env var
		assert.Len(t, deployment.Spec.Template.Spec.Containers[0].Env, 4)
		assert.Contains(t, deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "FOO", Value: "BAR"})
		assert.Contains(t, deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "BAR", Value: "FOO"})
		assert.Contains(t, deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "ARGOCD_EXEC_TIMEOUT", Value: "600s"})
	})

	t.Run("ExecTimeout set", func(t *testing.T) {
		logf.SetLogger(ZapLogger(true))
		a := makeTestArgoCD()
		timeout := 600
		a.Spec.Repo.ExecTimeout = &timeout

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		err := r.reconcileRepoDeployment(a, false)
		assert.NoError(t, err)
		deployment := &appsv1.Deployment{}
		err = r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-repo-server",
			Namespace: testNamespace,
		}, deployment)
		assert.NoError(t, err)

		// Check that the env vars are set, Count is 2 because of the default REDIS_PASSWORD env var
		assert.Len(t, deployment.Spec.Template.Spec.Containers[0].Env, 2)
		assert.Contains(t, deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "ARGOCD_EXEC_TIMEOUT", Value: "600s"})
	})

	t.Run("ExecTimeout set with env set explicitly", func(t *testing.T) {
		logf.SetLogger(ZapLogger(true))
		a := makeTestArgoCD()
		timeout := 600
		a.Spec.Repo.ExecTimeout = &timeout
		a.Spec.Repo.Env = []corev1.EnvVar{
			{
				Name:  "ARGOCD_EXEC_TIMEOUT",
				Value: "20s",
			},
		}

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		err := r.reconcileRepoDeployment(a, false)
		assert.NoError(t, err)
		deployment := &appsv1.Deployment{}
		err = r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-repo-server",
			Namespace: testNamespace,
		}, deployment)
		assert.NoError(t, err)

		// Check that the env vars are set, Count is 2 because of the default REDIS_PASSWORD env var
		assert.Len(t, deployment.Spec.Template.Spec.Containers[0].Env, 2)
		assert.Contains(t, deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "ARGOCD_EXEC_TIMEOUT", Value: "600s"})
	})
	t.Run("ExecTimeout not set", func(t *testing.T) {
		logf.SetLogger(ZapLogger(true))
		a := makeTestArgoCD()

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		err := r.reconcileRepoDeployment(a, false)
		assert.NoError(t, err)
		deployment := &appsv1.Deployment{}
		err = r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-repo-server",
			Namespace: testNamespace,
		}, deployment)
		assert.NoError(t, err)
	})
}

// reconcileRepoDeployment creates a Deployment with the correct mounts for the
// repo-server.
func TestReconcileArgoCD_reconcileRepoDeployment_mounts(t *testing.T) {
	t.Run("Create default mounts", func(t *testing.T) {
		logf.SetLogger(ZapLogger(true))
		a := makeTestArgoCD()

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		err := r.reconcileRepoDeployment(a, false)
		assert.NoError(t, err)

		deployment := &appsv1.Deployment{}
		err = r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-repo-server",
			Namespace: testNamespace,
		}, deployment)
		assert.NoError(t, err)
		assert.Equal(t, repoServerDefaultVolumeMounts(), deployment.Spec.Template.Spec.Containers[0].VolumeMounts)
	})

	t.Run("Add extra mounts", func(t *testing.T) {
		testMount := corev1.VolumeMount{
			Name:      "test-mount",
			MountPath: "/test-mount",
		}

		logf.SetLogger(ZapLogger(true))
		a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
			a.Spec.Repo.VolumeMounts = []corev1.VolumeMount{testMount}
		})

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		err := r.reconcileRepoDeployment(a, false)
		assert.NoError(t, err)

		deployment := &appsv1.Deployment{}
		err = r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-repo-server",
			Namespace: testNamespace,
		}, deployment)
		assert.NoError(t, err)
		assert.Contains(t, deployment.Spec.Template.Spec.Containers[0].VolumeMounts, testMount)
	})

	t.Run("Add extra volume mount and volume that override default /tmp volume mount and volume", func(t *testing.T) {
		testMount := corev1.VolumeMount{
			Name:      "test-mount",
			MountPath: "/tmp",
		}

		logf.SetLogger(ZapLogger(true))
		a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
			a.Spec.Repo.VolumeMounts = []corev1.VolumeMount{testMount}
			a.Spec.Repo.Volumes = []corev1.Volume{{Name: "test-mount"}}
		})

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		err := r.reconcileRepoDeployment(a, false)
		assert.NoError(t, err)

		deployment := &appsv1.Deployment{}
		err = r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-repo-server",
			Namespace: testNamespace,
		}, deployment)
		assert.NoError(t, err)

		assert.Len(t, deployment.Spec.Template.Spec.Containers, 1)

		container := deployment.Spec.Template.Spec.Containers[0]

		containsTestMount := false
		containsDefaultMount := false

		for _, volumeMount := range container.VolumeMounts {

			if volumeMount.Name == testMount.Name {
				containsTestMount = true
			} else if volumeMount.MountPath == "/tmp" {
				containsDefaultMount = true
			}
		}

		assert.True(t, containsTestMount, "should contain test-mount volume mount")
		assert.False(t, containsDefaultMount, "should not contain the default mount, since this is being overriden by the test-mount")

		containsTestMountVolume := false
		containsDefaultVolume := false
		for _, volume := range deployment.Spec.Template.Spec.Volumes {
			if volume.Name == "test-mount" {
				containsTestMountVolume = true
			}
			if volume.Name == "tmp" {
				containsDefaultVolume = true
			}
		}

		assert.True(t, containsTestMountVolume, "should contain test-mount molume")
		assert.False(t, containsDefaultVolume, "should not contain default tmp volume")

	})
}

func TestReconcileArgoCD_reconcileRepoDeployment_initContainers(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		ic := corev1.Container{
			Name:  "test-init-container",
			Image: "test-image",
		}
		a.Spec.Repo.InitContainers = []corev1.Container{ic}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := r.reconcileRepoDeployment(a, false)
	assert.NoError(t, err)

	deployment := &appsv1.Deployment{}
	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-repo-server",
		Namespace: testNamespace,
	}, deployment)
	assert.NoError(t, err)
	assert.Equal(t, deployment.Spec.Template.Spec.InitContainers[1].Name, "test-init-container")
}

func TestReconcileArgoCD_reconcileRepoDeployment_missingInitContainers(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
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
					InitContainers: []corev1.Container{},
				},
			},
		},
	}

	resObjs := []client.Object{a, d}
	subresObjs := []client.Object{a, d}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := r.reconcileRepoDeployment(a, false)
	assert.NoError(t, err)
	deployment := &appsv1.Deployment{}
	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-repo-server",
		Namespace: testNamespace,
	}, deployment)
	assert.NoError(t, err)
	assert.Len(t, deployment.Spec.Template.Spec.InitContainers, 1)
	assert.Equal(t, deployment.Spec.Template.Spec.InitContainers[0].Name, "copyutil")
}
func TestReconcileArgoCD_reconcileRepoDeployment_unexpectedInitContainer(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
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
					InitContainers: []corev1.Container{
						{
							Name:    "unknown",
							Command: []string{"testing-ic"},
							Image:   "test-image-ic",
						},
					},
				},
			},
		},
	}

	resObjs := []client.Object{a, d}
	subresObjs := []client.Object{a, d}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := r.reconcileRepoDeployment(a, false)
	assert.NoError(t, err)
	deployment := &appsv1.Deployment{}
	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-repo-server",
		Namespace: testNamespace,
	}, deployment)
	assert.NoError(t, err)
	assert.Len(t, deployment.Spec.Template.Spec.InitContainers, 1)
	assert.Equal(t, deployment.Spec.Template.Spec.InitContainers[0].Name, "copyutil")
}

func TestReconcileArgoCD_reconcileRepoDeployment_command(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := r.reconcileRepoDeployment(a, false)
	assert.NoError(t, err)

	deployment := &appsv1.Deployment{}
	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-repo-server",
		Namespace: testNamespace,
	}, deployment)
	assert.NoError(t, err)

	deployment.Spec.Template.Spec.Containers[0].Command[6] = "debug"
	err = r.reconcileRepoDeployment(a, false)
	assert.NoError(t, err)

	assert.Equal(t, "debug", deployment.Spec.Template.Spec.Containers[0].Command[6])
}

// reconcileRepoDeployments creates a Deployment with the proxy settings from the
// environment propagated.
func TestReconcileArgoCD_reconcileDeployments_proxy(t *testing.T) {

	t.Setenv("HTTP_PROXY", testHTTPProxy)
	t.Setenv("HTTPS_PROXY", testHTTPSProxy)
	t.Setenv("no_proxy", testNoProxy)

	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		//lint:ignore SA1019 known to be deprecated
		a.Spec.Grafana.Enabled = true //nolint:staticcheck // SA1019: We must test deprecated fields.
		a.Spec.SSO = &argoproj.ArgoCDSSOSpec{
			Provider: argoproj.SSOProviderTypeDex,
			Dex: &argoproj.ArgoCDDexSpec{
				Config: "test",
			},
		}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := r.reconcileDeployments(a, false)
	assert.NoError(t, err)
	err = r.reconcileDexDeployment(a)
	assert.NoError(t, err)

	for _, v := range deploymentNames {
		assertDeploymentHasProxyVars(t, r.Client, v)
	}
}

// reconcileRepoDeployments creates a Deployment with the proxy settings from the
// environment propagated.
//
// If the deployments already exist, they should be updated to reflect the new
// environment variables.
func TestReconcileArgoCD_reconcileDeployments_proxy_update_existing(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.SSO = &argoproj.ArgoCDSSOSpec{
			Provider: argoproj.SSOProviderTypeDex,
			Dex: &argoproj.ArgoCDDexSpec{
				Config: "test",
			},
		}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := r.reconcileDeployments(a, false)
	assert.NoError(t, err)

	err = r.reconcileDexDeployment(a)
	assert.NoError(t, err)

	for _, v := range deploymentNames {
		refuteDeploymentHasProxyVars(t, r.Client, v)
	}

	t.Setenv("HTTP_PROXY", testHTTPProxy)
	t.Setenv("HTTPS_PROXY", testHTTPSProxy)
	t.Setenv("no_proxy", testNoProxy)

	logf.SetLogger(ZapLogger(true))

	err = r.reconcileDeployments(a, false)
	assert.NoError(t, err)
	err = r.reconcileDexDeployment(a)
	assert.NoError(t, err)

	for _, v := range deploymentNames {
		assertDeploymentHasProxyVars(t, r.Client, v)
	}
}

// TODO: This should be subsumed into testing of the HA setup.
func TestReconcileArgoCD_reconcileDeployments_HA_proxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", testHTTPProxy)
	t.Setenv("HTTPS_PROXY", testHTTPSProxy)
	t.Setenv("no_proxy", testNoProxy)

	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.HA.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := r.reconcileDeployments(a, false)
	assert.NoError(t, err)

	assertDeploymentHasProxyVars(t, r.Client, "argocd-redis-ha-haproxy")
}

func TestReconcileArgoCD_reconcileDeployments_HA_proxy_with_resources(t *testing.T) {
	t.Setenv("HTTP_PROXY", testHTTPProxy)
	t.Setenv("HTTPS_PROXY", testHTTPSProxy)
	t.Setenv("no_proxy", testNoProxy)

	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCDWithResources(func(a *argoproj.ArgoCD) {
		a.Spec.HA.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	// test resource is Created on reconciliation
	assert.NoError(t, r.reconcileRedisHAProxyDeployment(a))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      a.Name + "-redis-ha-haproxy",
			Namespace: a.Namespace,
		},
		deployment))

	testResources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("128Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("250m"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("256Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("500m"),
		},
	}
	assert.Equal(t, deployment.Spec.Template.Spec.Containers[0].Resources, testResources)
	assert.Equal(t, deployment.Spec.Template.Spec.InitContainers[0].Resources, testResources)
	assert.Equal(t, deployment.Spec.Strategy.RollingUpdate.MaxSurge, &intstr.IntOrString{IntVal: 0})

	// test resource is Updated on reconciliation
	newResources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("256Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("500m"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("512Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("1"),
		},
	}
	a.Spec.HA.Resources = &newResources
	assert.NoError(t, r.reconcileRedisHAProxyDeployment(a))

	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      a.Name + "-redis-ha-haproxy",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Equal(t, deployment.Spec.Template.Spec.Containers[0].Resources, newResources)
	assert.Equal(t, deployment.Spec.Template.Spec.InitContainers[0].Resources, newResources)
	assert.Equal(t, deployment.Spec.Strategy.RollingUpdate.MaxSurge, &intstr.IntOrString{IntVal: 0})

}
func TestReconcileArgoCD_reconcileRedisHAProxyDeployment_ModifyContainerSpec(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.HA.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, r.reconcileRedisHAProxyDeployment(a))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-redis-ha-haproxy",
			Namespace: a.Namespace,
		},
		deployment))

	// Modify the deployment container environment variables
	deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
		Name:  "TEST_ENV",
		Value: "test",
	})

	assert.NoError(t, r.Update(context.TODO(), deployment))

	// Reconcile again
	assert.NoError(t, r.reconcileRedisHAProxyDeployment(a))

	// Check if the environment variable changes were reverted
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-redis-ha-haproxy",
			Namespace: a.Namespace,
		},
		deployment))

	assert.NotContains(t, deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
		Name:  "TEST_ENV",
		Value: "test",
	})

	// Modify the deployment initcontainer environment variables
	deployment.Spec.Template.Spec.InitContainers[0].Env = append(deployment.Spec.Template.Spec.InitContainers[0].Env, corev1.EnvVar{
		Name:  "TEST_ENV",
		Value: "test",
	})

	assert.NoError(t, r.Update(context.TODO(), deployment))

	// Reconcile again
	assert.NoError(t, r.reconcileRedisHAProxyDeployment(a))

	// Check if the environment variable changes were reverted
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-redis-ha-haproxy",
			Namespace: a.Namespace,
		},
		deployment))

	assert.NotContains(t, deployment.Spec.Template.Spec.InitContainers[0].Env, corev1.EnvVar{
		Name:  "TEST_ENV",
		Value: "test",
	})

	// Modify the deployment volumes
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "test-volume",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      "test-volume",
		MountPath: "/test",
	})

	assert.NoError(t, r.Update(context.TODO(), deployment))

	// Reconcile again
	assert.NoError(t, r.reconcileRedisHAProxyDeployment(a))

	// Check if the volume changes were reverted
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-redis-ha-haproxy",
			Namespace: a.Namespace,
		},
		deployment))

	assert.NotContains(t, deployment.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "test-volume",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	assert.NotContains(t, deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      "test-volume",
		MountPath: "/test",
	})
}

func TestReconcileArgoCD_reconcileRedisHAProxyDeployment_replicas(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	var (
		oneReplica                 int32 = 1
		twoReplicas                int32 = 2
		argocdDefaultRedisReplicas       = common.ArgoCDDefaultRedisHAReplicas
	)
	tests := []struct {
		name             string
		initialReplicas  *int32
		expectedReplicas *int32
		description      string
	}{
		{
			name:             "deployment with 1 replica should be updated to 3 replicas",
			initialReplicas:  &oneReplica,
			expectedReplicas: &argocdDefaultRedisReplicas,
			description:      "simulates upgrade where deployment had 1 replica",
		},
		{
			name:             "deployment with 2 replicas should be updated to 3 replicas",
			initialReplicas:  &twoReplicas,
			expectedReplicas: &argocdDefaultRedisReplicas,
			description:      "simulates deployment with incorrect replica count",
		},
		{
			name:             "deployment with nil replicas should be set to 3 replicas",
			initialReplicas:  nil,
			expectedReplicas: &argocdDefaultRedisReplicas,
			description:      "simulates deployment with unset replica count",
		},
		{
			name:             "deployment with 3 replicas should remain 3 replicas",
			initialReplicas:  &argocdDefaultRedisReplicas,
			expectedReplicas: &argocdDefaultRedisReplicas,
			description:      "simulates deployment already at correct replica count",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.HA.Enabled = true
			})

			// Create an existing deployment with the initial replica count
			existingDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      a.Name + "-redis-ha-haproxy",
					Namespace: a.Namespace,
					Labels: map[string]string{
						common.ArgoCDKeyName:      a.Name + "-redis-ha-haproxy",
						common.ArgoCDKeyComponent: "redis",
						common.ArgoCDKeyPartOf:    common.ArgoCDAppName,
						common.ArgoCDKeyManagedBy: a.Namespace,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: test.initialReplicas,
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "haproxy",
									Image: getRedisHAProxyContainerImage(a),
								},
							},
						},
					},
				},
			}

			resObjs := []client.Object{a, existingDeployment}
			subresObjs := []client.Object{a, existingDeployment}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			// Reconcile the deployment
			assert.NoError(t, r.reconcileRedisHAProxyDeployment(a))

			// Verify the replica count was updated correctly
			deployment := &appsv1.Deployment{}
			assert.NoError(t, r.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      a.Name + "-redis-ha-haproxy",
					Namespace: a.Namespace,
				},
				deployment))

			if test.expectedReplicas == nil {
				assert.Nil(t, deployment.Spec.Replicas, test.description)
			} else {
				assert.NotNil(t, deployment.Spec.Replicas, "replicas should not be nil")
				assert.Equal(t, *test.expectedReplicas, *deployment.Spec.Replicas, test.description)
			}
		})
	}
}

func TestReconcileArgoCD_reconcileRepoDeployment_updatesVolumeMounts(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
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
					InitContainers: []corev1.Container{
						{
							Command: []string{"testing"},
							Image:   "test-image",
						},
					},
				},
			},
		},
	}

	resObjs := []client.Object{a, d}
	subresObjs := []client.Object{a, d}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := r.reconcileRepoDeployment(a, false)
	assert.NoError(t, err)

	deployment := &appsv1.Deployment{}
	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-repo-server",
		Namespace: testNamespace,
	}, deployment)
	assert.NoError(t, err)

	assert.Len(t, deployment.Spec.Template.Spec.Volumes, 9)
	assert.Len(t, deployment.Spec.Template.Spec.Containers[0].VolumeMounts, 8)
}

func Test_proxyEnvVars(t *testing.T) {
	t.Setenv("HTTP_PROXY", testHTTPProxy)
	t.Setenv("HTTPS_PROXY", testHTTPSProxy)
	t.Setenv("no_proxy", testNoProxy)
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
		assert.Equal(t, tt.want, e)
	}
}

func TestReconcileArgoCD_reconcileDeployment_nodePlacement(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD((func(a *argoproj.ArgoCD) {
		a.Spec.NodePlacement = &argoproj.ArgoCDNodePlacementSpec{
			NodeSelector: deploymentDefaultNodeSelector(),
			Tolerations:  deploymentDefaultTolerations(),
		}
	}))

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := r.reconcileRepoDeployment(a, false) //can use other deployments as well
	assert.NoError(t, err)
	deployment := &appsv1.Deployment{}
	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-repo-server",
		Namespace: testNamespace,
	}, deployment)
	assert.NoError(t, err)

	nSelectors := deploymentDefaultNodeSelector()
	nSelectors = argoutil.AppendStringMap(nSelectors, common.DefaultNodeSelector())

	if diff := cmp.Diff(nSelectors, deployment.Spec.Template.Spec.NodeSelector); diff != "" {
		t.Fatalf("reconcileDeployment failed:\n%s", diff)
	}
	if diff := cmp.Diff(deploymentDefaultTolerations(), deployment.Spec.Template.Spec.Tolerations); diff != "" {
		t.Fatalf("reconcileDeployment failed:\n%s", diff)
	}
}

func deploymentDefaultNodeSelector() map[string]string {
	nodeSelector := map[string]string{
		"test_key1": "test_value1",
		"test_key2": "test_value2",
	}
	return nodeSelector
}
func deploymentDefaultTolerations() []corev1.Toleration {
	toleration := []corev1.Toleration{
		{
			Key:    "test_key1",
			Value:  "test_value1",
			Effect: corev1.TaintEffectNoSchedule,
		},
		{
			Key:      "test_key2",
			Value:    "test_value2",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}
	return toleration
}

func TestReconcileArgocd_reconcileRepoServerRedisTLS(t *testing.T) {
	t.Run("with DisableTLSVerification = false (the default)", func(t *testing.T) {
		logf.SetLogger(ZapLogger(true))
		a := makeTestArgoCD()

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		assert.NoError(t, r.reconcileRepoDeployment(a, true))

		deployment := &appsv1.Deployment{}
		assert.NoError(t, r.Get(
			context.TODO(),
			types.NamespacedName{
				Name:      "argocd-repo-server",
				Namespace: a.Namespace,
			},
			deployment))

		wantCmd := []string{
			"uid_entrypoint.sh",
			"argocd-repo-server",
			"--redis", "argocd-redis.argocd.svc.cluster.local:6379",
			"--redis-use-tls",
			"--redis-ca-certificate", "/app/config/reposerver/tls/redis/tls.crt",
			"--loglevel", "info",
			"--logformat", "text",
		}
		assert.Equal(t, wantCmd, deployment.Spec.Template.Spec.Containers[0].Command)
	})

	t.Run("with DisableTLSVerification = true", func(t *testing.T) {
		logf.SetLogger(ZapLogger(true))
		a := makeTestArgoCD(func(cd *argoproj.ArgoCD) {
			cd.Spec.Redis.DisableTLSVerification = true
		})

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		assert.NoError(t, r.reconcileRepoDeployment(a, true))

		deployment := &appsv1.Deployment{}
		assert.NoError(t, r.Get(
			context.TODO(),
			types.NamespacedName{
				Name:      "argocd-repo-server",
				Namespace: a.Namespace,
			},
			deployment))

		wantCmd := []string{
			"uid_entrypoint.sh",
			"argocd-repo-server",
			"--redis", "argocd-redis.argocd.svc.cluster.local:6379",
			"--redis-use-tls",
			"--redis-insecure-skip-tls-verify",
			"--loglevel", "info",
			"--logformat", "text",
		}
		assert.Equal(t, wantCmd, deployment.Spec.Template.Spec.Containers[0].Command)
	})
}

func TestReconcileArgoCD_reconcileServerDeployment(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, r.reconcileServerDeployment(a, false))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Get(
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
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command: []string{
					"argocd-server",
					"--staticassets",
					"/shared/app",
					"--dex-server",
					"https://argocd-dex-server.argocd.svc.cluster.local:5556",
					"--repo-server",
					"argocd-repo-server.argocd.svc.cluster.local:8081",
					"--redis",
					"argocd-redis.argocd.svc.cluster.local:6379",
					"--loglevel",
					"info",
					"--logformat",
					"text",
				},
				Env: []corev1.EnvVar{
					{Name: "REDIS_PASSWORD", Value: "",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "argocd-redis-initial-password",
								},
								Key: "admin.password",
							},
						},
					},
				},
				Ports: []corev1.ContainerPort{
					{ContainerPort: 8080},
					{ContainerPort: 8083},
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 3,
					PeriodSeconds:       30,
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 3,
					PeriodSeconds:       30,
				},
				SecurityContext: argoutil.DefaultSecurityContext(),
				VolumeMounts:    serverDefaultVolumeMounts(),
			},
		},
		Volumes:            serverDefaultVolumes(),
		ServiceAccountName: "argocd-argocd-server",
		NodeSelector:       common.DefaultNodeSelector(),
	}

	assert.Equal(t, want, deployment.Spec.Template.Spec)

	assert.NoError(t, r.reconcileServerDeployment(a, true))
	deployment = &appsv1.Deployment{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-server",
			Namespace: a.Namespace,
		},
		deployment))
	wantCmd := []string{
		"argocd-server",
		"--staticassets",
		"/shared/app",
		"--dex-server",
		"https://argocd-dex-server.argocd.svc.cluster.local:5556",
		"--repo-server",
		"argocd-repo-server.argocd.svc.cluster.local:8081",
		"--redis",
		"argocd-redis.argocd.svc.cluster.local:6379",
		"--redis-use-tls",
		"--redis-ca-certificate",
		"/app/config/server/tls/redis/tls.crt",
		"--loglevel",
		"info",
		"--logformat",
		"text",
	}
	assert.Equal(t, wantCmd, deployment.Spec.Template.Spec.Containers[0].Command)
}

func TestArgoCDServerDeploymentCommand(t *testing.T) {
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	baseCommand := []string{
		"argocd-server",
		"--staticassets",
		"/shared/app",
		"--dex-server",
		"https://argocd-dex-server.argocd.svc.cluster.local:5556",
		"--repo-server",
		"argocd-repo-server.argocd.svc.cluster.local:8081",
		"--redis",
		"argocd-redis.argocd.svc.cluster.local:6379",
		"--loglevel",
		"info",
		"--logformat",
		"text",
	}

	// When a single command argument is passed
	a.Spec.Server.ExtraCommandArgs = []string{
		"--rootpath",
		"/argocd",
	}

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.reconcileServerDeployment(a, false))

	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-server",
			Namespace: a.Namespace,
		},
		deployment))

	cmd := append(baseCommand, "--rootpath", "/argocd")
	assert.Equal(t, cmd, deployment.Spec.Template.Spec.Containers[0].Command)

	// When multiple command arguments are passed
	a.Spec.Server.ExtraCommandArgs = []string{
		"--rootpath",
		"/argocd",
		"--foo",
		"bar",
		"test",
	}

	assert.NoError(t, r.reconcileServerDeployment(a, false))
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-server",
			Namespace: a.Namespace,
		},
		deployment))

	cmd = append(cmd, "--foo", "bar", "test")
	assert.Equal(t, cmd, deployment.Spec.Template.Spec.Containers[0].Command)

	// When one of the ExtraCommandArgs already exists in cmd with same or different value
	a.Spec.Server.ExtraCommandArgs = []string{
		"--redis",
		"foo.scv.cluster.local:6379",
	}

	wantCmd := []string{
		"argocd-server",
		"--staticassets",
		"/shared/app",
		"--dex-server",
		"https://argocd-dex-server.argocd.svc.cluster.local:5556",
		"--repo-server",
		"argocd-repo-server.argocd.svc.cluster.local:8081",
		"--loglevel",
		"info",
		"--logformat",
		"text",
		"--redis",
		"foo.scv.cluster.local:6379",
	}

	assert.NoError(t, r.reconcileServerDeployment(a, false))
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-server",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Equal(t, wantCmd, deployment.Spec.Template.Spec.Containers[0].Command)

	// When ExtraCommandArgs contains a non-duplicate argument along with a duplicate
	a.Spec.Server.ExtraCommandArgs = []string{
		"--rootpath",
		"/argocd",
		"--foo",
		"bar",
		"test",
		"--logformat", // Duplicate flag and value
		"text",
		"--newarg", // Non-duplicate argument
		"newvalue",
		"--newarg", // Duplicate argument passing at once
		"newvalue",
	}

	assert.NoError(t, r.reconcileServerDeployment(a, false))
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-server",
			Namespace: a.Namespace,
		},
		deployment))

	// Non-duplicate argument "--newarg" should be added, duplicate "--newarg" which is added twice is ignored
	cmd = append(cmd, "--newarg", "newvalue")
	assert.Equal(t, cmd, deployment.Spec.Template.Spec.Containers[0].Command)

	// Remove all the command arguments that were added.
	a.Spec.Server.ExtraCommandArgs = []string{}

	assert.NoError(t, r.reconcileServerDeployment(a, false))
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-server",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Equal(t, baseCommand, deployment.Spec.Template.Spec.Containers[0].Command)
}

func TestReconcileServer_InitContainers(t *testing.T) {
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Server.InitContainers = []corev1.Container{
			{
				Name:  "test-init-container",
				Image: "test-image",
			},
		}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.reconcileServerDeployment(a, false))

	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-server",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Len(t, deployment.Spec.Template.Spec.InitContainers, 1)
	assert.Equal(t, "test-init-container", deployment.Spec.Template.Spec.InitContainers[0].Name)
	assert.Equal(t, "test-image", deployment.Spec.Template.Spec.InitContainers[0].Image)

	// Remove the init container
	a.Spec.Server.InitContainers = []corev1.Container{}

	assert.NoError(t, r.reconcileServerDeployment(a, false))
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-server",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Len(t, deployment.Spec.Template.Spec.InitContainers, 0)
	assert.Len(t, deployment.Spec.Template.Spec.Containers, 1)
}

func TestReconcile_SidecarContainers(t *testing.T) {
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Server.SidecarContainers = []corev1.Container{
			{
				Name:  "test-sidecar",
				Image: "test-image",
			},
		}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.reconcileServerDeployment(a, false))

	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-server",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Len(t, deployment.Spec.Template.Spec.Containers, 2)
	assert.Equal(t, "test-sidecar", deployment.Spec.Template.Spec.Containers[1].Name)
	assert.Equal(t, "test-image", deployment.Spec.Template.Spec.Containers[1].Image)

	// Remove the sidecar container
	a.Spec.Server.SidecarContainers = []corev1.Container{}

	assert.NoError(t, r.reconcileServerDeployment(a, false))
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-server",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Len(t, deployment.Spec.Template.Spec.Containers, 1)
}

func TestReconcileServer_RolloutUI(t *testing.T) {
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Server.EnableRolloutsUI = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.reconcileServerDeployment(a, false))

	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-server",
			Namespace: a.Namespace,
		},
		deployment))

	// Check for the init container
	assert.Len(t, deployment.Spec.Template.Spec.InitContainers, 1)
	assert.Equal(t, "rollout-extension", deployment.Spec.Template.Spec.InitContainers[0].Name)
	assert.Equal(t, common.ArgoCDExtensionInstallerImage, deployment.Spec.Template.Spec.InitContainers[0].Image)

	// assert that rollout-extensions volume is mounted at /tmp/extensions for both the initContainer and container
	foundExtensionsVolumeMount := false
	for _, volMnt := range deployment.Spec.Template.Spec.InitContainers[0].VolumeMounts {
		if volMnt.Name == "rollout-extensions" {
			foundExtensionsVolumeMount = true
			assert.NotNil(t, volMnt.MountPath)
			assert.Equal(t, "/tmp/extensions/", volMnt.MountPath)
		}
	}
	assert.True(t, foundExtensionsVolumeMount, "expected volume mount 'rollout-extensions' to be present in init container")
	foundExtensionsVolumeMount = false
	for _, vol := range deployment.Spec.Template.Spec.Containers[0].VolumeMounts {
		if vol.Name == "rollout-extensions" {
			foundExtensionsVolumeMount = true
			assert.NotNil(t, vol.MountPath)
			assert.Equal(t, "/tmp/extensions/", vol.MountPath)
		}
	}
	assert.True(t, foundExtensionsVolumeMount, "expected volume mount 'rollout-extensions' to be present in container")

	// assert that tmp volume is mounted at /tmp for both the initContainer and container
	foundTmpVolumeMount := false
	for _, volMnt := range deployment.Spec.Template.Spec.InitContainers[0].VolumeMounts {
		if volMnt.Name == "tmp" {
			foundTmpVolumeMount = true
			assert.NotNil(t, volMnt.MountPath)
			assert.Equal(t, volMnt.MountPath, "/tmp")
		}
	}
	assert.True(t, foundTmpVolumeMount, "expected volume mount 'tmp' to be present in init container")
	foundTmpVolumeMount = false
	for _, volMnt := range deployment.Spec.Template.Spec.Containers[0].VolumeMounts {
		if volMnt.Name == "tmp" {
			foundTmpVolumeMount = true
			assert.NotNil(t, volMnt.MountPath)
			assert.Equal(t, volMnt.MountPath, "/tmp")
		}
	}
	assert.True(t, foundTmpVolumeMount, "expected volume mount 'tmp' to be present in container")

	// Check for the volumes
	foundVolume := false
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.Name == "rollout-extensions" {
			foundVolume = true
			assert.NotNil(t, vol.EmptyDir)
		}
	}
	assert.True(t, foundVolume, "expected volume 'rollout-extensions' to be present")
	foundTmpVolume := false
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.Name == "tmp" {
			foundTmpVolume = true
			assert.NotNil(t, vol.EmptyDir)
		}
	}
	assert.True(t, foundTmpVolume, "expected volume 'tmp' to be present")

	// Disable rollouts UI
	a.Spec.Server.EnableRolloutsUI = false

	assert.NoError(t, r.reconcileServerDeployment(a, false))
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-server",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Len(t, deployment.Spec.Template.Spec.InitContainers, 0)
	assert.Len(t, deployment.Spec.Template.Spec.Containers, 1)

	// Check that volume is removed
	foundVolume = false
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.Name == "rollout-extensions" {
			foundVolume = true
		}
	}
	assert.False(t, foundVolume, "expected volume 'rollout-extension' to be removed")

	// assert that the tmp volume is present even if rollouts UI extension is disabled.
	foundTmpVolume = false
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.Name == "tmp" {
			foundTmpVolume = true
			assert.NotNil(t, vol.EmptyDir)
		}
	}
	assert.True(t, foundTmpVolume, "expected volume 'tmp' to be present even if rollouts is disabled")

	// assert that tmp volume is mounted at /tmp for the container, when rollouts UI extension is disabled.
	foundTmpVolumeMount = false
	for _, vol := range deployment.Spec.Template.Spec.Containers[0].VolumeMounts {
		if vol.Name == "tmp" {
			foundTmpVolumeMount = true
			assert.NotNil(t, vol.MountPath)
			assert.Equal(t, vol.MountPath, "/tmp")
		}
	}
	assert.True(t, foundTmpVolumeMount, "expected volume mount 'tmp' to be present in container")
}

func TestArgoCDServerCommand_isMergable(t *testing.T) {
	cmd := []string{"--server", "foo.svc.cluster.local", "--path", "/bar"}
	extraCMDArgs := []string{"--extra-path", "/"}
	assert.NoError(t, isMergable(extraCMDArgs, cmd))

	cmd = []string{"--server", "foo.svc.cluster.local", "--path", "/bar"}
	extraCMDArgs = []string{"--server", "bar.com"}
	assert.Error(t, isMergable(extraCMDArgs, cmd))
}

func TestReconcileArgoCD_reconcileServerDeploymentWithInsecure(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Server.Insecure = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, r.reconcileServerDeployment(a, false))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Get(
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
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command: []string{
					"argocd-server",
					"--insecure",
					"--staticassets",
					"/shared/app",
					"--dex-server",
					"https://argocd-dex-server.argocd.svc.cluster.local:5556",
					"--repo-server",
					"argocd-repo-server.argocd.svc.cluster.local:8081",
					"--redis",
					"argocd-redis.argocd.svc.cluster.local:6379",
					"--loglevel",
					"info",
					"--logformat",
					"text",
				},
				Env: []corev1.EnvVar{
					{Name: "REDIS_PASSWORD", Value: "",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "argocd-redis-initial-password",
								},
								Key: "admin.password",
							},
						},
					},
				},
				Ports: []corev1.ContainerPort{
					{ContainerPort: 8080},
					{ContainerPort: 8083},
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 3,
					PeriodSeconds:       30,
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 3,
					PeriodSeconds:       30,
				},
				SecurityContext: argoutil.DefaultSecurityContext(),
				VolumeMounts:    serverDefaultVolumeMounts(),
			},
		},
		Volumes:            serverDefaultVolumes(),
		ServiceAccountName: "argocd-argocd-server",
		NodeSelector:       common.DefaultNodeSelector(),
	}

	assert.Equal(t, want, deployment.Spec.Template.Spec)
}

func TestReconcileArgoCD_reconcileServerDeploymentChangedToInsecure(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, r.reconcileServerDeployment(a, false))

	a = makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Server.Insecure = true
	})
	assert.NoError(t, r.reconcileServerDeployment(a, false))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Get(
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
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command: []string{
					"argocd-server",
					"--insecure",
					"--staticassets",
					"/shared/app",
					"--dex-server",
					"https://argocd-dex-server.argocd.svc.cluster.local:5556",
					"--repo-server",
					"argocd-repo-server.argocd.svc.cluster.local:8081",
					"--redis",
					"argocd-redis.argocd.svc.cluster.local:6379",
					"--loglevel",
					"info",
					"--logformat",
					"text",
				},
				Env: []corev1.EnvVar{
					{Name: "REDIS_PASSWORD", Value: "",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "argocd-redis-initial-password",
								},
								Key: "admin.password",
							},
						},
					},
				},
				Ports: []corev1.ContainerPort{
					{ContainerPort: 8080},
					{ContainerPort: 8083},
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 3,
					PeriodSeconds:       30,
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 3,
					PeriodSeconds:       30,
				},
				SecurityContext: argoutil.DefaultSecurityContext(),
				VolumeMounts:    serverDefaultVolumeMounts(),
			},
		},
		Volumes:            serverDefaultVolumes(),
		ServiceAccountName: "argocd-argocd-server",
		NodeSelector:       common.DefaultNodeSelector(),
	}

	assert.Equal(t, want, deployment.Spec.Template.Spec)
}

func TestReconcileArgoCD_reconcileRedisDeploymentWithoutTLS(t *testing.T) {
	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{cr}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	want := []string{
		"--save",
		"",
		"--appendonly", "no",
		"--requirepass $(REDIS_PASSWORD)",
	}

	assert.NoError(t, r.reconcileRedisDeployment(cr, false))
	d := &appsv1.Deployment{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-redis", Namespace: cr.Namespace}, d))
	got := d.Spec.Template.Spec.Containers[0].Args
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Reconciliation unsucessful: got: %v, want: %v", got, want)
	}
}

func TestReconcileArgoCD_reconcileRedisDeploymentWithTLS(t *testing.T) {
	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{cr}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	want := []string{
		"--save", "",
		"--appendonly", "no",
		"--requirepass $(REDIS_PASSWORD)",
		"--tls-port", "6379",
		"--port", "0",
		"--tls-cert-file", "/app/config/redis/tls/tls.crt",
		"--tls-key-file", "/app/config/redis/tls/tls.key",
		"--tls-auth-clients", "no",
	}

	assert.NoError(t, r.reconcileRedisDeployment(cr, true))
	d := &appsv1.Deployment{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-redis", Namespace: cr.Namespace}, d))
	got := d.Spec.Template.Spec.Containers[0].Args
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Reconciliation unsucessful: got: %v, want: %v", got, want)
	}
}

func TestReconcileArgoCD_reconcileRedisDeployment(t *testing.T) {
	// tests reconciler hook for redis deployment
	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{cr}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	defer resetHooks()()
	Register(testDeploymentHook)

	assert.NoError(t, r.reconcileRedisDeployment(cr, false))
	d := &appsv1.Deployment{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-redis", Namespace: cr.Namespace}, d))
	assert.Equal(t, int32(3), *d.Spec.Replicas)
}

func TestReconcileArgoCD_reconcileRedisDeployment_testImageUpgrade(t *testing.T) {
	// tests reconciler hook for redis deployment
	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{cr}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	defer resetHooks()()
	Register(testDeploymentHook)

	// Verify redis deployment
	assert.NoError(t, r.reconcileRedisDeployment(cr, false))
	existing := &appsv1.Deployment{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-redis", Namespace: cr.Namespace}, existing))

	// Verify Image upgrade
	t.Setenv("ARGOCD_REDIS_IMAGE", "docker.io/redis/redis:latest")
	assert.NoError(t, r.reconcileRedisDeployment(cr, false))

	newRedis := &appsv1.Deployment{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-redis", Namespace: cr.Namespace}, newRedis))
	assert.Equal(t, newRedis.Spec.Template.Spec.Containers[0].Image, "docker.io/redis/redis:latest")
}

func TestReconcileArgoCD_reconcileRedisDeployment_with_error(t *testing.T) {
	// tests reconciler hook for redis deployment
	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{cr}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	defer resetHooks()()
	Register(testErrorHook)

	assert.Error(t, r.reconcileRedisDeployment(cr, false), "this is a test error")
}

func TestReconcileRedisDeployment_serviceAccountNameUpdate(t *testing.T) {
	// tests SA update for redis deployment

	tests := []struct {
		name       string
		SA         string
		expectedSA string
	}{
		{
			name:       "serviceAccountName field should reflect the original value",
			SA:         "argocd-argocd-redis",
			expectedSA: "argocd-argocd-redis",
		},
		{
			name:       "serviceAccountName field should be reset to the original value with an existing SA modification",
			SA:         "builder",
			expectedSA: "argocd-argocd-redis",
		},
		{
			name:       "serviceAccountName field should be reset to the original value with a non-existing SA modification",
			SA:         "argocd-argocd-redis-new",
			expectedSA: "argocd-argocd-redis",
		},
		{
			name:       "serviceAccountName field should be reset to the original value and not left empty",
			SA:         "",
			expectedSA: "argocd-argocd-redis",
		},
	}

	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{cr}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	// Verify redis deployment
	assert.NoError(t, r.reconcileRedisDeployment(cr, false))

	// Verify SA update
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			existing := &appsv1.Deployment{}
			assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-redis", Namespace: cr.Namespace}, existing))

			existing.Spec.Template.Spec.ServiceAccountName = test.SA
			assert.NoError(t, cl.Update(context.TODO(), existing))
			assert.NoError(t, r.reconcileRedisDeployment(cr, false))

			newRedis := &appsv1.Deployment{}
			assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-redis", Namespace: cr.Namespace}, newRedis))
			assert.Equal(t, newRedis.Spec.Template.Spec.ServiceAccountName, test.expectedSA)
		})
	}
}

func operationProcessors(n int32) argoCDOpt {
	return func(a *argoproj.ArgoCD) {
		a.Spec.Controller.Processors.Operation = n
	}
}

func Test_UpdateNodePlacement(t *testing.T) {

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-sample-server",
			Namespace: testNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{
						"test_key1": "test_value1",
						"test_key2": "test_value2",
					},
					Tolerations: []corev1.Toleration{
						{
							Key:    "test_key1",
							Value:  "test_value1",
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
		},
	}
	deployment2 := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-sample-server",
			Namespace: testNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{
						"test_key1": "test_value1",
					},
					Tolerations: []corev1.Toleration{
						{
							Key:    "test_key1",
							Value:  "test_value1",
							Effect: corev1.TaintEffectNoExecute,
						},
					},
				},
			},
		},
	}

	actualChanges := updateNodePlacement(deployment, deployment)
	assert.Empty(t, actualChanges, "updateNodePlacement returned unexpected changes")

	actualChanges = updateNodePlacement(deployment, deployment2)
	assert.Equal(t, []string{"node selector", "tolerations"}, actualChanges, "updateNodePlacement returned unexpected changes")
}

func assertDeploymentHasProxyVars(t *testing.T, c client.Client, name string) {
	t.Helper()
	deployment := &appsv1.Deployment{}
	err := c.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: testNamespace,
	}, deployment)
	assert.NoError(t, err)

	want := []corev1.EnvVar{
		{Name: "HTTP_PROXY", Value: testHTTPProxy},
		{Name: "HTTPS_PROXY", Value: testHTTPSProxy},
		{Name: "no_proxy", Value: testNoProxy},
	}
	for _, c := range deployment.Spec.Template.Spec.Containers {
		for _, w := range want {
			assert.Contains(t, c.Env, w)
		}
	}
	for _, c := range deployment.Spec.Template.Spec.InitContainers {
		assert.Len(t, c.Env, len(want))
		for _, w := range want {
			assert.Contains(t, c.Env, w)
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
	assert.NoError(t, err)

	names := []string{"http_proxy", "https_proxy", "no_proxy"}
	for _, name := range names {
		for _, c := range deployment.Spec.Template.Spec.Containers {
			for _, envVar := range c.Env {
				assert.NotEqual(t, strings.ToLower(envVar.Name), name)
			}
		}
		for _, c := range deployment.Spec.Template.Spec.InitContainers {
			for _, envVar := range c.Env {
				assert.NotEqual(t, strings.ToLower(envVar.Name), name)
			}
		}
	}
}

func assertNotFound(t *testing.T, err error) {
	t.Helper()
	assert.True(t, apierrors.IsNotFound(err))
}

func controllerProcessors(n int32) argoCDOpt {
	return func(a *argoproj.ArgoCD) {
		a.Spec.Controller.Processors.Status = n
	}
}

// repoServerVolumes returns the list of expected default volumes for the repo server
func repoServerDefaultVolumes() []corev1.Volume {
	volumes := []corev1.Volume{
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
			Name: "gpg-keys",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDGPGKeysConfigMapName,
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
		{
			Name: "argocd-repo-server-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRepoServerTLSSecretName,
					Optional:   boolPtr(true),
				},
			},
		},
		{
			Name: common.ArgoCDRedisServerTLSSecretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRedisServerTLSSecretName,
					Optional:   boolPtr(true),
				},
			},
		},
		{
			Name: "var-files",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "plugins",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "tmp",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	return volumes
}

// repoServerDefaultVolumeMounts return the default volume mounts for the repo server
func repoServerDefaultVolumeMounts() []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{Name: "ssh-known-hosts", MountPath: "/app/config/ssh"},
		{Name: "tls-certs", MountPath: "/app/config/tls"},
		{Name: "gpg-keys", MountPath: "/app/config/gpg/source"},
		{Name: "gpg-keyring", MountPath: "/app/config/gpg/keys"},
		{Name: "argocd-repo-server-tls", MountPath: "/app/config/reposerver/tls"},
		{Name: common.ArgoCDRedisServerTLSSecretName, MountPath: "/app/config/reposerver/tls/redis"},
		{Name: "plugins", MountPath: "/home/argocd/cmp-server/plugins"},
		{Name: "tmp", MountPath: "/tmp"},
	}
	return mounts
}

func serverDefaultVolumes() []corev1.Volume {
	volumes := []corev1.Volume{
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
			Name: "argocd-repo-server-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRepoServerTLSSecretName,
					Optional:   boolPtr(true),
				},
			},
		},
		{
			Name: common.ArgoCDRedisServerTLSSecretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRedisServerTLSSecretName,
					Optional:   boolPtr(true),
				},
			},
		},
		{
			Name: "plugins-home",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "argocd-cmd-params-cm",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "argocd-cmd-params-cm",
					},
					Optional: boolPtr(true),
					Items: []corev1.KeyToPath{
						{
							Key:  "server.profile.enabled",
							Path: "profiler.enabled",
						},
					},
				},
			},
		},
		{
			Name: "tmp",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	return volumes
}

func serverDefaultVolumeMounts() []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      "ssh-known-hosts",
			MountPath: "/app/config/ssh",
		}, {
			Name:      "tls-certs",
			MountPath: "/app/config/tls",
		}, {
			Name:      "argocd-repo-server-tls",
			MountPath: "/app/config/server/tls",
		}, {
			Name:      common.ArgoCDRedisServerTLSSecretName,
			MountPath: "/app/config/server/tls/redis",
		},
		{
			Name:      "plugins-home",
			MountPath: "/home/argocd",
		},
		{
			Name:      "argocd-cmd-params-cm",
			MountPath: "/home/argocd/params",
		},
		{
			Name:      "tmp",
			MountPath: "/tmp",
		},
	}
	return mounts
}

func TestReconcileArgoCD_reconcile_RepoServerChanges(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name           string
		mountSAToken   bool
		serviceAccount string
	}{
		{
			name:           "default Deployment",
			mountSAToken:   false,
			serviceAccount: "default",
		},
		{
			name:           "change Service Account and mountSAToken",
			mountSAToken:   true,
			serviceAccount: "argocd-argocd-server",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.Repo.MountSAToken = test.mountSAToken
				a.Spec.Repo.ServiceAccount = test.serviceAccount
			})

			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      test.serviceAccount,
					Namespace: a.Namespace,
					Labels:    argoutil.LabelsForCluster(a),
				},
			}
			err := r.Create(context.TODO(), sa)
			assert.NoError(t, err)
			err = r.reconcileRepoDeployment(a, false)
			assert.NoError(t, err)

			deployment := &appsv1.Deployment{}
			err = r.Get(context.TODO(), types.NamespacedName{
				Name:      "argocd-repo-server",
				Namespace: testNamespace,
			}, deployment)
			assert.NoError(t, err)
			assert.Equal(t, &test.mountSAToken, deployment.Spec.Template.Spec.AutomountServiceAccountToken)
			assert.Equal(t, test.serviceAccount, deployment.Spec.Template.Spec.ServiceAccountName)
		})
	}
}

func TestArgoCDRepoServerDeploymentCommand(t *testing.T) {
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	testRedisServerAddress := getRedisServerAddress(a)

	baseCommand := []string{
		"uid_entrypoint.sh",
		"argocd-repo-server",
		"--redis",
		testRedisServerAddress,
		"--loglevel",
		"info",
		"--logformat",
		"text",
	}

	// When a single command argument is passed
	a.Spec.Repo.ExtraRepoCommandArgs = []string{
		"--reposerver.max.combined.directory.manifests.size",
		"10M",
	}

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.reconcileRepoDeployment(a, false))

	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-repo-server",
			Namespace: a.Namespace,
		},
		deployment))

	cmd := append(baseCommand,
		"--reposerver.max.combined.directory.manifests.size", "10M")
	assert.Equal(t, cmd, deployment.Spec.Template.Spec.Containers[0].Command)

	// When multiple command arguments are passed
	a.Spec.Repo.ExtraRepoCommandArgs = []string{
		"--reposerver.max.combined.directory.manifests.size",
		"10M",
		"--foo",
		"bar",
		"test",
	}

	assert.NoError(t, r.reconcileRepoDeployment(a, false))
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-repo-server",
			Namespace: a.Namespace,
		},
		deployment))

	cmd = append(cmd, "--foo", "bar", "test")
	assert.Equal(t, cmd, deployment.Spec.Template.Spec.Containers[0].Command)

	// When one of the ExtraCommandArgs already exists in cmd with same or different value
	a.Spec.Repo.ExtraRepoCommandArgs = []string{
		"--redis",
		"foo.scv.cluster.local:6379",
	}

	wantCmd := []string{
		"uid_entrypoint.sh",
		"argocd-repo-server",
		"--loglevel",
		"info",
		"--logformat",
		"text",
		"--redis",
		"foo.scv.cluster.local:6379",
	}

	assert.NoError(t, r.reconcileRepoDeployment(a, false))
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-repo-server",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Equal(t, wantCmd, deployment.Spec.Template.Spec.Containers[0].Command)

	// Remove all the command arguments that were added.
	a.Spec.Repo.ExtraRepoCommandArgs = []string{}

	assert.NoError(t, r.reconcileRepoDeployment(a, false))
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-repo-server",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Equal(t, baseCommand, deployment.Spec.Template.Spec.Containers[0].Command)
}

func TestReconcileArgoCD_reconcileRepoDeployment_serviceAccount(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		testName                      string
		serviceAccountName            string
		expectedServiceAccountName    string
		isServiceAccountNameChanged   bool
		newServiceAccountName         string
		newExpectedServiceAccountName string
	}{
		{
			testName:                   "serviceAccountName field in the spec should reflect provided value",
			serviceAccountName:         "deployer",
			expectedServiceAccountName: "deployer",
		}, {
			testName:                      "serviceAccountName field in the spec should have updated value",
			serviceAccountName:            "deployer",
			expectedServiceAccountName:    "deployer",
			isServiceAccountNameChanged:   true,
			newServiceAccountName:         "builder",
			newExpectedServiceAccountName: "builder",
		}, {
			testName:                      "Empty serviceAccountName field in the spec should have updated value",
			serviceAccountName:            "",
			expectedServiceAccountName:    "",
			isServiceAccountNameChanged:   true,
			newServiceAccountName:         "builder",
			newExpectedServiceAccountName: "builder",
		}, {
			testName:                      "serviceAccountName field in the spec should be changed to empty",
			serviceAccountName:            "builder",
			expectedServiceAccountName:    "builder",
			isServiceAccountNameChanged:   true,
			newServiceAccountName:         "",
			newExpectedServiceAccountName: "",
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {

			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.Repo.ServiceAccount = test.serviceAccountName
			})

			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			err := r.reconcileRepoDeployment(a, false)
			assert.NoError(t, err)

			deployment := &appsv1.Deployment{}
			key := types.NamespacedName{
				Name:      "argocd-repo-server",
				Namespace: testNamespace,
			}

			err = r.Get(context.TODO(), key, deployment)

			assert.NoError(t, err)
			assert.Equal(t, test.expectedServiceAccountName, deployment.Spec.Template.Spec.ServiceAccountName)

			// check if SA name is changed
			if test.isServiceAccountNameChanged {

				a.Spec.Repo.ServiceAccount = test.newServiceAccountName

				err = r.reconcileRepoDeployment(a, false)
				assert.NoError(t, err)

				err = r.Get(context.TODO(), key, deployment)

				assert.NoError(t, err)
				assert.Equal(t, test.newExpectedServiceAccountName, deployment.Spec.Template.Spec.ServiceAccountName)
			}
		})
	}
}

func TestReconcileArgoCD_reconcileRepoDeployment_sidecarContainerImage(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Repo.SidecarContainers = []corev1.Container{
			{
				Name: "test-sidecar1",
			},
			{
				Name: "test-sidecar2",
			},
			{
				Name:  "test-sidecar3",
				Image: "test-image",
			},
		}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.reconcileRepoDeployment(a, false))

	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-repo-server",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Len(t, deployment.Spec.Template.Spec.Containers, 4)
	assert.Equal(t, "test-sidecar1", deployment.Spec.Template.Spec.Containers[1].Name)
	assert.Equal(t, getRepoServerContainerImage(a), deployment.Spec.Template.Spec.Containers[1].Image)
	assert.Equal(t, "test-sidecar2", deployment.Spec.Template.Spec.Containers[2].Name)
	assert.Equal(t, getRepoServerContainerImage(a), deployment.Spec.Template.Spec.Containers[2].Image)
	assert.Equal(t, "test-sidecar3", deployment.Spec.Template.Spec.Containers[3].Name)
	assert.Equal(t, "test-image", deployment.Spec.Template.Spec.Containers[3].Image)
}

// If `remote` field is used in CR, then the component resources should not be created
func TestReconcileArgoCD_reconcileRedisWithRemote(t *testing.T) {
	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{cr}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	redisRemote := "https://remote.redis.instance"

	cr.Spec.Redis.Remote = &redisRemote
	assert.NoError(t, r.reconcileRedisDeployment(cr, false))

	d := &appsv1.Deployment{}

	assert.ErrorContains(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-redis", Namespace: cr.Namespace}, d),
		"deployments.apps \""+cr.Name+"-redis\" not found")

	// once remote is set to nil, reconciliation should trigger deployment resource creation
	cr.Spec.Redis.Remote = nil

	assert.NoError(t, r.reconcileRedisDeployment(cr, false))
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-redis", Namespace: cr.Namespace}, d))
}

func TestReconcileArgoCD_reconcileRepoServerWithRemote(t *testing.T) {
	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{cr}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	repoServerRemote := "https://remote.repo-server.instance"

	cr.Spec.Repo.Remote = &repoServerRemote
	assert.NoError(t, r.reconcileRepoDeployment(cr, false))

	d := &appsv1.Deployment{}

	assert.ErrorContains(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-repo-server", Namespace: cr.Namespace}, d),
		"deployments.apps \""+cr.Name+"-repo-server\" not found")

	// once remote is set to nil, reconciliation should trigger deployment resource creation
	cr.Spec.Repo.Remote = nil

	assert.NoError(t, r.reconcileRepoDeployment(cr, false))
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-repo-server", Namespace: cr.Namespace}, d))
}

func Test_getRolloutInitContainer(t *testing.T) {
	tests := []struct {
		name      string
		envSet    bool
		wantImage string
		wantEnv   []corev1.EnvVar
	}{
		{
			name:      "when running in argocd-operator",
			envSet:    false,
			wantImage: "quay.io/argoprojlabs/argocd-extension-installer:v0.0.8",
			wantEnv: []corev1.EnvVar{
				{
					Name:  "EXTENSION_URL",
					Value: common.ArgoRolloutsExtensionURL,
				},
			},
		},
		{
			name:      "when running in gitops-operator",
			envSet:    true,
			wantImage: "updated_container",
			wantEnv:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envSet {
				err := os.Setenv(common.ArgoCDExtensionImageEnvName, "updated_container")
				assert.NoError(t, err)
			}

			containers := getRolloutInitContainer()

			assert.Equalf(t, tt.wantImage, containers[0].Image, "Image check")
			assert.Equalf(t, tt.wantEnv, containers[0].Env, "Env check")

		})
	}
}

func TestSetReplicasAndEnvVar_WhenServerReplicasIsDefined(t *testing.T) {
	t.Run("should set replicas and ARGOCD_API_SERVER_REPLICAS env var when spec.server.replicas is set", func(t *testing.T) {
		logf.SetLogger(ZapLogger(true))
		a := makeTestArgoCD()
		var replicas *int32
		v := int32(2)
		replicas = &v
		a.Spec.Server.Replicas = replicas

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		err := r.reconcileServerDeployment(a, false)
		assert.NoError(t, err)
		deployment := &appsv1.Deployment{}
		err = r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-server",
			Namespace: testNamespace,
		}, deployment)
		assert.NoError(t, err)

		// Check that the env vars are set, Count is 2 because of the default REDIS_PASSWORD env var
		assert.Len(t, deployment.Spec.Template.Spec.Containers[0].Env, 2)
		assert.Contains(t, deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "ARGOCD_API_SERVER_REPLICAS", Value: "2"})
	})

}

func TestReconcileArgoCD_reconcileRepoServerWithFipsEnabled(t *testing.T) {
	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{cr}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
	r.FipsConfigChecker = &MockTrueFipsChecker{}
	repoServerRemote := "https://remote.repo-server.instance"

	cr.Spec.Repo.Remote = &repoServerRemote
	assert.NoError(t, r.reconcileRepoDeployment(cr, false))

	d := &appsv1.Deployment{}

	assert.ErrorContains(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-repo-server", Namespace: cr.Namespace}, d),
		"deployments.apps \""+cr.Name+"-repo-server\" not found")

	// once remote is set to nil, reconciliation should trigger deployment resource creation
	cr.Spec.Repo.Remote = nil

	assert.NoError(t, r.reconcileRepoDeployment(cr, false))
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-repo-server", Namespace: cr.Namespace}, d))
	foundEnv := false
	for _, env := range d.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "GODEBUG" {
			foundEnv = true
			assert.Equal(t, env.Value, "fips140=on", "GODEBUG environment must be set to fips140=on when fips is enabled")
		}
	}
	assert.True(t, foundEnv, "environment GODEBUG must be set when FIPS is enabled")
}

func TestReconcileArgoCD_reconcileRepoServerWithFipsDisabled(t *testing.T) {
	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{cr}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
	r.FipsConfigChecker = &MockFalseFipsChecker{}
	repoServerRemote := "https://remote.repo-server.instance"

	cr.Spec.Repo.Remote = &repoServerRemote
	assert.NoError(t, r.reconcileRepoDeployment(cr, false))

	d := &appsv1.Deployment{}

	assert.ErrorContains(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-repo-server", Namespace: cr.Namespace}, d),
		"deployments.apps \""+cr.Name+"-repo-server\" not found")

	// once remote is set to nil, reconciliation should trigger deployment resource creation
	cr.Spec.Repo.Remote = nil

	assert.NoError(t, r.reconcileRepoDeployment(cr, false))
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-repo-server", Namespace: cr.Namespace}, d))
	foundEnv := false
	for _, env := range d.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "GODEBUG" {
			foundEnv = true
		}
	}
	assert.False(t, foundEnv, "environment GODEBUG must NOT be set when FIPS is disabled")
}

func TestDeploymentWithLongName(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	// Create ArgoCD with a very long name that will trigger truncation
	longName := "this-is-a-very-long-argocd-instance-name-that-will-exceed-the-kubernetes-name-limit-and-require-truncation"
	a := makeTestArgoCD()
	a.Name = longName

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	// Test repo server deployment
	err := r.reconcileRepoDeployment(a, false)
	assert.NoError(t, err)

	// Get all deployments and find the repo server deployment
	deploymentList := &appsv1.DeploymentList{}
	err = r.List(context.TODO(), deploymentList, client.InNamespace(a.Namespace))
	assert.NoError(t, err)

	var repoDeployment *appsv1.Deployment
	for i := range deploymentList.Items {
		if deploymentList.Items[i].Labels[common.ArgoCDKeyComponent] == "repo-server" {
			repoDeployment = &deploymentList.Items[i]
			break
		}
	}
	assert.NotNil(t, repoDeployment, "Repo server deployment should exist")

	// Verify that the deployment name is truncated and within limits
	assert.LessOrEqual(t, len(repoDeployment.Name), 63)
	assert.Contains(t, repoDeployment.Name, "repo-server")

	// Verify that the labels are set correctly
	assert.Equal(t, repoDeployment.Name, repoDeployment.Labels[common.ArgoCDKeyName])
	assert.Equal(t, "repo-server", repoDeployment.Labels[common.ArgoCDKeyComponent])

	// Verify that the selector matches the labels
	assert.Equal(t, repoDeployment.Name, repoDeployment.Spec.Selector.MatchLabels[common.ArgoCDKeyName])

	// Verify that the pod template labels match
	assert.Equal(t, repoDeployment.Name, repoDeployment.Spec.Template.Labels[common.ArgoCDKeyName])
}
