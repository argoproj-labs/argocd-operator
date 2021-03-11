package argocd

import (
	"os"
	"reflect"
	"testing"
	"time"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	"gotest.tools/assert"
)

const (
	dexTestImage     = "testing/dex:latest"
	argoTestImage    = "testing/argocd:latest"
	grafanaTestImage = "testing/grafana:latest"
	redisTestImage   = "testing/redis:latest"
	redisHATestImage = "testing/redis:latest-ha"
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
		assert.ErrorContains(t, err, `failed to create a requirement for invalid label value: "argocd-*/`)
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
