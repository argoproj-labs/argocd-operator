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

	"github.com/argoproj/argo-cd/v3/util/env"
	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd"
	"github.com/argoproj-labs/argocd-operator/controllers/argocdexport"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"

	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	notificationsConfig "github.com/argoproj-labs/argocd-operator/controllers/notificationsconfiguration"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	corev1 "k8s.io/api/core/v1"

	v1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	v1beta1 "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/cacheutils"
	cw "github.com/argoproj-labs/argocd-operator/pkg/clientwrapper"
	"github.com/argoproj-labs/argocd-operator/version"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(v1beta1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func printVersion() {
	setupLog.Info(fmt.Sprintf("Go Version: %s", goruntime.Version()))
	setupLog.Info(fmt.Sprintf("Go OS/Arch: %s/%s", goruntime.GOOS, goruntime.GOARCH))
	setupLog.Info(fmt.Sprintf("Version of %s-operator: %v", common.ArgoCDAppName, version.Version))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var labelSelectorFlag string

	var secureMetrics = false
	var enableHTTP2 = false
	var skipControllerNameValidation = true

	flag.StringVar(&metricsAddr, "metrics-bind-address", fmt.Sprintf(":%d", common.OperatorMetricsPort), "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&labelSelectorFlag, "label-selector", env.StringFromEnv(common.ArgoCDLabelSelectorKey, common.ArgoCDDefaultLabelSelector), "The label selector is used to map to a subset of ArgoCD instances to reconcile")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&enableHTTP2, "enable-http2", enableHTTP2, "If HTTP/2 should be enabled for the metrics and webhook servers.")
	flag.BoolVar(&secureMetrics, "metrics-secure", secureMetrics, "If the metrics endpoint should be served securely.")

	//Configure log level
	logLevelStr := strings.ToLower(os.Getenv("LOG_LEVEL"))
	logLevel := zapcore.InfoLevel
	switch logLevelStr {
	case "debug":
		logLevel = zapcore.DebugLevel
	case "info":
		logLevel = zapcore.InfoLevel
	case "warn":
		logLevel = zapcore.WarnLevel
	case "error":
		logLevel = zapcore.ErrorLevel
	case "panic":
		logLevel = zapcore.PanicLevel
	case "fatal":
		logLevel = zapcore.FatalLevel
	}

	opts := zap.Options{
		Level:       logLevel,
		Development: true,
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

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	printVersion()

	// Check the label selector format eg. "foo=bar"
	if _, err := labels.Parse(labelSelectorFlag); err != nil {
		setupLog.Error(err, "error parsing the labelSelector '%s'.", labelSelectorFlag)
		os.Exit(1)
	}
	setupLog.Info(fmt.Sprintf("Watching labelselector \"%s\"", labelSelectorFlag))

	// Inspect cluster to verify availability of extra features
	if err := argocd.InspectCluster(); err != nil {
		setupLog.Info("unable to inspect cluster")
	}

	namespace, err := getWatchNamespace()
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
		// With controller-runtime v0.19.0, unique controller name validation is
		// enforced. The operator may fail to start due to this as we don't have unique
		// names. Use SkipNameValidation to ingnore the uniquness check and prevent panic.
		Controller: controllerconfig.Controller{
			SkipNameValidation: &skipControllerNameValidation,
		},
	}

	// Use transformers to strip data from Secrets and ConfigMaps
	// that are not tracked by the operator to reduce memory usage.
	if strings.ToLower(os.Getenv("MEMORY_OPTIMIZATION_ENABLED")) != "false" {
		setupLog.Info("memory optimization is enabled")
		options.Cache = cache.Options{
			Scheme: scheme,
			ByObject: map[crclient.Object]cache.ByObject{
				&corev1.Secret{}:    {Transform: cacheutils.StripDataFromSecretOrConfigMapTransform()},
				&corev1.ConfigMap{}: {Transform: cacheutils.StripDataFromSecretOrConfigMapTransform()},
			},
		}
	}

	if watchedNsCache := getDefaultWatchedNamespacesCacheOptions(); watchedNsCache != nil {
		options.Cache.DefaultNamespaces = watchedNsCache
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	var client crclient.Client
	if strings.ToLower(os.Getenv("MEMORY_OPTIMIZATION_ENABLED")) != "false" {
		liveClient, err := crclient.New(ctrl.GetConfigOrDie(), crclient.Options{Scheme: mgr.GetScheme()})
		if err != nil {
			setupLog.Error(err, "unable to create live client")
			os.Exit(1)
		}

		// Wraps the controller runtime's default client to provide:
		//   1. Fallback to the live client when a Secret/ConfigMap is stripped in the cache.
		//   2. Automatic labeling of fetched objects, so they are retained in full form
		//      in subsequent cache updates and avoid repeated live lookups.
		client = cw.NewClientWrapper(mgr.GetClient(), liveClient)
	} else {
		client = mgr.GetClient()
	}
	setupLog.Info("Registering Components.")

	// Setup Scheme for all resources
	if err := v1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}

	if err := v1beta1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}

	// Setup Scheme for Prometheus if available.
	if argocd.IsPrometheusAPIAvailable() {
		if err := monitoringv1.AddToScheme(mgr.GetScheme()); err != nil {
			setupLog.Error(err, "")
			os.Exit(1)
		}
	}

	// Setup Scheme for OpenShift Routes if available.
	if argoutil.IsRouteAPIAvailable() {
		if err := routev1.Install(mgr.GetScheme()); err != nil {
			setupLog.Error(err, "")
			os.Exit(1)
		}
	}

	// Set up the scheme for openshift config if available
	if argocd.IsVersionAPIAvailable() {
		if err := configv1.Install(mgr.GetScheme()); err != nil {
			setupLog.Error(err, "")
			os.Exit(1)
		}
	}

	k8sClient, err := initK8sClient()
	if err != nil {
		setupLog.Error(err, "Failed to initialize Kubernetes client")
		os.Exit(1)
	}
	if err = (&argocd.ReconcileArgoCD{
		Client:        client,
		Scheme:        mgr.GetScheme(),
		LabelSelector: labelSelectorFlag,
		K8sClient:     k8sClient,
		LocalUsers: &argocd.LocalUsersInfo{
			TokenRenewalTimers: map[string]*argocd.TokenRenewalTimer{},
		},
		FipsConfigChecker: argoutil.NewLinuxFipsConfigChecker(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ArgoCD")
		os.Exit(1)
	}
	if err = (&argocdexport.ReconcileArgoCDExport{
		Client: client,
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ArgoCDExport")
		os.Exit(1)
	}
	if err = (&notificationsConfig.NotificationsConfigurationReconciler{
		Client: client,
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NotificationsConfiguration")
		os.Exit(1)
	}
	// Register validation webhook for ArgoCD v1beta1
	if err = (v1beta1.ArgoCD{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create validation webhook", "webhook", "ArgoCD")
		os.Exit(1)
	}

	// Start conversion webhook only if ENABLE_CONVERSION_WEBHOOK is set
	if strings.EqualFold(os.Getenv("ENABLE_CONVERSION_WEBHOOK"), "true") {
		if err = (v1beta1.ArgoCD{}).SetupWebhookWithManager(mgr); err != nil {
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
	watchedNamespaces, err := getWatchNamespace()
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

// getWatchNamespace returns the Namespace the operator should be watching for changes
func getWatchNamespace() (string, error) {
	// WatchNamespaceEnvVar is the constant for env variable WATCH_NAMESPACE
	// which specifies the Namespace to watch.
	// An empty value means the operator is running with cluster scope.
	var watchNamespaceEnvVar = "WATCH_NAMESPACE"

	ns, found := os.LookupEnv(watchNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", watchNamespaceEnvVar)
	}
	return ns, nil
}

func initK8sClient() (*kubernetes.Clientset, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		setupLog.Error(err, "unable to get k8s config")
		return nil, err
	}

	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		setupLog.Error(err, "unable to create k8s client")
		return nil, err
	}

	return k8sClient, nil
}
