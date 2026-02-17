package argocd

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/kubernetes/fake"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/argoproj-labs/argocd-operator/common"

	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	testRedisImage        = "redis"
	testRedisImageVersion = "test"
)

func controllerDefaultVolumes() []corev1.Volume {
	volumes := []corev1.Volume{
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
			Name: "argocd-home",
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
							Key:  "controller.profile.enabled",
							Path: "profiler.enabled",
						},
						{
							Key:  "controller.resource.health.persist",
							Path: "controller.resource.health.persist",
						},
					},
				},
			},
		},
		{
			Name: "argocd-application-controller-tmp",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	return volumes
}

func controllerDefaultVolumeMounts() []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      "argocd-repo-server-tls",
			MountPath: "/app/config/controller/tls",
		},
		{
			Name:      common.ArgoCDRedisServerTLSSecretName,
			MountPath: "/app/config/controller/tls/redis",
		},
		{
			Name:      "argocd-home",
			MountPath: "/home/argocd",
		},
		{
			Name:      "argocd-cmd-params-cm",
			MountPath: "/home/argocd/params",
		},
		{
			Name:      "argocd-application-controller-tmp",
			MountPath: "/tmp",
		},
	}
	return mounts
}

func TestReconcileArgoCD_reconcileRedisStatefulSet_HA_disabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	s := newStatefulSetWithSuffix("redis-ha-server", "redis", a)

	assert.NoError(t, r.reconcileRedisStatefulSet(a))
	// resource Creation should fail as HA was disabled
	assert.Errorf(t, r.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s), "not found")
}

func TestReconcileArgoCD_reconcileRedisStatefulSet_HA_enabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	s := newStatefulSetWithSuffix("redis-ha-server", "redis", a)

	a.Spec.HA.Enabled = true
	// test resource is Created when HA is enabled
	assert.NoError(t, r.reconcileRedisStatefulSet(a))
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s))

	// test resource is Updated on reconciliation
	a.Spec.Redis.Image = testRedisImage
	a.Spec.Redis.Version = testRedisImageVersion
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
	assert.NoError(t, r.reconcileRedisStatefulSet(a))
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s))
	for _, container := range s.Spec.Template.Spec.Containers {
		assert.Equal(t, container.Image, fmt.Sprintf("%s:%s", testRedisImage, testRedisImageVersion))
		assert.Equal(t, container.Resources, newResources)
	}
	assert.Equal(t, s.Spec.Template.Spec.InitContainers[0].Resources, newResources)

	// test resource is Deleted, when HA is disabled
	a.Spec.HA.Enabled = false
	assert.NoError(t, r.reconcileRedisStatefulSet(a))
	assert.Errorf(t, r.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s), "not found")
}

func TestReconcileArgoCD_reconcileApplicationController(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))

	ss := &appsv1.StatefulSet{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-application-controller",
			Namespace: a.Namespace,
		},
		ss))
	command := ss.Spec.Template.Spec.Containers[0].Command
	want := []string{
		"argocd-application-controller",
		"--operation-processors", "10",
		"--redis", "argocd-redis.argocd.svc.cluster.local:6379",
		"--repo-server", "argocd-repo-server.argocd.svc.cluster.local:8081",
		"--status-processors", "20",
		"--kubectl-parallelism-limit", "10",
		"--loglevel", "info",
		"--logformat", "text",
		"--persist-resource-health",
	}
	if diff := cmp.Diff(want, command); diff != "" {
		t.Fatalf("reconciliation failed:\n%s", diff)
	}
	wantVolumes := controllerDefaultVolumes()
	if diff := cmp.Diff(wantVolumes, ss.Spec.Template.Spec.Volumes); diff != "" {
		t.Fatalf("reconciliation failed:\n%s", diff)
	}
	wantVolumeMounts := controllerDefaultVolumeMounts()
	if diff := cmp.Diff(wantVolumeMounts, ss.Spec.Template.Spec.Containers[0].VolumeMounts); diff != "" {
		t.Fatalf("reconciliation failed:\n%s", diff)
	}
}

func TestReconcileArgoCD_reconcileApplicationController_withRedisTLS(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, true))

	ss := &appsv1.StatefulSet{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-application-controller",
			Namespace: a.Namespace,
		},
		ss))
	command := ss.Spec.Template.Spec.Containers[0].Command
	want := []string{
		"argocd-application-controller",
		"--operation-processors", "10",
		"--redis", "argocd-redis.argocd.svc.cluster.local:6379",
		"--redis-use-tls",
		"--redis-ca-certificate", "/app/config/controller/tls/redis/tls.crt",
		"--repo-server", "argocd-repo-server.argocd.svc.cluster.local:8081",
		"--status-processors", "20",
		"--kubectl-parallelism-limit", "10",
		"--loglevel", "info",
		"--logformat", "text",
		"--persist-resource-health"}
	if diff := cmp.Diff(want, command); diff != "" {
		t.Fatalf("reconciliation failed:\n%s", diff)
	}
}

func TestReconcileArgoCD_reconcileApplicationController_withUpdate(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))

	a = makeTestArgoCD(controllerProcessors(30))
	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))

	ss := &appsv1.StatefulSet{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-application-controller",
			Namespace: a.Namespace,
		},
		ss))
	command := ss.Spec.Template.Spec.Containers[0].Command
	want := []string{
		"argocd-application-controller",
		"--operation-processors", "10",
		"--redis", "argocd-redis.argocd.svc.cluster.local:6379",
		"--repo-server", "argocd-repo-server.argocd.svc.cluster.local:8081",
		"--status-processors", "30",
		"--kubectl-parallelism-limit", "10",
		"--loglevel", "info",
		"--logformat", "text",
		"--persist-resource-health"}
	if diff := cmp.Diff(want, command); diff != "" {
		t.Fatalf("reconciliation failed:\n%s", diff)
	}
}

func TestReconcileArgoCD_reconcileApplicationController_withUpgrade(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	deploy := newDeploymentWithSuffix("application-controller", "application-controller", a)
	assert.NoError(t, r.Create(context.TODO(), deploy))

	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))
	err := r.Get(context.TODO(), types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, deploy)
	assert.Errorf(t, err, "not found")
}

func TestReconcileArgoCD_reconcileApplicationController_withResources(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCDWithResources(func(a *argoproj.ArgoCD) {
		a.Spec.Import = &argoproj.ArgoCDImportSpec{
			Name: "testimport",
		}
	})
	ex := argoprojv1alpha1.ArgoCDExport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testimport",
			Namespace: a.Namespace,
		},
		Spec: argoprojv1alpha1.ArgoCDExportSpec{
			Storage: &argoprojv1alpha1.ArgoCDExportStorageSpec{},
		},
	}

	resObjs := []client.Object{a, &ex}
	subresObjs := []client.Object{a, &ex}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, argoprojv1alpha1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))

	ss := &appsv1.StatefulSet{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-application-controller",
			Namespace: a.Namespace,
		},
		ss))

	testResources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("1024Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("1000m"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("2048Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("2000m"),
		},
	}
	rsC := ss.Spec.Template.Spec.Containers[0].Resources
	assert.True(t, testResources.Requests.Cpu().Equal(*rsC.Requests.Cpu()))
	assert.True(t, testResources.Requests.Memory().Equal(*rsC.Requests.Memory()))
	assert.True(t, testResources.Limits.Cpu().Equal(*rsC.Limits.Cpu()))
	assert.True(t, testResources.Limits.Memory().Equal(*rsC.Limits.Memory()))

	// Negative test - differing limits and requests
	testResources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("2024Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("2000m"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("3048Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("1000m"),
		},
	}
	assert.False(t, testResources.Requests.Cpu().Equal(*rsC.Requests.Cpu()))
	assert.False(t, testResources.Requests.Memory().Equal(*rsC.Requests.Memory()))
	assert.False(t, testResources.Limits.Cpu().Equal(*rsC.Limits.Cpu()))
	assert.False(t, testResources.Limits.Memory().Equal(*rsC.Limits.Memory()))
}

func TestReconcileArgoCD_reconcileApplicationController_withSharding(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		sharding argoproj.ArgoCDApplicationControllerShardSpec
		replicas int32
		vars     []corev1.EnvVar
	}{
		{
			sharding: argoproj.ArgoCDApplicationControllerShardSpec{
				Enabled:  false,
				Replicas: 3,
			},
			replicas: 1,
			vars: []corev1.EnvVar{
				{Name: "ARGOCD_CONTROLLER_RESOURCE_HEALTH_PERSIST", ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: common.ArgoCDCmdParamsConfigMapName},
						Key:                  "controller.resource.health.persist",
					},
				}},
				{Name: "ARGOCD_RECONCILIATION_TIMEOUT", ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: common.ArgoCDConfigMapName},
						Key:                  common.ArgoCDKeyTimeout,
						Optional:             boolPtr(true),
					},
				}},
				{Name: "HOME", Value: "/home/argocd"},
				{Name: "REDIS_PASSWORD", Value: "",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "argocd-redis-initial-password",
							},
							Key: "admin.password",
						},
					}},
			},
		},
		{
			sharding: argoproj.ArgoCDApplicationControllerShardSpec{
				Enabled:  true,
				Replicas: 1,
			},
			replicas: 1,
			vars: []corev1.EnvVar{
				{Name: "ARGOCD_CONTROLLER_REPLICAS", Value: "1"},
				{Name: "ARGOCD_CONTROLLER_RESOURCE_HEALTH_PERSIST", ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: common.ArgoCDCmdParamsConfigMapName},
						Key:                  "controller.resource.health.persist",
					},
				}},
				{Name: "ARGOCD_RECONCILIATION_TIMEOUT", ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: common.ArgoCDConfigMapName},
						Key:                  common.ArgoCDKeyTimeout,
						Optional:             boolPtr(true),
					},
				}},
				{Name: "HOME", Value: "/home/argocd"},
				{Name: "REDIS_PASSWORD", Value: "",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "argocd-redis-initial-password",
							},
							Key: "admin.password",
						},
					}},
			},
		},
		{
			sharding: argoproj.ArgoCDApplicationControllerShardSpec{
				Enabled:  true,
				Replicas: 3,
			},
			replicas: 3,
			vars: []corev1.EnvVar{
				{Name: "ARGOCD_CONTROLLER_REPLICAS", Value: "3"},
				{Name: "ARGOCD_CONTROLLER_RESOURCE_HEALTH_PERSIST", ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: common.ArgoCDCmdParamsConfigMapName},
						Key:                  "controller.resource.health.persist",
					},
				}},
				{Name: "ARGOCD_RECONCILIATION_TIMEOUT", ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: common.ArgoCDConfigMapName},
						Key:                  common.ArgoCDKeyTimeout,
						Optional:             boolPtr(true),
					},
				}},
				{Name: "HOME", Value: "/home/argocd"},
				{Name: "REDIS_PASSWORD", Value: "",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "argocd-redis-initial-password",
							},
							Key: "admin.password",
						},
					}},
			},
		},
		{
			sharding: argoproj.ArgoCDApplicationControllerShardSpec{
				DynamicScalingEnabled: boolPtr(true),
				MinShards:             2,
				MaxShards:             4,
				ClustersPerShard:      1,
			},
			replicas: 2,
			vars: []corev1.EnvVar{
				{Name: "ARGOCD_CONTROLLER_REPLICAS", Value: "2"},
				{Name: "ARGOCD_CONTROLLER_RESOURCE_HEALTH_PERSIST", ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: common.ArgoCDCmdParamsConfigMapName},
						Key:                  "controller.resource.health.persist",
					},
				}},
				{Name: "ARGOCD_RECONCILIATION_TIMEOUT", ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: common.ArgoCDConfigMapName},
						Key:                  common.ArgoCDKeyTimeout,
						Optional:             boolPtr(true),
					},
				}},
				{Name: "HOME", Value: "/home/argocd"},
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
		},
	}

	for _, st := range tests {
		a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
			a.Spec.Controller.Sharding = st.sharding
		})

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))

		ss := &appsv1.StatefulSet{}
		assert.NoError(t, r.Get(
			context.TODO(),
			types.NamespacedName{
				Name:      "argocd-application-controller",
				Namespace: a.Namespace,
			},
			ss))

		env := ss.Spec.Template.Spec.Containers[0].Env
		rep := ss.Spec.Replicas

		diffEnv := cmp.Diff(env, st.vars)
		diffRep := cmp.Diff(rep, &st.replicas)

		if diffEnv != "" {
			t.Fatalf("Reconciliation of EnvVars failed:\n%s", diffEnv)
		}

		if diffRep != "" {
			t.Fatalf("Reconciliation of Replicas failed:\n%s", diffRep)
		}
	}
}

func TestReconcileArgoCD_reconcileApplicationController_withAppSync(t *testing.T) {

	expectedEnv := []corev1.EnvVar{
		{Name: "ARGOCD_CONTROLLER_RESOURCE_HEALTH_PERSIST", ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: common.ArgoCDCmdParamsConfigMapName},
				Key:                  "controller.resource.health.persist",
			},
		}},
		{Name: "ARGOCD_RECONCILIATION_TIMEOUT", Value: "600s"},
		{Name: "HOME", Value: "/home/argocd"},
		{Name: "REDIS_PASSWORD", Value: "",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "argocd-redis-initial-password",
					},
					Key: "admin.password",
				},
			}},
	}

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Controller.AppSync = &metav1.Duration{Duration: time.Minute * 10}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))

	ss := &appsv1.StatefulSet{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-application-controller",
			Namespace: a.Namespace,
		},
		ss))

	env := ss.Spec.Template.Spec.Containers[0].Env

	diffEnv := cmp.Diff(env, expectedEnv)

	if diffEnv != "" {
		t.Fatalf("Reconciliation of EnvVars failed:\n%s", diffEnv)
	}
}

func TestReconcileArgoCD_reconcileApplicationController_withEnv(t *testing.T) {

	expectedEnv := []corev1.EnvVar{
		{Name: "ARGOCD_CONTROLLER_RESOURCE_HEALTH_PERSIST", ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: common.ArgoCDCmdParamsConfigMapName},
				Key:                  "controller.resource.health.persist",
			},
		}},
		{Name: "ARGOCD_RECONCILIATION_TIMEOUT", ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: common.ArgoCDConfigMapName},
				Key:                  common.ArgoCDKeyTimeout,
				Optional:             boolPtr(true),
			},
		}},
		{Name: "CUSTOM_ENV_VAR", Value: "custom-value"},
		{Name: "HOME", Value: "/home/argocd"},
		{Name: "REDIS_PASSWORD", Value: "",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "argocd-redis-initial-password",
					},
					Key: "admin.password",
				},
			}},
	}

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		// Assuming spec.controller.env is a slice
		a.Spec.Controller.Env = []corev1.EnvVar{
			{Name: "CUSTOM_ENV_VAR", Value: "custom-value"},
		}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))

	ss := &appsv1.StatefulSet{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-application-controller",
			Namespace: a.Namespace,
		},
		ss))

	env := ss.Spec.Template.Spec.Containers[0].Env

	diffEnv := cmp.Diff(env, expectedEnv)

	if diffEnv != "" {
		t.Fatalf("Reconciliation of EnvVars failed:\n%s", diffEnv)
	}
}

func Test_UpdateNodePlacementStateful(t *testing.T) {

	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-sample-server",
			Namespace: testNamespace,
		},
		Spec: appsv1.StatefulSetSpec{
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
	ss2 := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-sample-server",
			Namespace: testNamespace,
		},
		Spec: appsv1.StatefulSetSpec{
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
	expectedChange := false
	actualChange := false
	explanation := ""
	updateNodePlacementStateful(ss, ss, &actualChange, &explanation)
	if actualChange != expectedChange {
		t.Fatalf("updateNodePlacementStateful failed, value of changed: %t", actualChange)
	}
	if explanation != "" {
		t.Fatalf("updateNodePlacementStateful returned unexpected explanation: '%s'", explanation)
	}
	updateNodePlacementStateful(ss, ss2, &actualChange, &explanation)
	if actualChange == expectedChange {
		t.Fatalf("updateNodePlacementStateful failed, value of changed: %t", actualChange)
	}
	if explanation != "node selector, tolerations" {
		t.Fatalf("updateNodePlacementStateful returned unexpected explanation: '%s'", explanation)
	}
}

func Test_ContainsInvalidImage(t *testing.T) {

	a := makeTestArgoCD()
	po := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argo-cd-application-controller",
			Namespace: a.Namespace,
			Labels: map[string]string{
				common.ArgoCDKeyName: fmt.Sprintf("%s-%s", a.Name, "application-controller"),
			},
		},
	}
	objs := []client.Object{
		po,
		a,
	}

	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, objs, objs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	// Test that containsInvalidImage returns false if there is nothing wrong with the Pod
	containsInvalidImageRes, err := containsInvalidImage(*a, *r)
	assert.NoError(t, err)
	if containsInvalidImageRes {
		t.Fatalf("containsInvalidImage failed, got true, expected false")
	}

	// Test that containsInvalidImage returns true if the Pod is in ErrImagePull
	po.Status.ContainerStatuses = []corev1.ContainerStatus{
		{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ErrImagePull"}}}}
	err = cl.Status().Update(context.Background(), po)
	assert.NoError(t, err)

	containsInvalidImageRes, err = containsInvalidImage(*a, *r)
	assert.NoError(t, err)
	assert.True(t, containsInvalidImageRes)

}

func TestReconcileArgoCD_reconcileApplicationController_withDynamicSharding(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		sharding         argoproj.ArgoCDApplicationControllerShardSpec
		expectedReplicas int32
		vars             []corev1.EnvVar
	}{
		{
			sharding: argoproj.ArgoCDApplicationControllerShardSpec{
				Enabled:               false,
				Replicas:              1,
				DynamicScalingEnabled: boolPtr(true),
				MinShards:             2,
				MaxShards:             4,
				ClustersPerShard:      1,
			},
			expectedReplicas: 3,
		},
		{
			// Replicas less than minimum shards
			sharding: argoproj.ArgoCDApplicationControllerShardSpec{
				Enabled:               false,
				Replicas:              1,
				DynamicScalingEnabled: boolPtr(true),
				MinShards:             1,
				MaxShards:             4,
				ClustersPerShard:      3,
			},
			expectedReplicas: 1,
		},
		{
			// Replicas more than maximum shards
			sharding: argoproj.ArgoCDApplicationControllerShardSpec{
				Enabled:               false,
				Replicas:              1,
				DynamicScalingEnabled: boolPtr(true),
				MinShards:             1,
				MaxShards:             2,
				ClustersPerShard:      1,
			},
			expectedReplicas: 2,
		},
	}

	for _, st := range tests {
		a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
			a.Spec.Controller.Sharding = st.sharding
		})

		clusterSecret1 := argoutil.NewSecretWithSuffix(a, "cluster1")
		clusterSecret1.Labels = map[string]string{common.ArgoCDSecretTypeLabel: "cluster"}

		clusterSecret2 := argoutil.NewSecretWithSuffix(a, "cluster2")
		clusterSecret2.Labels = map[string]string{common.ArgoCDSecretTypeLabel: "cluster"}

		clusterSecret3 := argoutil.NewSecretWithSuffix(a, "cluster3")
		clusterSecret3.Labels = map[string]string{common.ArgoCDSecretTypeLabel: "cluster"}

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		assert.NoError(t, r.Create(context.TODO(), clusterSecret1))
		assert.NoError(t, r.Create(context.TODO(), clusterSecret2))
		assert.NoError(t, r.Create(context.TODO(), clusterSecret3))

		replicas := r.getApplicationControllerReplicaCount(a)

		assert.Equal(t, st.expectedReplicas, replicas)

	}
}

func TestReconcileAppController_Initcontainer(t *testing.T) {
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Controller.InitContainers = []corev1.Container{
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

	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))

	ss := &appsv1.StatefulSet{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-application-controller",
			Namespace: a.Namespace,
		},
		ss))

	assert.Equal(t, 1, len(ss.Spec.Template.Spec.InitContainers))
	assert.Equal(t, "test-init-container", ss.Spec.Template.Spec.InitContainers[0].Name)
	assert.Equal(t, "test-image", ss.Spec.Template.Spec.InitContainers[0].Image)

	// Remove InitContainers
	a.Spec.Controller.InitContainers = nil
	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))

	ss = &appsv1.StatefulSet{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-application-controller",
			Namespace: a.Namespace,
		},
		ss))

	assert.Equal(t, 0, len(ss.Spec.Template.Spec.InitContainers))
}

func TestReconcileArgoCD_sidecarcontainer(t *testing.T) {
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Controller.SidecarContainers = []corev1.Container{
			{
				Name:  "test-sidecar-container",
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

	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))

	ss := &appsv1.StatefulSet{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-application-controller",
			Namespace: a.Namespace,
		},
		ss))

	assert.Equal(t, 2, len(ss.Spec.Template.Spec.Containers))
	assert.Equal(t, "test-sidecar-container", ss.Spec.Template.Spec.Containers[1].Name)
	assert.Equal(t, "test-image", ss.Spec.Template.Spec.Containers[1].Image)

	// Remove SidecarContainers
	a.Spec.Controller.SidecarContainers = nil
	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))

	ss = &appsv1.StatefulSet{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-application-controller",
			Namespace: a.Namespace,
		},
		ss))

	assert.Equal(t, 1, len(ss.Spec.Template.Spec.Containers))
}

func TestReconcileArgoCD_reconcileRedisStatefulSet_ModifyContainerSpec(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCD()
	a.Spec.HA.Enabled = true

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	// Initial reconciliation to create the StatefulSet
	assert.NoError(t, r.reconcileRedisStatefulSet(a))

	s := newStatefulSetWithSuffix("redis-ha-server", "redis", a)
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s))

	// Modify the container environment variable
	s.Spec.Template.Spec.Containers[0].Env = append(s.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
		Name:  "NEW_ENV_VAR",
		Value: "new-value",
	})
	assert.NoError(t, r.Update(context.TODO(), s))

	// Reconcile again and check if the environment variable is reverted
	assert.NoError(t, r.reconcileRedisStatefulSet(a))
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s))

	envVarFound := false
	for _, env := range s.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "NEW_ENV_VAR" {
			envVarFound = true
			break
		}
	}
	assert.False(t, envVarFound, "NEW_ENV_VAR should not be present")

	// Modify the SecurityContext
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s))
	expectedSecurityContext := s.Spec.Template.Spec.SecurityContext
	fsGroup := int64(2000)
	newSecurityContext := &corev1.PodSecurityContext{
		FSGroup: &fsGroup,
	}
	s.Spec.Template.Spec.SecurityContext = newSecurityContext
	assert.NoError(t, r.Update(context.TODO(), s))
	// Reconcile again and check if the SecurityContext is reverted
	assert.NoError(t, r.reconcileRedisStatefulSet(a))
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s))
	assert.Equal(t, true, reflect.DeepEqual(expectedSecurityContext, s.Spec.Template.Spec.SecurityContext))

	// Modify the InitContainer environment variable
	s.Spec.Template.Spec.InitContainers[0].Env = append(s.Spec.Template.Spec.InitContainers[0].Env, corev1.EnvVar{
		Name:  "NEW_ENV_VAR",
		Value: "new-value",
	})
	assert.NoError(t, r.Update(context.TODO(), s))

	// Reconcile again and check if the environment variable is reverted
	assert.NoError(t, r.reconcileRedisStatefulSet(a))
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s))

	envVarFound = false
	for _, env := range s.Spec.Template.Spec.InitContainers[0].Env {
		if env.Name == "NEW_ENV_VAR" {
			envVarFound = true
			break
		}
	}
	assert.False(t, envVarFound, "NEW_ENV_VAR should not be present")

	// Modify the container volume and volume mount
	s.Spec.Template.Spec.Containers[0].VolumeMounts = append(s.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      "new-volume",
		MountPath: "/new/path",
	})
	s.Spec.Template.Spec.Volumes = append(s.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "new-volume",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	assert.NoError(t, r.Update(context.TODO(), s))
	// Reconcile again and check if the volume mount is reverted
	assert.NoError(t, r.reconcileRedisStatefulSet(a))
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s))

	volumeMountFound := false
	for _, vm := range s.Spec.Template.Spec.Containers[0].VolumeMounts {
		if vm.Name == "new-volume" {
			volumeMountFound = true
			break
		}
	}
	assert.False(t, volumeMountFound, "new-volume should not be present in volume mounts")

	volumeFound := false
	for _, v := range s.Spec.Template.Spec.Volumes {
		if v.Name == "new-volume" {
			volumeFound = true
			break
		}
	}
	assert.False(t, volumeFound, "new-volume should not be present in volumes")

	// Modify the container imagePullPolicy
	s.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullNever
	s.Spec.Template.Spec.Containers[1].ImagePullPolicy = corev1.PullAlways

	assert.NoError(t, r.Update(context.TODO(), s))
	// Reconcile again and check
	assert.NoError(t, r.reconcileRedisStatefulSet(a))
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s))

	assert.Equal(t, corev1.PullIfNotPresent, s.Spec.Template.Spec.Containers[0].ImagePullPolicy)
	assert.Equal(t, corev1.PullIfNotPresent, s.Spec.Template.Spec.Containers[1].ImagePullPolicy)
}

func TestStatefulSetWithLongName(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	// Create ArgoCD with a very long name that will trigger truncation
	longName := "this-is-a-very-long-argocd-instance-name-that-will-exceed-the-kubernetes-name-limit-and-require-truncation"
	a := makeTestArgoCD()
	a.Name = longName

	// Enable HA and Redis to ensure the statefulset is created
	a.Spec.HA.Enabled = true
	enabled := true
	a.Spec.Redis = argoproj.ArgoCDRedisSpec{
		Enabled: &enabled,
	}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	// Test Redis HA StatefulSet
	err := r.reconcileRedisStatefulSet(a)
	assert.NoError(t, err)

	// Get all statefulsets and find the Redis HA statefulset
	statefulsetList := &appsv1.StatefulSetList{}
	err = r.List(context.TODO(), statefulsetList, client.InNamespace(a.Namespace))
	assert.NoError(t, err)

	var redisStatefulset *appsv1.StatefulSet
	for i := range statefulsetList.Items {
		if statefulsetList.Items[i].Labels[common.ArgoCDKeyComponent] == "redis" {
			redisStatefulset = &statefulsetList.Items[i]
			break
		}
	}
	assert.NotNil(t, redisStatefulset, "Redis HA statefulset should exist")

	// Verify that the StatefulSet name is truncated and within limits
	assert.LessOrEqual(t, len(redisStatefulset.Name), 63)
	assert.Contains(t, redisStatefulset.Name, "redis")

	// Verify that the StatefulSet name contains the "server" suffix (unique identifier)
	assert.Contains(t, redisStatefulset.Name, "redis-ha-server", "StatefulSet name should contain the full suffix for uniqueness")

	// Verify that the component label is set correctly
	assert.Equal(t, "redis", redisStatefulset.Labels[common.ArgoCDKeyComponent])

	// Verify that the selector uses the component name (our fix)
	expectedComponentName := nameWithSuffix("redis-ha", a)
	assert.Equal(t, expectedComponentName, redisStatefulset.Spec.Selector.MatchLabels[common.ArgoCDKeyName])

	// Verify that the pod template labels use the component name (our fix)
	assert.Equal(t, expectedComponentName, redisStatefulset.Spec.Template.Labels[common.ArgoCDKeyName])

	// Verify that the service name uses the component name (our fix)
	assert.Equal(t, expectedComponentName, redisStatefulset.Spec.ServiceName)
}
