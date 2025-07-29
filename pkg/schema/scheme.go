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
)

var (
	schemeLog = ctrl.Log.WithName("scheme")
)

// SchemeOptions provides toggles to register optional APIs.
type SchemeOptions struct {
	EnablePrometheus bool
	EnableRoutes     bool
	EnableVersion    bool
	EnableKeycloak   bool
}

func SetupScheme(scheme *runtime.Scheme, opts SchemeOptions) error {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	registerArgoCDAPIs(scheme)
	// Setup Scheme for Prometheus if enabled.
	if opts.EnablePrometheus {
		if err := monitoringv1.AddToScheme(scheme); err != nil {
			schemeLog.Error(err, "Failed to register Prometheus API")
			return err
		}
	}
	// Setup Scheme for OpenShift Routes if enabled.
	if opts.EnableRoutes {
		if err := routev1.Install(scheme); err != nil {
			schemeLog.Error(err, "Failed to register Route API")
			return err
		}
	}
	// Setup Scheme for OpenShift config if enabled.
	if opts.EnableVersion {
		if err := configv1.Install(scheme); err != nil {
			schemeLog.Error(err, "Failed to register Version API")
			return err
		}
	}
	// Setup Schemes for SSO if template instance is available.
	if opts.EnableKeycloak {
		schemeLog.Info("Keycloak instance can be managed using OpenShift Template.")
		if err := templatev1.Install(scheme); err != nil {
			schemeLog.Error(err, "Failed to register Template API")
			return err
		}
		if err := appsv1.Install(scheme); err != nil {
			schemeLog.Error(err, "Failed to register DeploymentConfig API")
			return err
		}
		if err := oauthv1.Install(scheme); err != nil {
			schemeLog.Error(err, "Failed to register OAuth API")
			return err
		}
	} else {
		schemeLog.Info("Keycloak instance cannot be managed using OpenShift Template, as //DeploymentConfig/Template API is not present.")
	}

	schemeLog.Info("Scheme setup complete.")
	return nil
}

func registerArgoCDAPIs(scheme *runtime.Scheme) {
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(v1beta1.AddToScheme(scheme))
}
