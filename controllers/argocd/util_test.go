package argocd

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/kubernetes/fake"
)

const (
	dexTestImage              = "testing/dex:latest"
	argoTestImage             = "testing/argocd:latest"
	argoTestImageOtherVersion = "testing/argocd:test"
	redisTestImage            = "testing/redis:latest"
	redisHATestImage          = "testing/redis:latest-ha"
	redisHAProxyTestImage     = "testing/redis-ha-haproxy:latest-ha"
)

func parallelismLimit(n int32) argoCDOpt {
	return func(a *argoproj.ArgoCD) {
		a.Spec.Controller.ParallelismLimit = n
	}
}

func logFormat(f string) argoCDOpt {
	return func(a *argoproj.ArgoCD) {
		a.Spec.Controller.LogFormat = f
	}
}

func logLevel(l string) argoCDOpt {
	return func(a *argoproj.ArgoCD) {
		a.Spec.Controller.LogLevel = l
	}
}

func extraCommandArgs(l []string) argoCDOpt {
	return func(a *argoproj.ArgoCD) {
		a.Spec.Controller.ExtraCommandArgs = l
	}
}

func appSync(s int) argoCDOpt {
	return func(a *argoproj.ArgoCD) {
		a.Spec.Controller.AppSync = &metav1.Duration{Duration: time.Second * time.Duration(s)}
	}
}

var imageTests = []struct {
	name      string
	pre       func(t *testing.T)
	opts      []argoCDOpt
	want      string
	imageFunc func(a *argoproj.ArgoCD) string
}{
	{
		name:      "dex default configuration",
		imageFunc: getDexContainerImage,
		want:      argoutil.CombineImageTag(common.ArgoCDDefaultDexImage, common.ArgoCDDefaultDexVersion),
	},
	{
		name:      "dex spec configuration",
		imageFunc: getDexContainerImage,
		want:      dexTestImage,
		opts: []argoCDOpt{func(a *argoproj.ArgoCD) {
			a.Spec.SSO = &argoproj.ArgoCDSSOSpec{
				Provider: argoproj.SSOProviderTypeDex,
				Dex: &argoproj.ArgoCDDexSpec{
					Image:   "testing/dex",
					Version: "latest",
				},
			}
		}},
	},
	{
		name:      "dex env configuration",
		imageFunc: getDexContainerImage,
		want:      dexTestImage,
		pre: func(t *testing.T) {
			t.Setenv(common.ArgoCDDexImageEnvName, dexTestImage)
		},
	},
	{
		name:      "argo default configuration",
		imageFunc: getArgoContainerImage,
		want:      argoutil.CombineImageTag(common.ArgoCDDefaultArgoImage, common.ArgoCDDefaultArgoVersion),
	},
	{
		name:      "argo spec configuration",
		imageFunc: getArgoContainerImage,
		want:      argoTestImage, opts: []argoCDOpt{func(a *argoproj.ArgoCD) {
			a.Spec.Image = "testing/argocd"
			a.Spec.Version = "latest"
		}},
	},
	{
		name:      "argo env configuration",
		imageFunc: getArgoContainerImage,
		want:      argoTestImage,
		pre: func(t *testing.T) {
			t.Setenv(common.ArgoCDImageEnvName, argoTestImage)
		},
	},
	{
		name:      "repo default configuration",
		imageFunc: getRepoServerContainerImage,
		want:      argoutil.CombineImageTag(common.ArgoCDDefaultArgoImage, common.ArgoCDDefaultArgoVersion),
	},
	{
		name:      "repo spec configuration",
		imageFunc: getRepoServerContainerImage,
		want:      argoTestImage, opts: []argoCDOpt{func(a *argoproj.ArgoCD) {
			a.Spec.Repo.Image = "testing/argocd"
			a.Spec.Repo.Version = "latest"
		}},
	},
	{
		name:      "repo configuration fallback spec",
		imageFunc: getRepoServerContainerImage,
		want:      argoTestImageOtherVersion, opts: []argoCDOpt{func(a *argoproj.ArgoCD) {
			a.Spec.Image = "testing/argocd"
			a.Spec.Version = "test"
		}},
	},
	{
		name:      "argo env configuration",
		imageFunc: getRepoServerContainerImage,
		want:      argoTestImage,
		pre: func(t *testing.T) {
			t.Setenv(common.ArgoCDImageEnvName, argoTestImage)
		},
	},
	{
		name:      "redis default configuration",
		imageFunc: getRedisContainerImage,
		want:      argoutil.CombineImageTag(common.ArgoCDDefaultRedisImage, common.ArgoCDDefaultRedisVersion),
	},
	{
		name:      "redis spec configuration",
		imageFunc: getRedisContainerImage,
		want:      redisTestImage,
		opts: []argoCDOpt{func(a *argoproj.ArgoCD) {
			a.Spec.Redis.Image = "testing/redis"
			a.Spec.Redis.Version = "latest"
		}},
	},
	{
		name:      "redis env configuration",
		imageFunc: getRedisContainerImage,
		want:      redisTestImage,
		pre: func(t *testing.T) {
			t.Setenv(common.ArgoCDRedisImageEnvName, redisTestImage)
		},
	},
	{
		name:      "redis ha default configuration",
		imageFunc: getRedisHAContainerImage,
		want: argoutil.CombineImageTag(
			common.ArgoCDDefaultRedisImage,
			common.ArgoCDDefaultRedisVersionHA),
	},
	{
		name:      "redis ha spec configuration",
		imageFunc: getRedisHAContainerImage,
		want:      redisHATestImage,
		opts: []argoCDOpt{func(a *argoproj.ArgoCD) {
			a.Spec.Redis.Image = "testing/redis"
			a.Spec.Redis.Version = "latest-ha"
		}},
	},
	{
		name:      "redis ha env configuration",
		imageFunc: getRedisHAContainerImage,
		want:      redisHATestImage,
		pre: func(t *testing.T) {
			t.Setenv(common.ArgoCDRedisHAImageEnvName, redisHATestImage)
		},
	},
	{
		name:      "redis ha proxy default configuration",
		imageFunc: getRedisHAProxyContainerImage,
		want: argoutil.CombineImageTag(
			common.ArgoCDDefaultRedisHAProxyImage,
			common.ArgoCDDefaultRedisHAProxyVersion),
	},
	{
		name:      "redis ha proxy spec configuration",
		imageFunc: getRedisHAProxyContainerImage,
		want:      redisHAProxyTestImage,
		opts: []argoCDOpt{func(a *argoproj.ArgoCD) {
			a.Spec.HA.RedisProxyImage = "testing/redis-ha-haproxy"
			a.Spec.HA.RedisProxyVersion = "latest-ha"
		}},
	},
	{
		name:      "redis ha proxy env configuration",
		imageFunc: getRedisHAProxyContainerImage,
		want:      redisHAProxyTestImage,
		pre: func(t *testing.T) {
			t.Setenv(common.ArgoCDRedisHAProxyImageEnvName, redisHAProxyTestImage)
		},
	},
}

func TestContainerImages_configuration(t *testing.T) {
	for _, tt := range imageTests {
		t.Run(tt.name, func(rt *testing.T) {
			if tt.pre != nil {
				tt.pre(rt)
			}
			a := makeTestArgoCD(tt.opts...)
			image := tt.imageFunc(a)
			if image != tt.want {
				rt.Errorf("got %q, want %q", image, tt.want)
			}
		})
	}
}

var argoServerURITests = []struct {
	name         string
	routeEnabled bool
	opts         []argoCDOpt
	want         string
}{
	{
		name:         "test with no host name - default",
		routeEnabled: false,
		want:         "https://argocd-server",
	},
	{
		name:         "test with external host name",
		routeEnabled: false,
		opts: []argoCDOpt{func(a *argoproj.ArgoCD) {
			a.Spec.Server.Host = "test-host-name"
		}},
		want: "https://test-host-name",
	},
}

func setRouteAPIFound(t *testing.T, routeEnabled bool) {
	routeAPIEnabledTemp := routeAPIFound
	t.Cleanup(func() {
		routeAPIFound = routeAPIEnabledTemp
	})
	routeAPIFound = routeEnabled
}

func TestGetArgoServerURI(t *testing.T) {
	for _, tt := range argoServerURITests {
		t.Run(tt.name, func(t *testing.T) {
			cr := makeTestArgoCD(tt.opts...)
			r := &ReconcileArgoCD{}
			setRouteAPIFound(t, tt.routeEnabled)
			result := r.getArgoServerURI(cr)
			if result != tt.want {
				t.Errorf("%s test failed, got=%q want=%q", tt.name, result, tt.want)
			}
		})
	}
}

func TestRemoveDeletionFinalizer(t *testing.T) {
	t.Run("ArgoCD resource present", func(t *testing.T) {
		a := makeTestArgoCD(addFinalizer(common.ArgoCDDeletionFinalizer))

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch)

		err := r.removeDeletionFinalizer(a)
		assert.NoError(t, err)
		if a.IsDeletionFinalizerPresent() {
			t.Fatal("Expected deletion finalizer to be removed")
		}
	})
	t.Run("ArgoCD resource absent", func(t *testing.T) {
		a := makeTestArgoCD(addFinalizer(common.ArgoCDDeletionFinalizer))

		resObjs := []client.Object{}
		subresObjs := []client.Object{}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch)

		err := r.removeDeletionFinalizer(a)
		assert.Error(t, err, `failed to remove deletion finalizer from argocd: argocds.argoproj.io "argocd" not found`)
	})
}

func TestAddDeletionFinalizer(t *testing.T) {
	t.Run("ArgoCD resource present", func(t *testing.T) {
		a := makeTestArgoCD()

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch)

		err := r.addDeletionFinalizer(a)
		assert.NoError(t, err)
		if !a.IsDeletionFinalizerPresent() {
			t.Fatal("Expected deletion finalizer to be added")
		}
	})
	t.Run("ArgoCD resource absent", func(t *testing.T) {
		a := makeTestArgoCD()

		resObjs := []client.Object{}
		subresObjs := []client.Object{}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch)

		err := r.addDeletionFinalizer(a)
		assert.Error(t, err, `failed to add deletion finalizer for argocd: argocds.argoproj.io "argocd" not found`)
	})
}

func TestArgoCDInstanceSelector(t *testing.T) {
	t.Run("Selector for a Valid name", func(t *testing.T) {
		validName := "argocd-server"
		selector, err := argocdInstanceSelector(validName)
		assert.NoError(t, err)
		assert.Equal(t, selector.String(), "app.kubernetes.io/managed-by=argocd-server")
	})
	t.Run("Selector for an Invalid name", func(t *testing.T) {
		invalidName := "argocd-*/"
		selector, err := argocdInstanceSelector(invalidName)
		//assert.ErrorContains(t, err, `failed to create a requirement for values[0][app.kubernetes.io/managed-by]: Invalid value: "argocd-*/`)
		//TODO: https://github.com/stretchr/testify/pull/1022 introduced ErrorContains, but is not yet available in a tagged release. Revert to ErrorContains once this becomes available
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `failed to create a requirement for values[0][app.kubernetes.io/managed-by]: Invalid value: "argocd-*/`)

		assert.Equal(t, selector, nil)
	})
}

func TestGetArgoApplicationControllerCommand(t *testing.T) {

	defaultResult := []string{
		"argocd-application-controller",
		"--operation-processors",
		"10",
		"--redis",
		"argocd-redis.argocd.svc.cluster.local:6379",
		"--repo-server",
		"argocd-repo-server.argocd.svc.cluster.local:8081",
		"--status-processors",
		"20",
		"--kubectl-parallelism-limit",
		"10",
		"--loglevel",
		"info",
		"--logformat",
		"text",
	}

	controllerProcesorsChangedResult := func(n string) []string {
		return []string{
			"argocd-application-controller",
			"--operation-processors",
			"10",
			"--redis",
			"argocd-redis.argocd.svc.cluster.local:6379",
			"--repo-server",
			"argocd-repo-server.argocd.svc.cluster.local:8081",
			"--status-processors",
			n,
			"--kubectl-parallelism-limit",
			"10",
			"--loglevel",
			"info",
			"--logformat",
			"text",
		}
	}

	operationProcesorsChangedResult := func(n string) []string {
		return []string{
			"argocd-application-controller",
			"--operation-processors",
			n,
			"--redis",
			"argocd-redis.argocd.svc.cluster.local:6379",
			"--repo-server",
			"argocd-repo-server.argocd.svc.cluster.local:8081",
			"--status-processors",
			"20",
			"--kubectl-parallelism-limit",
			"10",
			"--loglevel",
			"info",
			"--logformat",
			"text",
		}
	}

	parallelismLimitChangedResult := func(n string) []string {
		return []string{
			"argocd-application-controller",
			"--operation-processors",
			"10",
			"--redis",
			"argocd-redis.argocd.svc.cluster.local:6379",
			"--repo-server",
			"argocd-repo-server.argocd.svc.cluster.local:8081",
			"--status-processors",
			"20",
			"--kubectl-parallelism-limit",
			n,
			"--loglevel",
			"info",
			"--logformat",
			"text",
		}
	}

	logFormatChangedResult := func(f string) []string {
		return []string{
			"argocd-application-controller",
			"--operation-processors",
			"10",
			"--redis",
			"argocd-redis.argocd.svc.cluster.local:6379",
			"--repo-server",
			"argocd-repo-server.argocd.svc.cluster.local:8081",
			"--status-processors",
			"20",
			"--kubectl-parallelism-limit",
			"10",
			"--loglevel",
			"info",
			"--logformat",
			f,
		}
	}

	logLevelChangedResult := func(l string) []string {
		return []string{
			"argocd-application-controller",
			"--operation-processors",
			"10",
			"--redis",
			"argocd-redis.argocd.svc.cluster.local:6379",
			"--repo-server",
			"argocd-repo-server.argocd.svc.cluster.local:8081",
			"--status-processors",
			"20",
			"--kubectl-parallelism-limit",
			"10",
			"--loglevel",
			l,
			"--logformat",
			"text",
		}
	}

	extraCommandArgsChangedResult := func(l []string) []string {
		return append(defaultResult, l...)
	}

	cmdTests := []struct {
		name string
		opts []argoCDOpt
		want []string
	}{
		{
			"defaults",
			[]argoCDOpt{},
			defaultResult,
		},
		{
			"configured status processors",
			[]argoCDOpt{controllerProcessors(30)},
			controllerProcesorsChangedResult("30"),
		},
		{
			"configured status processors to zero",
			[]argoCDOpt{controllerProcessors(0)},
			defaultResult,
		},
		{
			"configured status processors to be between zero and default",
			[]argoCDOpt{controllerProcessors(10)},
			controllerProcesorsChangedResult("10"),
		},
		{
			"configured operation processors",
			[]argoCDOpt{operationProcessors(15)},
			operationProcesorsChangedResult("15"),
		},
		{
			"configured operation processors to zero",
			[]argoCDOpt{operationProcessors(0)},
			defaultResult,
		},
		{
			"configured operation processors to be between zero and ten",
			[]argoCDOpt{operationProcessors(5)},
			operationProcesorsChangedResult("5"),
		},
		{
			"configured parallelism limit",
			[]argoCDOpt{parallelismLimit(30)},
			parallelismLimitChangedResult("30"),
		},
		{
			"configured parallelism limit to zero",
			[]argoCDOpt{parallelismLimit(0)},
			defaultResult,
		},
		{
			"configured invalid logformat",
			[]argoCDOpt{logFormat("arbitrary")},
			defaultResult,
		},
		{
			"configured json logformat",
			[]argoCDOpt{logFormat("json")},
			logFormatChangedResult("json"),
		},
		{
			"configured text logformat",
			[]argoCDOpt{logFormat("text")},
			logFormatChangedResult("text"),
		},
		{
			"configured invalid loglevel",
			[]argoCDOpt{logLevel("arbitrary")},
			defaultResult,
		},
		{
			"configured debug loglevel",
			[]argoCDOpt{logLevel("debug")},
			logLevelChangedResult("debug"),
		},
		{
			"configured info loglevel",
			[]argoCDOpt{logLevel("info")},
			logLevelChangedResult("info"),
		},
		{
			"configured warn loglevel",
			[]argoCDOpt{logLevel("warn")},
			logLevelChangedResult("warn"),
		},
		{
			"configured error loglevel",
			[]argoCDOpt{logLevel("error")},
			logLevelChangedResult("error"),
		},
		{
			"configured extraCommandArgs",
			[]argoCDOpt{extraCommandArgs([]string{"--app-hard-resync", "--app-resync"})},
			extraCommandArgsChangedResult([]string{"--app-hard-resync", "--app-resync"}),
		},
		{
			"overriding default argument using extraCommandArgs",
			[]argoCDOpt{extraCommandArgs([]string{"--operation-processors", "15"})},
			defaultResult,
		},
		{
			"configured empty extraCommandArgs",
			[]argoCDOpt{extraCommandArgs([]string{})},
			defaultResult,
		},
	}

	for _, tt := range cmdTests {
		cr := makeTestArgoCD(tt.opts...)
		cmd := getArgoApplicationControllerCommand(cr, false)

		if !reflect.DeepEqual(cmd, tt.want) {
			t.Fatalf("got %#v, want %#v", cmd, tt.want)
		}
	}
}

func TestGetArgoApplicationContainerEnv(t *testing.T) {

	sync60s := []v1.EnvVar{
		{Name: "HOME", Value: "/home/argocd", ValueFrom: (*v1.EnvVarSource)(nil)},
		{Name: "REDIS_PASSWORD", Value: "",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("argocd-redis-initial-password"),
					},
					Key: "admin.password",
				},
			}},
		{Name: "ARGOCD_RECONCILIATION_TIMEOUT", Value: "60s", ValueFrom: (*v1.EnvVarSource)(nil)}}

	cmdTests := []struct {
		name string
		opts []argoCDOpt
		want []v1.EnvVar
	}{
		{
			"configured apsync to 60s",
			[]argoCDOpt{appSync(60)},
			sync60s,
		},
	}

	for _, tt := range cmdTests {
		cr := makeTestArgoCD(tt.opts...)
		env := getArgoControllerContainerEnv(cr, 1)

		if !reflect.DeepEqual(env, tt.want) {
			t.Fatalf("got %#v, want %#v", env, tt.want)
		}
	}
}

func TestDeleteRBACsForNamespace(t *testing.T) {
	a := makeTestArgoCD()
	testClient := testclient.NewSimpleClientset()
	testNameSpace := "testNameSpace"

	role := newRole("xyz", policyRuleForApplicationController(), a)
	role.Namespace = testNameSpace

	// create role with label
	_, err := testClient.RbacV1().Roles(testNameSpace).Create(context.TODO(), role, metav1.CreateOptions{})
	assert.NoError(t, err)

	role2 := newRole("abc", policyRuleForApplicationController(), a)
	role2.Namespace = testNameSpace
	role2.Labels = map[string]string{}

	// create role without label
	_, err = testClient.RbacV1().Roles(testNameSpace).Create(context.TODO(), role2, metav1.CreateOptions{})
	assert.NoError(t, err)

	roleBinding := newRoleBindingWithname("xyz", a)
	roleBinding.Namespace = testNameSpace

	// create roleBinding with label
	_, err = testClient.RbacV1().RoleBindings(testNameSpace).Create(context.TODO(), roleBinding, metav1.CreateOptions{})
	assert.NoError(t, err)

	roleBinding2 := newRoleBindingWithname("abc", a)
	roleBinding2.Namespace = testNameSpace
	roleBinding2.Labels = map[string]string{}

	// create RoleBinding without label
	_, err = testClient.RbacV1().RoleBindings(testNameSpace).Create(context.TODO(), roleBinding2, metav1.CreateOptions{})
	assert.NoError(t, err)

	// run deleteRBACsForNamespace
	assert.NoError(t, deleteRBACsForNamespace(testNameSpace, testClient))

	// role with the label should be deleted
	_, err = testClient.RbacV1().Roles(testNameSpace).Get(context.TODO(), role.Name, metav1.GetOptions{})
	//assert.ErrorContains(t, err, "not found")
	//TODO: https://github.com/stretchr/testify/pull/1022 introduced ErrorContains, but is not yet available in a tagged release. Revert to ErrorContains once this becomes available
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// role without the label should still exists, no error
	_, err = testClient.RbacV1().Roles(testNameSpace).Get(context.TODO(), role2.Name, metav1.GetOptions{})
	assert.NoError(t, err)

	// roleBinding with the label should be deleted
	_, err = testClient.RbacV1().Roles(testNameSpace).Get(context.TODO(), roleBinding.Name, metav1.GetOptions{})
	//assert.ErrorContains(t, err, "not found")
	//TODO: https://github.com/stretchr/testify/pull/1022 introduced ErrorContains, but is not yet available in a tagged release. Revert to ErrorContains once this becomes available
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// roleBinding without the label should still exists, no error
	_, err = testClient.RbacV1().Roles(testNameSpace).Get(context.TODO(), roleBinding2.Name, metav1.GetOptions{})
	assert.NoError(t, err)

}

func TestRemoveManagedNamespaceFromClusterSecretAfterDeletion(t *testing.T) {
	a := makeTestArgoCD()
	testClient := testclient.NewSimpleClientset()
	testNameSpace := "testNameSpace"

	secret := argoutil.NewSecretWithSuffix(a, "xyz")
	secret.Labels = map[string]string{common.ArgoCDSecretTypeLabel: "cluster"}
	secret.Data = map[string][]byte{
		"server":     []byte(common.ArgoCDDefaultServer),
		"namespaces": []byte(strings.Join([]string{testNameSpace, "testNamespace2"}, ",")),
	}

	// create secret with the label
	_, err := testClient.CoreV1().Secrets(a.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	assert.NoError(t, err)

	// run deleteManagedNamespaceFromClusterSecret
	assert.NoError(t, deleteManagedNamespaceFromClusterSecret(a.Namespace, testNameSpace, testClient))

	// secret should still exists with updated list of namespaces
	s, err := testClient.CoreV1().Secrets(a.Namespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, string(s.Data["namespaces"]), "testNamespace2")

}

func TestRemoveManagedByLabelFromNamespaces(t *testing.T) {
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	nsArgocd := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: a.Namespace,
	}}
	err := r.Client.Create(context.TODO(), nsArgocd)
	assert.NoError(t, err)

	ns := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "testNamespace",
		Labels: map[string]string{
			common.ArgoCDManagedByLabel: a.Namespace,
		}},
	}

	err = r.Client.Create(context.TODO(), ns)
	assert.NoError(t, err)

	ns2 := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "testNamespace2",
		Labels: map[string]string{
			common.ArgoCDManagedByLabel: a.Namespace,
		}},
	}

	err = r.Client.Create(context.TODO(), ns2)
	assert.NoError(t, err)

	ns3 := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "testNamespace3",
		Labels: map[string]string{
			common.ArgoCDManagedByLabel: "newNamespace",
		}},
	}

	err = r.Client.Create(context.TODO(), ns3)
	assert.NoError(t, err)

	err = r.removeManagedByLabelFromNamespaces(a.Namespace)
	assert.NoError(t, err)

	nsList := &v1.NamespaceList{}
	err = r.Client.List(context.TODO(), nsList)
	assert.NoError(t, err)
	for _, n := range nsList.Items {
		if n.Name == ns3.Name {
			_, ok := n.Labels[common.ArgoCDManagedByLabel]
			assert.Equal(t, ok, true)
			continue
		}
		_, ok := n.Labels[common.ArgoCDManagedByLabel]
		assert.Equal(t, ok, false)
	}
}

func TestSetManagedNamespaces(t *testing.T) {
	a := makeTestArgoCD()

	ns1 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-1",
			Labels: map[string]string{
				common.ArgoCDManagedByLabel: testNamespace,
			},
		},
	}

	ns2 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-2",
			Labels: map[string]string{
				common.ArgoCDManagedByLabel: testNamespace,
			},
		},
	}

	ns3 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-3",
			Labels: map[string]string{
				common.ArgoCDManagedByLabel: "random-namespace",
			},
		},
	}

	ns4 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-4",
		},
	}

	resObjs := []client.Object{a, &ns1, &ns2, &ns3, &ns4}

	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	err := r.setManagedNamespaces(a)
	assert.NoError(t, err)

	assert.Equal(t, len(r.ManagedNamespaces.Items), 3)
	for _, n := range r.ManagedNamespaces.Items {
		if n.Labels[common.ArgoCDManagedByLabel] != testNamespace && n.Name != testNamespace {
			t.Errorf("Expected namespace %s to be managed by Argo CD instance %s", n.Name, testNamespace)
		}
	}
}

func TestSetManagedSourceNamespaces(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec = argoproj.ArgoCDSpec{
		SourceNamespaces: []string{
			"test-namespace-1",
		},
	}
	ns1 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-1",
			Labels: map[string]string{
				common.ArgoCDManagedByClusterArgoCDLabel: testNamespace,
			},
		},
	}

	resObjs := []client.Object{a, &ns1}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	err := r.setManagedSourceNamespaces(a)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(r.ManagedSourceNamespaces))
	assert.Contains(t, r.ManagedSourceNamespaces, "test-namespace-1")
}

func TestGetSourceNamespacesWithWildcardPatternNamespace(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec = argoproj.ArgoCDSpec{
		SourceNamespaces: []string{
			"test*",
		},
	}
	ns1 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-1",
		},
	}

	ns2 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-2",
		},
	}
	ns3 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "other-namespace",
		},
	}

	resObjs := []client.Object{a, &ns1, &ns2, &ns3}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	sourceNamespaces, err := r.getSourceNamespaces(a)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(sourceNamespaces))
	assert.Contains(t, sourceNamespaces, "test-namespace-1")
	assert.Contains(t, sourceNamespaces, "test-namespace-2")
	assert.NotContains(t, sourceNamespaces, "other-namespace")
}

func TestGetSourceNamespacesWithSpecificNamespace(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec = argoproj.ArgoCDSpec{
		SourceNamespaces: []string{
			"test",
		},
	}
	ns1 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
	ns2 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-1",
		},
	}
	ns3 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "other-namespace",
		},
	}

	resObjs := []client.Object{a, &ns1, &ns2, &ns3}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	sourceNamespaces, err := r.getSourceNamespaces(a)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(sourceNamespaces))
	assert.Contains(t, sourceNamespaces, "test")
	assert.NotContains(t, sourceNamespaces, "test-namespace-1")
	assert.NotContains(t, sourceNamespaces, "other-namespace")
}

func TestGetSourceNamespacesWithMultipleSourceNamespaces(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec = argoproj.ArgoCDSpec{
		SourceNamespaces: []string{
			"test*",
			"dev*",
		},
	}
	ns1 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
	ns2 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-1",
		},
	}
	ns3 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dev-namespace-1",
		},
	}
	ns4 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "other-namespace",
		},
	}

	resObjs := []client.Object{a, &ns1, &ns2, &ns3, &ns4}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	sourceNamespaces, err := r.getSourceNamespaces(a)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(sourceNamespaces))
	assert.Contains(t, sourceNamespaces, "test")
	assert.Contains(t, sourceNamespaces, "test-namespace-1")
	assert.Contains(t, sourceNamespaces, "dev-namespace-1")
	assert.NotContains(t, sourceNamespaces, "other-namespace")
}

func TestGetSourceNamespacesWithWildCardNamespace(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec = argoproj.ArgoCDSpec{
		SourceNamespaces: []string{
			"*",
		},
	}
	ns1 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-1",
		},
	}
	ns2 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-2",
		},
	}
	ns3 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "other-namespace",
		},
	}

	resObjs := []client.Object{a, &ns1, &ns2, &ns3}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	sourceNamespaces, err := r.getSourceNamespaces(a)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(sourceNamespaces))
	assert.Contains(t, sourceNamespaces, "other-namespace")
	assert.Contains(t, sourceNamespaces, "test-namespace-1")
	assert.Contains(t, sourceNamespaces, "test-namespace-2")
}

func TestGenerateRandomString(t *testing.T) {

	// verify the creation of unique strings
	s1 := generateRandomString(20)
	s2 := generateRandomString(20)
	assert.NotEqual(t, s1, s2)

	// verify length
	a, _ := b64.URLEncoding.DecodeString(s1)
	assert.Len(t, a, 20)

	b, _ := b64.URLEncoding.DecodeString(s2)
	assert.Len(t, b, 20)
}

func generateEncodedPEM(t *testing.T) []byte {
	key, err := argoutil.NewPrivateKey()
	assert.NoError(t, err)

	cert, err := argoutil.NewSelfSignedCACertificate("foo", key)
	assert.NoError(t, err)

	encoded := argoutil.EncodeCertificatePEM(cert)
	return encoded
}

// TestReconcileArgoCD_reconcileDexOAuthClientSecret This test make sures that if dex is enabled a service account is created with token stored in a secret which is used for oauth
func TestReconcileArgoCD_reconcileDexOAuthClientSecret(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(ac *argoproj.ArgoCD) {
		ac.Spec.SSO = &argoproj.ArgoCDSSOSpec{
			Provider: argoproj.SSOProviderTypeDex,
			Dex: &argoproj.ArgoCDDexSpec{
				OpenShiftOAuth: true,
			},
		}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, createNamespace(r, a.Namespace, ""))
	_, err := r.reconcileServiceAccount(common.ArgoCDDefaultDexServiceAccountName, a)
	assert.NoError(t, err)
	_, err = r.getDexOAuthClientSecret(a)
	assert.NoError(t, err)
	sa := newServiceAccountWithName(common.ArgoCDDefaultDexServiceAccountName, a)
	assert.NoError(t, argoutil.FetchObject(r.Client, a.Namespace, sa.Name, sa))
	tokenExists := false
	for _, saSecret := range sa.Secrets {
		if strings.Contains(saSecret.Name, "dex-server-token") {
			tokenExists = true
		}
	}
	assert.True(t, tokenExists, "Dex is enabled but unable to create oauth client secret")
}
