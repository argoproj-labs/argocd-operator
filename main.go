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

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	goruntime "runtime"
	"strings"

	"github.com/argoproj/argo-cd/v2/util/env"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "github.com/openshift/api/apps/v1"
	configv1 "github.com/openshift/api/config/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	sdkVersion "github.com/operator-framework/operator-sdk/version"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd"
	"github.com/argoproj-labs/argocd-operator/controllers/argocdexport"
	"github.com/argoproj-labs/argocd-operator/pkg/monitoring"
	"github.com/argoproj-labs/argocd-operator/pkg/openshift"
	"github.com/argoproj-labs/argocd-operator/pkg/util"

	notificationsConfig "github.com/argoproj-labs/argocd-operator/controllers/notificationsconfiguration"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/version"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(argoprojv1alpha1.AddToScheme(scheme))
	utilruntime.Must(argoproj.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func printVersion() {
	setupLog.Info(fmt.Sprintf("Go Version: %s", goruntime.Version()))
	setupLog.Info(fmt.Sprintf("Go OS/Arch: %s/%s", goruntime.GOOS, goruntime.GOARCH))
	setupLog.Info(fmt.Sprintf("Version of operator-sdk: %v", sdkVersion.Version))
	setupLog.Info(fmt.Sprintf("Version of %s-operator: %v", common.ArgoCDAppName, version.Version))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var labelSelectorFlag string

	var secureMetrics = false
	var enableHTTP2 = false

	flag.StringVar(&metricsAddr, "metrics-bind-address", fmt.Sprintf(":%d", common.OperatorMetricsPort), "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&labelSelectorFlag, "label-selector", env.StringFromEnv(common.ArgoCDLabelSelectorKey, common.ArgoCDDefaultLabelSelector), "The label selector is used to map to a subset of ArgoCD instances to reconcile")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&enableHTTP2, "enable-http2", enableHTTP2, "If HTTP/2 should be enabled for the metrics and webhook servers.")
	flag.BoolVar(&secureMetrics, "metrics-secure", secureMetrics, "If the metrics endpoint should be served securely.")

	opts := ctrlzap.Options{
		Development: true,
		Encoder:     zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		TimeEncoder: zapcore.RFC3339TimeEncoder,
		Level:       util.GetLogLevel(util.GetEnv(common.ArgoCDOperatorLogLevelEnvVar)),
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	disableHTTP2 := func(c *tls.Config) {
		if enableHTTP2 {
			return
		}
		c.NextProtos = []string{"http/1.1"}
	}
	webhookServerOptions := webhook.Options{
		TLSOpts: []func(config *tls.Config){disableHTTP2},
		Port:    9443,
	}
	webhookServer := webhook.NewServer(webhookServerOptions)

	metricsServerOptions := metricsserver.Options{
		SecureServing: secureMetrics,
		BindAddress:   metricsAddr,
		TLSOpts:       []func(*tls.Config){disableHTTP2},
	}

	ctrl.SetLogger(ctrlzap.New(ctrlzap.UseFlagOptions(&opts)))

	printVersion()

	// Check the label selector format eg. "foo=bar"
	if _, err := labels.Parse(labelSelectorFlag); err != nil {
		setupLog.Error(err, "error parsing the labelSelector '%s'.", labelSelectorFlag)
		os.Exit(1)
	}
	setupLog.Info(fmt.Sprintf("Watching labelselector \"%s\"", labelSelectorFlag))

	// Inspect cluster to verify availability of extra features
	err := argocd.VerifyClusterAPIs()
	if err != nil {
		setupLog.Error(err, "error verifying one or more APIs on the cluster")
	}

	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		setupLog.Error(err, "Failed to get watch namespace, defaulting to all namespace mode")
	}
	setupLog.Info(fmt.Sprintf("Watching namespace \"%s\"", namespace))

	// Set default manager options
	options := manager.Options{
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		Scheme:                 scheme,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "b674928d.argoproj.io",
	}

	if watchedNsCache := getDefaultWatchedNamespacesCacheOptions(); watchedNsCache != nil {
		options.Cache = cache.Options{
			DefaultNamespaces: watchedNsCache,
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	setupLog.Info("Registering Components.")

	// Setup Scheme for all resources
	if err := argoprojv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}

	if err := argoproj.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}

	if err := argocd.VerifyClusterAPIs(); err != nil {
		setupLog.Error(err, "unable to verify certain APIs")
	}

	// Setup Scheme for Prometheus if available.
	if monitoring.IsPrometheusAPIAvailable() {
		if err := monitoringv1.AddToScheme(mgr.GetScheme()); err != nil {
			setupLog.Error(err, "")
			os.Exit(1)
		}
	}

	// Setup Scheme for OpenShift Routes if available.
	if openshift.IsRouteAPIAvailable() {
		if err := routev1.Install(mgr.GetScheme()); err != nil {
			setupLog.Error(err, "")
			os.Exit(1)
		}
	}

	// Set up the scheme for openshift config if available
	if openshift.IsVersionAPIAvailable() {
		if err := configv1.Install(mgr.GetScheme()); err != nil {
			setupLog.Error(err, "")
			os.Exit(1)
		}
	}

	// Setup Schemes for SSO if template instance is available.
	if openshift.IsTemplateAPIAvailable() {
		if err := templatev1.Install(mgr.GetScheme()); err != nil {
			setupLog.Error(err, "")
			os.Exit(1)
		}
		if err := appsv1.Install(mgr.GetScheme()); err != nil {
			setupLog.Error(err, "")
			os.Exit(1)
		}
		if err := oauthv1.Install(mgr.GetScheme()); err != nil {
			setupLog.Error(err, "")
			os.Exit(1)
		}
	}

	if err = (&argocd.ArgoCDReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		LabelSelector: labelSelectorFlag,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ArgoCD")
		os.Exit(1)
	}
	if err = (&argocdexport.ArgoCDExportReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ArgoCDExport")
		os.Exit(1)
	}
	if err = (&notificationsConfig.NotificationsConfigurationReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NotificationsConfiguration")
		os.Exit(1)
	}

	// Start webhook only if ENABLE_CONVERSION_WEBHOOK is set
	if strings.EqualFold(os.Getenv("ENABLE_CONVERSION_WEBHOOK"), "true") {
		if err = (&argoproj.ArgoCD{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "ArgoCD")
			os.Exit(1)
		}
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func getDefaultWatchedNamespacesCacheOptions() map[string]cache.Config {
	watchedNamespaces, err := k8sutil.GetWatchNamespace()
	if err != nil {
		setupLog.Error(err, "Failed to get watch namespace, defaulting to all namespace mode")
		return nil
	}

	if watchedNamespaces == "" {
		return nil
	}

	watchedNsList := strings.Split(watchedNamespaces, ",")
	setupLog.Info(fmt.Sprintf("Watching namespaces: %v", watchedNsList))

	defaultNamespacesCacheConfig := map[string]cache.Config{}
	for _, ns := range watchedNsList {
		defaultNamespacesCacheConfig[ns] = cache.Config{}
	}

	return defaultNamespacesCacheConfig
}
