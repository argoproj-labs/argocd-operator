/*


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

package main

import (
	"flag"
	"fmt"
	"os"
	goruntime "runtime"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/resources"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	v1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/controllers"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

// init will setup the schemes used by the operator.
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	// Setup Scheme for Prometheus if available.
	if resources.IsPrometheusAPIAvailable() {
		if err := monitoringv1.AddToScheme(scheme); err != nil {
			setupLog.Error(err, "")
			os.Exit(1)
		}
	}

	// Setup Scheme for OpenShift Routes if available.
	if resources.IsRouteAPIAvailable() {
		if err := routev1.AddToScheme(scheme); err != nil {
			setupLog.Error(err, "")
			os.Exit(1)
		}
	}

	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// main is the entrypoint for the application.
func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	printVersion()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "b674928d.argoproj.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.ArgoCDReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("ArgoCD"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ArgoCD")
		os.Exit(1)
	}
	if err = (&controllers.ArgoCDExportReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("ArgoCDExport"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ArgoCDExport")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// printVersion will output the runtime environment and version information.
func printVersion() {
	setupLog.Info(fmt.Sprintf("go Version: %s", goruntime.Version()))
	setupLog.Info(fmt.Sprintf("go os/arch: %s/%s", goruntime.GOOS, goruntime.GOARCH))
	setupLog.Info(fmt.Sprintf("%s-operator version: %v", common.ArgoCDAppName, common.Version))
}
