/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package scheme provides centralized scheme registration for all API types
package schema

import (
	"os"

	appsv1 "github.com/openshift/api/apps/v1"
	configv1 "github.com/openshift/api/config/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	v1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	v1beta1 "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd"
)

var (
	schemeLog = ctrl.Log.WithName("scheme")
)

func SetupScheme(scheme *runtime.Scheme) {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	registerArgoCDAPIs(scheme)
	registerPrometheusAPIsIfAvailable(scheme)
	registerOpenShiftAPIsIfAvailable(scheme)
	schemeLog.Info("Scheme setup complete.")
}

func registerArgoCDAPIs(scheme *runtime.Scheme) {
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(v1beta1.AddToScheme(scheme))
}

func registerPrometheusAPIsIfAvailable(scheme *runtime.Scheme) {
	if argocd.IsPrometheusAPIAvailable() {
		if err := monitoringv1.AddToScheme(scheme); err != nil {
			schemeLog.Error(err, "")
			os.Exit(1)
		}
	}
}

func registerOpenShiftAPIsIfAvailable(scheme *runtime.Scheme) {
	// Setup Scheme for OpenShift Routes if available.
	if argocd.IsRouteAPIAvailable() {
		if err := routev1.Install(scheme); err != nil {
			schemeLog.Error(err, "")
			os.Exit(1)
		}
	}

	// Setup the scheme for openshift config if available
	if argocd.IsVersionAPIAvailable() {
		if err := configv1.Install(scheme); err != nil {
			schemeLog.Error(err, "")
			os.Exit(1)
		}
	}

	// Setup Schemes for SSO if template instance is available.
	if argocd.CanUseKeycloakWithTemplate() {
		schemeLog.Info("Keycloak instance can be managed using OpenShift Template.")
		if err := appsv1.Install(scheme); err != nil {
			schemeLog.Error(err, "")
			os.Exit(1)
		}
		if err := templatev1.Install(scheme); err != nil {
			schemeLog.Error(err, "")
			os.Exit(1)
		}
		if err := oauthv1.Install(scheme); err != nil {
			schemeLog.Error(err, "")
			os.Exit(1)
		}
	} else {
		schemeLog.Info("Keycloak instance cannot be managed using OpenShift Template, as //DeploymentConfig/Template API is not present.")
	}
}
