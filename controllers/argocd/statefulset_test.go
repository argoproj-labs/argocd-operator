package argocd

import (
	"context"
	"fmt"
	"testing"
	"time"

	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

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
	r := makeTestReconciler(cl, sch)

	s := newStatefulSetWithSuffix("redis-ha-server", "redis", a)

	assert.NoError(t, r.reconcileRedisStatefulSet(a))
	// resource Creation should fail as HA was disabled
	assert.Errorf(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s), "not found")
}

func TestReconcileArgoCD_reconcileRedisStatefulSet_HA_enabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	s := newStatefulSetWithSuffix("redis-ha-server", "redis", a)

	a.Spec.HA.Enabled = true
	// test resource is Created when HA is enabled
	assert.NoError(t, r.reconcileRedisStatefulSet(a))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s))

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
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s))
	for _, container := range s.Spec.Template.Spec.Containers {
		assert.Equal(t, container.Image, fmt.Sprintf("%s:%s", testRedisImage, testRedisImageVersion))
		assert.Equal(t, container.Resources, newResources)
	}
	assert.Equal(t, s.Spec.Template.Spec.InitContainers[0].Resources, newResources)

	// test resource is Deleted, when HA is disabled
	a.Spec.HA.Enabled = false
	assert.NoError(t, r.reconcileRedisStatefulSet(a))
	assert.Errorf(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s), "not found")
}

func TestReconcileArgoCD_reconcileApplicationController(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))

	ss := &appsv1.StatefulSet{}
	assert.NoError(t, r.Client.Get(
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
		"--logformat", "text"}
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
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, true))

	ss := &appsv1.StatefulSet{}
	assert.NoError(t, r.Client.Get(
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
		"--logformat", "text"}
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
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))

	a = makeTestArgoCD(controllerProcessors(30))
	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))

	ss := &appsv1.StatefulSet{}
	assert.NoError(t, r.Client.Get(
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
		"--logformat", "text"}
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
	r := makeTestReconciler(cl, sch)

	deploy := newDeploymentWithSuffix("application-controller", "application-controller", a)
	assert.NoError(t, r.Client.Create(context.TODO(), deploy))

	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, deploy)
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
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))

	ss := &appsv1.StatefulSet{}
	assert.NoError(t, r.Client.Get(
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
				{Name: "HOME", Value: "/home/argocd"},
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
				{Name: "HOME", Value: "/home/argocd"},
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
				{Name: "HOME", Value: "/home/argocd"},
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
		r := makeTestReconciler(cl, sch)

		assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))

		ss := &appsv1.StatefulSet{}
		assert.NoError(t, r.Client.Get(
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
		{Name: "ARGOCD_RECONCILIATION_TIMEOUT", Value: "600s"},
		{Name: "HOME", Value: "/home/argocd"},
	}

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Controller.AppSync = &metav1.Duration{Duration: time.Minute * 10}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, r.reconcileApplicationControllerStatefulSet(a, false))

	ss := &appsv1.StatefulSet{}
	assert.NoError(t, r.Client.Get(
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
	updateNodePlacementStateful(ss, ss, &actualChange)
	if actualChange != expectedChange {
		t.Fatalf("updateNodePlacement failed, value of changed: %t", actualChange)
	}
	updateNodePlacementStateful(ss, ss2, &actualChange)
	if actualChange == expectedChange {
		t.Fatalf("updateNodePlacement failed, value of changed: %t", actualChange)
	}
}

func Test_ContainsValidImage(t *testing.T) {

	a := makeTestArgoCD()
	po := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
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
	r := makeTestReconciler(cl, sch)

	if containsInvalidImage(a, r) {
		t.Fatalf("containsInvalidImage failed, got true, expected false")
	}

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
		r := makeTestReconciler(cl, sch)

		assert.NoError(t, r.Client.Create(context.TODO(), clusterSecret1))
		assert.NoError(t, r.Client.Create(context.TODO(), clusterSecret2))
		assert.NoError(t, r.Client.Create(context.TODO(), clusterSecret3))

		replicas := r.getApplicationControllerReplicaCount(a)

		assert.Equal(t, int32(st.expectedReplicas), replicas)

	}
}
