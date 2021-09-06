package argocd

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"gotest.tools/assert"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
)

const (
	dexTestImage          = "testing/dex:latest"
	argoTestImage         = "testing/argocd:latest"
	grafanaTestImage      = "testing/grafana:latest"
	redisTestImage        = "testing/redis:latest"
	redisHATestImage      = "testing/redis:latest-ha"
	redisHAProxyTestImage = "testing/redis-ha-haproxy:latest-ha"
)

var imageTests = []struct {
	name      string
	pre       func(t *testing.T)
	opts      []argoCDOpt
	want      string
	imageFunc func(a *argoprojv1alpha1.ArgoCD) string
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
		opts: []argoCDOpt{func(a *argoprojv1alpha1.ArgoCD) {
			a.Spec.Dex.Image = "testing/dex"
			a.Spec.Dex.Version = "latest"
		}},
	},
	{
		name:      "dex env configuration",
		imageFunc: getDexContainerImage,
		want:      dexTestImage,
		pre: func(t *testing.T) {
			old := os.Getenv(common.ArgoCDDexImageEnvName)
			t.Cleanup(func() {
				os.Setenv(common.ArgoCDDexImageEnvName, old)
			})
			os.Setenv(common.ArgoCDDexImageEnvName, dexTestImage)
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
		want:      argoTestImage, opts: []argoCDOpt{func(a *argoprojv1alpha1.ArgoCD) {
			a.Spec.Image = "testing/argocd"
			a.Spec.Version = "latest"
		}},
	},
	{
		name:      "argo env configuration",
		imageFunc: getArgoContainerImage,
		want:      argoTestImage,
		pre: func(t *testing.T) {
			old := os.Getenv(common.ArgoCDImageEnvName)
			t.Cleanup(func() {
				os.Setenv(common.ArgoCDImageEnvName, old)
			})
			os.Setenv(common.ArgoCDImageEnvName, argoTestImage)
		},
	},
	{
		name:      "grafana default configuration",
		imageFunc: getGrafanaContainerImage,
		want:      argoutil.CombineImageTag(common.ArgoCDDefaultGrafanaImage, common.ArgoCDDefaultGrafanaVersion),
	},
	{
		name:      "grafana spec configuration",
		imageFunc: getGrafanaContainerImage,
		want:      grafanaTestImage,
		opts: []argoCDOpt{func(a *argoprojv1alpha1.ArgoCD) {
			a.Spec.Grafana.Image = "testing/grafana"
			a.Spec.Grafana.Version = "latest"
		}},
	},
	{
		name:      "grafana env configuration",
		imageFunc: getGrafanaContainerImage,
		want:      grafanaTestImage,
		pre: func(t *testing.T) {
			old := os.Getenv(common.ArgoCDGrafanaImageEnvName)
			t.Cleanup(func() {
				os.Setenv(common.ArgoCDGrafanaImageEnvName, old)
			})
			os.Setenv(common.ArgoCDGrafanaImageEnvName, grafanaTestImage)
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
		opts: []argoCDOpt{func(a *argoprojv1alpha1.ArgoCD) {
			a.Spec.Redis.Image = "testing/redis"
			a.Spec.Redis.Version = "latest"
		}},
	},
	{
		name:      "redis env configuration",
		imageFunc: getRedisContainerImage,
		want:      redisTestImage,
		pre: func(t *testing.T) {
			old := os.Getenv(common.ArgoCDRedisImageEnvName)
			t.Cleanup(func() {
				os.Setenv(common.ArgoCDRedisImageEnvName, old)
			})
			os.Setenv(common.ArgoCDRedisImageEnvName, redisTestImage)
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
		opts: []argoCDOpt{func(a *argoprojv1alpha1.ArgoCD) {
			a.Spec.Redis.Image = "testing/redis"
			a.Spec.Redis.Version = "latest-ha"
		}},
	},
	{
		name:      "redis ha env configuration",
		imageFunc: getRedisHAContainerImage,
		want:      redisHATestImage,
		pre: func(t *testing.T) {
			old := os.Getenv(common.ArgoCDRedisHAImageEnvName)
			t.Cleanup(func() {
				os.Setenv(common.ArgoCDRedisHAImageEnvName, old)
			})
			os.Setenv(common.ArgoCDRedisHAImageEnvName, redisHATestImage)
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
		opts: []argoCDOpt{func(a *argoprojv1alpha1.ArgoCD) {
			a.Spec.HA.RedisProxyImage = "testing/redis-ha-haproxy"
			a.Spec.HA.RedisProxyVersion = "latest-ha"
		}},
	},
	{
		name:      "redis ha proxy env configuration",
		imageFunc: getRedisHAProxyContainerImage,
		want:      redisHAProxyTestImage,
		pre: func(t *testing.T) {
			old := os.Getenv(common.ArgoCDRedisHAProxyImageEnvName)
			t.Cleanup(func() {
				os.Setenv(common.ArgoCDRedisHAProxyImageEnvName, old)
			})
			os.Setenv(common.ArgoCDRedisHAProxyImageEnvName, redisHAProxyTestImage)
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
		opts: []argoCDOpt{func(a *argoprojv1alpha1.ArgoCD) {
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
		r := makeTestReconciler(t, a)
		err := r.removeDeletionFinalizer(a)
		assert.NilError(t, err)
		if a.IsDeletionFinalizerPresent() {
			t.Fatal("Expected deletion finalizer to be removed")
		}
	})
	t.Run("ArgoCD resource absent", func(t *testing.T) {
		a := makeTestArgoCD(addFinalizer(common.ArgoCDDeletionFinalizer))
		r := makeTestReconciler(t)
		err := r.removeDeletionFinalizer(a)
		assert.Error(t, err, `failed to remove deletion finalizer from argocd: argocds.argoproj.io "argocd" not found`)
	})
}

func TestAddDeletionFinalizer(t *testing.T) {
	t.Run("ArgoCD resource present", func(t *testing.T) {
		a := makeTestArgoCD()
		r := makeTestReconciler(t, a)
		err := r.addDeletionFinalizer(a)
		assert.NilError(t, err)
		if !a.IsDeletionFinalizerPresent() {
			t.Fatal("Expected deletion finalizer to be added")
		}
	})
	t.Run("ArgoCD resource absent", func(t *testing.T) {
		a := makeTestArgoCD()
		r := makeTestReconciler(t)
		err := r.addDeletionFinalizer(a)
		assert.Error(t, err, `failed to add deletion finalizer for argocd: argocds.argoproj.io "argocd" not found`)
	})
}

func TestArgoCDInstanceSelector(t *testing.T) {
	t.Run("Selector for a Valid name", func(t *testing.T) {
		validName := "argocd-server"
		selector, err := argocdInstanceSelector(validName)
		assert.NilError(t, err)
		assert.Equal(t, selector.String(), "app.kubernetes.io/managed-by=argocd-server")
	})
	t.Run("Selector for an Invalid name", func(t *testing.T) {
		invalidName := "argocd-*/"
		selector, err := argocdInstanceSelector(invalidName)
		assert.ErrorContains(t, err, `failed to create a requirement for values[0][app.kubernetes.io/managed-by]: Invalid value: "argocd-*/`)
		assert.Equal(t, selector, nil)
	})
}

func TestGetArgoApplicationControllerCommand(t *testing.T) {
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
				"argocd-redis.argocd.svc.cluster.local:6379",
				"--repo-server",
				"argocd-repo-server.argocd.svc.cluster.local:8081",
				"--status-processors",
				"20",
				"--loglevel",
				"info",
				"--logformat",
				"text",
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
				"argocd-redis.argocd.svc.cluster.local:6379",
				"--repo-server",
				"argocd-repo-server.argocd.svc.cluster.local:8081",
				"--status-processors",
				"30",
				"--loglevel",
				"info",
				"--logformat",
				"text",
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
				"argocd-redis.argocd.svc.cluster.local:6379",
				"--repo-server",
				"argocd-repo-server.argocd.svc.cluster.local:8081",
				"--status-processors",
				"20",
				"--loglevel",
				"info",
				"--logformat",
				"text",
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
				"argocd-redis.argocd.svc.cluster.local:6379",
				"--repo-server",
				"argocd-repo-server.argocd.svc.cluster.local:8081",
				"--status-processors",
				"20",
				"--app-resync",
				"600",
				"--loglevel",
				"info",
				"--logformat",
				"text",
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

func TestDeleteRBACsForNamespace(t *testing.T) {
	a := makeTestArgoCD()
	testClient := testclient.NewSimpleClientset()
	testNameSpace := "testNameSpace"

	role := newRole("xyz", policyRuleForApplicationController(), a)
	role.Namespace = testNameSpace

	// create role with label
	_, err := testClient.RbacV1().Roles(testNameSpace).Create(context.TODO(), role, metav1.CreateOptions{})
	assert.NilError(t, err)

	role2 := newRole("abc", policyRuleForApplicationController(), a)
	role2.Namespace = testNameSpace
	role2.Labels = map[string]string{}

	// create role without label
	_, err = testClient.RbacV1().Roles(testNameSpace).Create(context.TODO(), role2, metav1.CreateOptions{})
	assert.NilError(t, err)

	roleBinding := newRoleBindingWithname("xyz", a)
	roleBinding.Namespace = testNameSpace

	// create roleBinding with label
	_, err = testClient.RbacV1().RoleBindings(testNameSpace).Create(context.TODO(), roleBinding, metav1.CreateOptions{})
	assert.NilError(t, err)

	roleBinding2 := newRoleBindingWithname("abc", a)
	roleBinding2.Namespace = testNameSpace
	roleBinding2.Labels = map[string]string{}

	// create RoleBinding without label
	_, err = testClient.RbacV1().RoleBindings(testNameSpace).Create(context.TODO(), roleBinding2, metav1.CreateOptions{})
	assert.NilError(t, err)

	secret := argoutil.NewSecretWithSuffix(a, "xyz")
	secret.Labels = map[string]string{common.ArgoCDSecretTypeLabel: "cluster"}
	secret.Data = map[string][]byte{
		"server":     []byte(common.ArgoCDDefaultServer),
		"namespaces": []byte(strings.Join([]string{testNameSpace, "testNamespace2"}, ",")),
	}

	// create secret with the label
	_, err = testClient.CoreV1().Secrets(a.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	assert.NilError(t, err)

	// run deleteRBACsForNamespace
	assert.NilError(t, deleteRBACsForNamespace(a.Namespace, testNameSpace, testClient))

	// role with the label should be deleted
	_, err = testClient.RbacV1().Roles(testNameSpace).Get(context.TODO(), role.Name, metav1.GetOptions{})
	assert.ErrorContains(t, err, "not found")
	// role without the label should still exists, no error
	_, err = testClient.RbacV1().Roles(testNameSpace).Get(context.TODO(), role2.Name, metav1.GetOptions{})
	assert.NilError(t, err)

	// roleBinding with the label should be deleted
	_, err = testClient.RbacV1().Roles(testNameSpace).Get(context.TODO(), roleBinding.Name, metav1.GetOptions{})
	assert.ErrorContains(t, err, "not found")
	// roleBinding without the label should still exists, no error
	_, err = testClient.RbacV1().Roles(testNameSpace).Get(context.TODO(), roleBinding2.Name, metav1.GetOptions{})
	assert.NilError(t, err)

	// secret should still exists with updated list of namespaces
	s, err := testClient.CoreV1().Secrets(a.Namespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.DeepEqual(t, string(s.Data["namespaces"]), "testNamespace2")
}

func TestRemoveManagedByLabelFromNamespaces(t *testing.T) {
	a := makeTestArgoCD()
	r := makeTestReconciler(t)

	ns := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "testNamespace",
		Labels: map[string]string{
			common.ArgoCDManagedByLabel: a.Namespace,
		}},
	}

	err := r.Client.Create(context.TODO(), ns)
	assert.NilError(t, err)

	ns2 := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "testNamespace2",
		Labels: map[string]string{
			common.ArgoCDManagedByLabel: a.Namespace,
		}},
	}

	err = r.Client.Create(context.TODO(), ns2)
	assert.NilError(t, err)

	ns3 := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "testNamespace3",
		Labels: map[string]string{
			common.ArgoCDManagedByLabel: "newNamespace",
		}},
	}

	err = r.Client.Create(context.TODO(), ns3)
	assert.NilError(t, err)

	err = r.removeManagedByLabelFromNamespaces(a.Namespace)
	assert.NilError(t, err)

	nsList := &v1.NamespaceList{}
	err = r.Client.List(context.TODO(), nsList)
	assert.NilError(t, err)
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
