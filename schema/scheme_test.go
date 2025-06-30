package schema

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/controllers/argocd"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestSetupScheme(t *testing.T) {
	t.Run("SetupScheme should register all required schemes", func(t *testing.T) {
		scheme := runtime.NewScheme()

		// Call SetupScheme
		SetupScheme(scheme)

		// Verify the scheme is not nil
		assert.NotNil(t, scheme, "Scheme should not be nil")

		// Verify that ArgoCD API schemes are registered
		v1alpha1GVK := schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "ArgoCD"}
		v1beta1GVK := schema.GroupVersionKind{Group: "argoproj.io", Version: "v1beta1", Kind: "ArgoCD"}
		assert.True(t, scheme.Recognizes(v1alpha1GVK), "v1alpha1 ArgoCD scheme should be registered")
		assert.True(t, scheme.Recognizes(v1beta1GVK), "v1beta1 ArgoCD scheme should be registered")

		// Verify conditional schemes based on API availability
		if argocd.IsPrometheusAPIAvailable() {
			prometheusGVK := schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "Prometheus"}
			assert.True(t, scheme.Recognizes(prometheusGVK), "Prometheus scheme should be registered when API is available")
		}

		if argocd.IsRouteAPIAvailable() {
			routeGVK := schema.GroupVersionKind{Group: "route.openshift.io", Version: "v1", Kind: "Route"}
			assert.True(t, scheme.Recognizes(routeGVK), "OpenShift Route scheme should be registered when API is available")
		}

		if argocd.IsVersionAPIAvailable() {
			clusterVersionGVK := schema.GroupVersionKind{Group: "config.openshift.io", Version: "v1", Kind: "ClusterVersion"}
			assert.True(t, scheme.Recognizes(clusterVersionGVK), "OpenShift Config scheme should be registered when API is available")
		}

		if argocd.CanUseKeycloakWithTemplate() {
			templateGVK := schema.GroupVersionKind{Group: "template.openshift.io", Version: "v1", Kind: "Template"}
			deploymentConfigGVK := schema.GroupVersionKind{Group: "apps.openshift.io", Version: "v1", Kind: "DeploymentConfig"}
			oauthClientGVK := schema.GroupVersionKind{Group: "oauth.openshift.io", Version: "v1", Kind: "OAuthClient"}

			assert.True(t, scheme.Recognizes(templateGVK), "OpenShift Template scheme should be registered when Keycloak can use templates")
			assert.True(t, scheme.Recognizes(deploymentConfigGVK), "OpenShift DeploymentConfig scheme should be registered when Keycloak can use templates")
			assert.True(t, scheme.Recognizes(oauthClientGVK), "OpenShift OAuth scheme should be registered when Keycloak can use templates")
		}
	})

	t.Run("SetupScheme should not panic with empty scheme", func(t *testing.T) {
		scheme := runtime.NewScheme()

		// This should not panic
		assert.NotPanics(t, func() {
			SetupScheme(scheme)
		}, "SetupScheme should not panic with empty scheme")
	})
}
