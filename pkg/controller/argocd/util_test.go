package argocd

import (
	"os"
	"testing"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
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
	name        string
	description string
	opts        []argoCDOpt
	want        string
}{
	{
		name:        "test with no host name - default",
		description: "test case when no hostname is provided and both ingress and route are disabled",
		want:        "https://argocd-server",
	},
	{
		name:        "test with external host name - no scheme",
		description: "test case when external hostname is provided without scheme and both ingress and route are disabled",
		opts: []argoCDOpt{func(a *argoprojv1alpha1.ArgoCD) {
			a.Spec.Server.Host = "test-host-name"
		}},
		want: "https://test-host-name",
	},
	{
		name:        "test with external host name - scheme provided",
		description: "test case when external hostname is provided with https scheme and both ingress and route are disabled",
		opts: []argoCDOpt{func(a *argoprojv1alpha1.ArgoCD) {
			a.Spec.Server.Host = "https://test-host-name"
		}},
		want: "https://test-host-name",
	},
	{
		name:        "test with external http host name",
		description: "test case when external hostname is provided with http scheme and both ingress and route are disabled",
		opts: []argoCDOpt{func(a *argoprojv1alpha1.ArgoCD) {
			a.Spec.Server.Host = "http://test-host-name"
		}},
		want: "http://test-host-name",
	},
}

func TestGetArgoServerURI(t *testing.T) {
	for _, tt := range argoServerURITests {
		cr := makeTestArgoCD(tt.opts...)
		r := &ReconcileArgoCD{}
		result := r.getArgoServerURI(cr)
		if result != tt.want {
			t.Errorf("%s test failed, got=%q want=%q", tt.name, result, tt.want)
		}
	}
}
