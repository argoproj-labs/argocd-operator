package argocd

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	argopass "github.com/argoproj/argo-cd/v2/util/password"
	"github.com/prometheus/client_golang/prometheus"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/server"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	configv1 "github.com/openshift/api/config/v1"
	templatev1 "github.com/openshift/api/template/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/sethvargo/go-password/password"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// DeprecationEventEmissionStatus is meant to track which deprecation events have been emitted already. This is temporary and can be removed in v0.0.6 once we have provided enough
// deprecation notice
type DeprecationEventEmissionStatus struct {
	SSOSpecDeprecationWarningEmitted    bool
	DexSpecDeprecationWarningEmitted    bool
	DisableDexDeprecationWarningEmitted bool
}

// DeprecationEventEmissionTracker map stores the namespace containing ArgoCD instance as key and DeprecationEventEmissionStatus as value,
// where DeprecationEventEmissionStatus tracks the events that have been emitted for the instance in the particular namespace.
// This is temporary and can be removed in v0.0.6 when we remove the deprecated fields.
var DeprecationEventEmissionTracker = make(map[string]DeprecationEventEmissionStatus)

var (
	versionAPIFound    = false
	prometheusAPIFound = false
	routeAPIFound      = false
	templateAPIFound   = false
)

// IsVersionAPIAvailable returns true if the version api is present
func IsVersionAPIAvailable() bool {
	return versionAPIFound
}

// verifyVersionAPI will verify that the template API is present.
func verifyVersionAPI() error {
	found, err := argoutil.VerifyAPI(configv1.GroupName, configv1.GroupVersion.Version)
	if err != nil {
		return err
	}
	versionAPIFound = found
	return nil
}

// IsRouteAPIAvailable returns true if the Route API is present.
func IsRouteAPIAvailable() bool {
	return routeAPIFound
}

// verifyRouteAPI will verify that the Route API is present.
func verifyRouteAPI() error {
	found, err := argoutil.VerifyAPI(routev1.GroupName, routev1.GroupVersion.Version)
	if err != nil {
		return err
	}
	routeAPIFound = found
	return nil
}

// IsPrometheusAPIAvailable returns true if the Prometheus API is present.
func IsPrometheusAPIAvailable() bool {
	return prometheusAPIFound
}

// verifyPrometheusAPI will verify that the Prometheus API is present.
func verifyPrometheusAPI() error {
	found, err := argoutil.VerifyAPI(monitoringv1.SchemeGroupVersion.Group, monitoringv1.SchemeGroupVersion.Version)
	if err != nil {
		return err
	}
	prometheusAPIFound = found
	return nil
}

// IsTemplateAPIAvailable returns true if the template API is present.
func IsTemplateAPIAvailable() bool {
	return templateAPIFound
}

// verifyTemplateAPI will verify that the template API is present.
func verifyTemplateAPI() error {
	found, err := argoutil.VerifyAPI(templatev1.GroupVersion.Group, templatev1.GroupVersion.Version)
	if err != nil {
		return err
	}
	templateAPIFound = found
	return nil
}

// generateArgoAdminPassword will generate and return the admin password for Argo CD.
func generateArgoAdminPassword() ([]byte, error) {
	pass, err := password.Generate(
		common.ArgoCDDefaultAdminPasswordLength,
		common.ArgoCDDefaultAdminPasswordNumDigits,
		common.ArgoCDDefaultAdminPasswordNumSymbols,
		false, false)

	return []byte(pass), err
}

// generateArgoServerKey will generate and return the server signature key for session validation.
func generateArgoServerSessionKey() ([]byte, error) {
	pass, err := password.Generate(
		common.ArgoCDDefaultServerSessionKeyLength,
		common.ArgoCDDefaultServerSessionKeyNumDigits,
		common.ArgoCDDefaultServerSessionKeyNumSymbols,
		false, false)

	return []byte(pass), err
}

// nameWithSuffix will return a name based on the given ArgoCD. The given suffix is appended to the generated name.
// Example: Given an ArgoCD with the name "example-argocd", providing the suffix "foo" would result in the value of
// "example-argocd-foo" being returned.
func nameWithSuffix(suffix string, cr *argoproj.ArgoCD) string {
	return fmt.Sprintf("%s-%s", cr.Name, suffix)
}

// contains returns true if a string is part of the given slice.
func contains(s []string, g string) bool {
	for _, a := range s {
		if a == g {
			return true
		}
	}
	return false
}

// generateRandomBytes returns a securely generated random bytes.
func generateRandomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		log.Error(err, "")
	}
	return b
}

// generateRandomString returns a securely generated random string.
func generateRandomString(s int) string {
	b := generateRandomBytes(s)
	return base64.URLEncoding.EncodeToString(b)
}

// getClusterVersion returns the OpenShift Cluster version in which the operator is installed
func getClusterVersion(client client.Client) (string, error) {
	if !IsVersionAPIAvailable() {
		return "", nil
	}
	clusterVersion := &configv1.ClusterVersion{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: "version"}, clusterVersion)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	return clusterVersion.Status.Desired.Version, nil
}

func AddSeccompProfileForOpenShift(client client.Client, podspec *corev1.PodSpec) {
	if !IsVersionAPIAvailable() {
		return
	}
	version, err := getClusterVersion(client)
	if err != nil {
		log.Error(err, "couldn't get OpenShift version")
	}
	if version == "" || semver.Compare(fmt.Sprintf("v%s", version), "v4.10.999") > 0 {
		if podspec.SecurityContext == nil {
			podspec.SecurityContext = &corev1.PodSecurityContext{}
		}
		if podspec.SecurityContext.SeccompProfile == nil {
			podspec.SecurityContext.SeccompProfile = &corev1.SeccompProfile{}
		}
		if len(podspec.SecurityContext.SeccompProfile.Type) == 0 {
			podspec.SecurityContext.SeccompProfile.Type = corev1.SeccompProfileTypeRuntimeDefault
		}
	}
}

func isProxyCluster() bool {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "failed to get k8s config")
	}

	// Initialize config client.
	configClient, err := configv1client.NewForConfig(cfg)
	if err != nil {
		log.Error(err, "failed to initialize openshift config client")
		return false
	}

	proxy, err := configClient.Proxies().Get(context.TODO(), "cluster", metav1.GetOptions{})
	if err != nil {
		log.Error(err, "failed to get proxy configuration")
		return false
	}

	if proxy.Spec.HTTPSProxy != "" {
		log.Info("proxy configuration detected")
		return true
	}

	return false
}

func getOpenShiftAPIURL() string {
	k8s, err := initK8sClient()
	if err != nil {
		log.Error(err, "failed to initialize k8s client")
	}

	cm, err := k8s.CoreV1().ConfigMaps("openshift-console").Get(context.TODO(), "console-config", metav1.GetOptions{})
	if err != nil {
		log.Error(err, "")
	}

	var cf string
	if v, ok := cm.Data["console-config.yaml"]; ok {
		cf = v
	}

	data := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(cf), data)
	if err != nil {
		log.Error(err, "")
	}

	var apiURL interface{}
	var out string
	if c, ok := data["clusterInfo"]; ok {
		ci, _ := c.(map[interface{}]interface{})

		apiURL = ci["masterPublicURL"]
		out = fmt.Sprintf("%v", apiURL)
	}

	return out
}

func initK8sClient() (*kubernetes.Clientset, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to get k8s config")
		return nil, err
	}

	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Error(err, "unable to create k8s client")
		return nil, err
	}

	return k8sClient, nil
}

// getLogLevel returns the log level for a specified component if it is set or returns the default log level if it is not set
func getLogLevel(logField string) string {

	switch strings.ToLower(logField) {
	case "debug",
		"info",
		"warn",
		"error":
		return logField
	}
	return common.ArgoCDDefaultLogLevel
}

// getLogFormat returns the log format for a specified component if it is set or returns the default log format if it is not set
func getLogFormat(logField string) string {
	switch strings.ToLower(logField) {
	case "text",
		"json":
		return logField
	}
	return common.ArgoCDDefaultLogFormat
}

func containsString(arr []string, s string) bool {
	for _, val := range arr {
		if strings.TrimSpace(val) == s {
			return true
		}
	}
	return false
}

func splitList(s string) []string {
	elems := strings.Split(s, ",")
	for i := range elems {
		elems[i] = strings.TrimSpace(elems[i])
	}
	return elems
}

// boolPtr returns a pointer to val
func boolPtr(val bool) *bool {
	return &val
}

func int64Ptr(val int64) *int64 {
	return &val
}

// loadTemplateFile will parse a template with the given path and execute it with the given params.
func loadTemplateFile(path string, params map[string]string) (string, error) {
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		log.Error(err, "unable to parse template")
		return "", err
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, params)
	if err != nil {
		log.Error(err, "unable to execute template")
		return "", err
	}
	return buf.String(), nil
}

// fqdnServiceRef will return the FQDN referencing a specific service name, as set up by the operator, with the
// given port.
func fqdnServiceRef(service string, port int, cr *argoproj.ArgoCD) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", nameWithSuffix(service, cr), cr.Namespace, port)
}

// InspectCluster will verify the availability of extra features available to the cluster, such as Prometheus and
// OpenShift Routes.
func InspectCluster() error {
	if err := verifyPrometheusAPI(); err != nil {
		return err
	}

	if err := verifyRouteAPI(); err != nil {
		return err
	}

	if err := verifyTemplateAPI(); err != nil {
		return err
	}

	if err := verifyVersionAPI(); err != nil {
		return err
	}
	return nil
}

func allowedNamespace(current string, namespaces string) bool {

	clusterConfigNamespaces := splitList(namespaces)
	if len(clusterConfigNamespaces) > 0 {
		if clusterConfigNamespaces[0] == "*" {
			return true
		}

		for _, n := range clusterConfigNamespaces {
			if n == current {
				return true
			}
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	var result []string
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return result
}

func proxyEnvVars(vars ...corev1.EnvVar) []corev1.EnvVar {
	result := []corev1.EnvVar{}
	result = append(result, vars...)
	proxyKeys := []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"}
	for _, p := range proxyKeys {
		if k, v := caseInsensitiveGetenv(p); k != "" {
			result = append(result, corev1.EnvVar{Name: k, Value: v})
		}
	}
	return result
}

func caseInsensitiveGetenv(s string) (string, string) {
	if v := os.Getenv(s); v != "" {
		return s, v
	}
	ls := strings.ToLower(s)
	if v := os.Getenv(ls); v != "" {
		return ls, v
	}
	return "", ""
}

// getArgoContainerImage will return the container image for ArgoCD.
func getArgoContainerImage(cr *argoproj.ArgoCD) string {
	defaultTag, defaultImg := false, false
	img := cr.Spec.Image
	if img == "" {
		img = common.ArgoCDDefaultArgoImage
		defaultImg = true
	}

	tag := cr.Spec.Version
	if tag == "" {
		tag = common.ArgoCDDefaultArgoVersion
		defaultTag = true
	}
	if e := os.Getenv(common.ArgoCDImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}

	return argoutil.CombineImageTag(img, tag)
}

// isMergable returns error if any of the extraArgs is already part of the default command Arguments.
func isMergable(extraArgs []string, cmd []string) error {
	if len(extraArgs) > 0 {
		for _, arg := range extraArgs {
			if len(arg) > 2 && arg[:2] == "--" {
				if ok := contains(cmd, arg); ok {
					err := errors.New("duplicate argument error")
					log.Error(err, fmt.Sprintf("Arg %s is already part of the default command arguments", arg))
					return err
				}
			}
		}
	}
	return nil
}

func (r *ReconcileArgoCD) setManagedNamespaces(cr *argoproj.ArgoCD) error {
	namespaces := &corev1.NamespaceList{}
	listOption := client.MatchingLabels{
		common.ArgoCDManagedByLabel: cr.Namespace,
	}

	// get the list of namespaces managed by the Argo CD instance
	if err := r.Client.List(context.TODO(), namespaces, listOption); err != nil {
		return err
	}

	namespaces.Items = append(namespaces.Items, corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: cr.Namespace}})
	r.ManagedNamespaces = namespaces
	return nil
}

func (r *ReconcileArgoCD) setManagedSourceNamespaces(cr *argoproj.ArgoCD) error {
	r.ManagedSourceNamespaces = make(map[string]string)
	namespaces := &corev1.NamespaceList{}
	listOption := client.MatchingLabels{
		common.ArgoCDManagedByClusterArgoCDLabel: cr.Namespace,
	}

	// get the list of namespaces managed by the Argo CD instance
	if err := r.Client.List(context.TODO(), namespaces, listOption); err != nil {
		return err
	}

	for _, namespace := range namespaces.Items {
		r.ManagedSourceNamespaces[namespace.Name] = ""
	}

	return nil
}

func filterObjectsBySelector(c client.Client, objectList client.ObjectList, selector labels.Selector) error {
	return c.List(context.TODO(), objectList, client.MatchingLabelsSelector{Selector: selector})
}

// hasArgoAdminPasswordChanged will return true if the Argo admin password has changed.
func hasArgoAdminPasswordChanged(actual *corev1.Secret, expected *corev1.Secret) bool {
	actualPwd := string(actual.Data[common.ArgoCDKeyAdminPassword])
	expectedPwd := string(expected.Data[common.ArgoCDKeyAdminPassword])

	validPwd, _ := argopass.VerifyPassword(expectedPwd, actualPwd)
	if !validPwd {
		log.Info("admin password has changed")
		return true
	}
	return false
}

// hasArgoTLSChanged will return true if the Argo TLS certificate or key have changed.
func hasArgoTLSChanged(actual *corev1.Secret, expected *corev1.Secret) bool {
	actualCert := string(actual.Data[common.ArgoCDKeyTLSCert])
	actualKey := string(actual.Data[common.ArgoCDKeyTLSPrivateKey])
	expectedCert := string(expected.Data[common.ArgoCDKeyTLSCert])
	expectedKey := string(expected.Data[common.ArgoCDKeyTLSPrivateKey])

	if actualCert != expectedCert || actualKey != expectedKey {
		log.Info("tls secret has changed")
		return true
	}
	return false
}

// nowBytes is a shortcut function to return the current date/time in RFC3339 format.
func nowBytes() []byte {
	return []byte(time.Now().UTC().Format(time.RFC3339))
}

// nowNano returns a string with the current UTC time as epoch in nanoseconds
func nowNano() string {
	return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
}

// newRoute returns a new Route instance for the given ArgoCD.
func newRoute(cr *argoproj.ArgoCD) *routev1.Route {
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}
}

// newRouteWithName returns a new Route with the given name and ArgoCD.
func newRouteWithName(name string, cr *argoproj.ArgoCD) *routev1.Route {
	route := newRoute(cr)
	route.ObjectMeta.Name = name

	lbls := route.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	route.ObjectMeta.Labels = lbls

	return route
}

// newRouteWithSuffix returns a new Route with the given name suffix for the ArgoCD.
func newRouteWithSuffix(suffix string, cr *argoproj.ArgoCD) *routev1.Route {
	return newRouteWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), cr)
}

// triggerDeploymentRollout will update the label with the given key to trigger a new rollout of the Deployment.
func (r *ReconcileArgoCD) triggerDeploymentRollout(deployment *appsv1.Deployment, key string) error {
	if !argoutil.IsObjectFound(r.Client, deployment.Namespace, deployment.Name, deployment) {
		log.Info(fmt.Sprintf("unable to locate deployment with name: %s", deployment.Name))
		return nil
	}

	deployment.Spec.Template.ObjectMeta.Labels[key] = nowNano()
	return r.Client.Update(context.TODO(), deployment)
}

// triggerStatefulSetRollout will update the label with the given key to trigger a new rollout of the StatefulSet.
func (r *ReconcileArgoCD) triggerStatefulSetRollout(sts *appsv1.StatefulSet, key string) error {
	if !argoutil.IsObjectFound(r.Client, sts.Namespace, sts.Name, sts) {
		log.Info(fmt.Sprintf("unable to locate deployment with name: %s", sts.Name))
		return nil
	}

	sts.Spec.Template.ObjectMeta.Labels[key] = nowNano()
	return r.Client.Update(context.TODO(), sts)
}

// triggerRollout will trigger a rollout of a Kubernetes resource specified as
// obj. It currently supports Deployment and StatefulSet resources.
func (r *ReconcileArgoCD) triggerRollout(obj interface{}, key string) error {
	switch res := obj.(type) {
	case *appsv1.Deployment:
		return r.triggerDeploymentRollout(res, key)
	case *appsv1.StatefulSet:
		return r.triggerStatefulSetRollout(res, key)
	default:
		return fmt.Errorf("resource of unknown type %T, cannot trigger rollout", res)
	}
}

// ensureAutoTLSAnnotation ensures that the service svc has the desired state
// of the auto TLS annotation set, which is either set (when enabled is true)
// or unset (when enabled is false).
//
// Returns true when annotations have been updated, otherwise returns false.
//
// When this method returns true, the svc resource will need to be updated on
// the cluster.
func ensureAutoTLSAnnotation(svc *corev1.Service, secretName string, enabled bool) bool {
	var autoTLSAnnotationName, autoTLSAnnotationValue string

	// We currently only support OpenShift for automatic TLS
	if IsRouteAPIAvailable() {
		autoTLSAnnotationName = common.AnnotationOpenShiftServiceCA
		if svc.Annotations == nil {
			svc.Annotations = make(map[string]string)
		}
		autoTLSAnnotationValue = secretName
	}

	if autoTLSAnnotationName != "" {
		val, ok := svc.Annotations[autoTLSAnnotationName]
		if enabled {
			if !ok || val != secretName {
				log.Info(fmt.Sprintf("requesting AutoTLS on service %s", svc.ObjectMeta.Name))
				svc.Annotations[autoTLSAnnotationName] = autoTLSAnnotationValue
				return true
			}
		} else {
			if ok {
				log.Info(fmt.Sprintf("removing AutoTLS from service %s", svc.ObjectMeta.Name))
				delete(svc.Annotations, autoTLSAnnotationName)
				return true
			}
		}
	}

	return false
}

func appendOpenShiftNonRootSCC(rules []v1.PolicyRule, client client.Client) []v1.PolicyRule {
	if IsVersionAPIAvailable() {
		// Starting with OpenShift 4.11, we need to use the resource name "nonroot-v2" instead of "nonroot"
		resourceName := "nonroot"
		version, err := getClusterVersion(client)
		if err != nil {
			log.Error(err, "couldn't get OpenShift version")
		}
		if version == "" || semver.Compare(fmt.Sprintf("v%s", version), "v4.10.999") > 0 {
			resourceName = "nonroot-v2"
		}
		orules := v1.PolicyRule{
			APIGroups: []string{
				"security.openshift.io",
			},
			ResourceNames: []string{
				resourceName,
			},
			Resources: []string{
				"securitycontextconstraints",
			},
			Verbs: []string{
				"use",
			},
		}
		rules = append(rules, orules)
	}
	return rules
}

func (r *ReconcileArgoCD) addDeletionFinalizer(argocd *argoproj.ArgoCD) error {
	argocd.Finalizers = append(argocd.Finalizers, common.ArgoCDDeletionFinalizer)
	if err := r.Client.Update(context.TODO(), argocd); err != nil {
		return fmt.Errorf("failed to add deletion finalizer for %s: %w", argocd.Name, err)
	}
	return nil
}

// old reconcile function - leave as is
func (r *ReconcileArgoCD) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {

	reconcileStartTS := time.Now()
	defer func() {
		ReconcileTime.WithLabelValues(request.Namespace).Observe(time.Since(reconcileStartTS).Seconds())
	}()

	reqLogger := ctrlLog.FromContext(ctx, "namespace", request.Namespace, "name", request.Name)
	reqLogger.Info("Reconciling ArgoCD")

	argocd := &argoproj.ArgoCD{}
	err := r.Client.Get(ctx, request.NamespacedName, argocd)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Fetch labelSelector from r.LabelSelector (command-line option)
	labelSelector, err := labels.Parse(r.LabelSelector)
	if err != nil {
		reqLogger.Info(fmt.Sprintf("error parsing the labelSelector '%s'.", labelSelector))
		return reconcile.Result{}, err
	}
	// Match the value of labelSelector from ReconcileArgoCD to labels from the argocd instance
	if !labelSelector.Matches(labels.Set(argocd.Labels)) {
		reqLogger.Info(fmt.Sprintf("the ArgoCD instance '%s' does not match the label selector '%s' and skipping for reconciliation", request.NamespacedName, r.LabelSelector))
		return reconcile.Result{}, fmt.Errorf("Error: failed to reconcile ArgoCD instance: '%s'", request.NamespacedName)
	}

	newPhase := argocd.Status.Phase
	// If we discover a new Argo CD instance in a previously un-seen namespace
	// we add it to the map and increment active instance count by phase
	// as well as total active instance count
	if _, ok := ActiveInstanceMap[request.Namespace]; !ok {
		if newPhase != "" {
			ActiveInstanceMap[request.Namespace] = newPhase
			ActiveInstancesByPhase.WithLabelValues(newPhase).Inc()
			ActiveInstancesTotal.Inc()
		}
	} else {
		// If we discover an existing instance's phase has changed since we last saw it
		// increment instance count with new phase and decrement instance count with old phase
		// update the phase in corresponding map entry
		// total instance count remains the same
		if oldPhase := ActiveInstanceMap[argocd.Namespace]; oldPhase != newPhase {
			ActiveInstanceMap[argocd.Namespace] = newPhase
			ActiveInstancesByPhase.WithLabelValues(newPhase).Inc()
			ActiveInstancesByPhase.WithLabelValues(oldPhase).Dec()
		}
	}

	ActiveInstanceReconciliationCount.WithLabelValues(argocd.Namespace).Inc()

	if argocd.GetDeletionTimestamp() != nil {

		// Argo CD instance marked for deletion; remove entry from activeInstances map and decrement active instance count
		// by phase as well as total
		delete(ActiveInstanceMap, argocd.Namespace)
		ActiveInstancesByPhase.WithLabelValues(newPhase).Dec()
		ActiveInstancesTotal.Dec()
		ActiveInstanceReconciliationCount.DeleteLabelValues(argocd.Namespace)
		ReconcileTime.DeletePartialMatch(prometheus.Labels{"namespace": argocd.Namespace})

		if argocd.IsDeletionFinalizerPresent() {
			if err := r.deleteClusterResources(argocd); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to delete ClusterResources: %w", err)
			}

			if isRemoveManagedByLabelOnArgoCDDeletion() {
				if err := r.removeManagedByLabelFromNamespaces(argocd.Namespace); err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to remove label from namespace[%v], error: %w", argocd.Namespace, err)
				}
			}

			if err := r.removeUnmanagedSourceNamespaceResources(argocd); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to remove resources from sourceNamespaces, error: %w", err)
			}

			if err := r.removeDeletionFinalizer(argocd); err != nil {
				return reconcile.Result{}, err
			}

			// remove namespace of deleted Argo CD instance from deprecationEventEmissionTracker (if exists) so that if another instance
			// is created in the same namespace in the future, that instance is appropriately tracked
			delete(DeprecationEventEmissionTracker, argocd.Namespace)
		}
		return reconcile.Result{}, nil
	}

	if !argocd.IsDeletionFinalizerPresent() {
		if err := r.addDeletionFinalizer(argocd); err != nil {
			return reconcile.Result{}, err
		}
	}

	// get the latest version of argocd instance before reconciling
	if err = r.Client.Get(ctx, request.NamespacedName, argocd); err != nil {
		return reconcile.Result{}, err
	}

	if err = r.setManagedNamespaces(argocd); err != nil {
		return reconcile.Result{}, err
	}

	if err = r.setManagedSourceNamespaces(argocd); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcileResources(argocd); err != nil {
		// Error reconciling ArgoCD sub-resources - requeue the request.
		return reconcile.Result{}, err
	}

	// Return and don't requeue
	return reconcile.Result{}, nil
}

func namespaceFilterPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// This checks if ArgoCDManagedByLabel exists in newMeta, if exists then -
			// 1. Check if oldMeta had the label or not? if no, return true
			// 2. if yes, check if the old and new values are different, if yes,
			// first deleteRBACs for the old value & return true.
			// Event is then handled by the reconciler, which would create appropriate RBACs.
			if valNew, ok := e.ObjectNew.GetLabels()[common.ArgoCDManagedByLabel]; ok {
				if valOld, ok := e.ObjectOld.GetLabels()[common.ArgoCDManagedByLabel]; ok && valOld != valNew {
					k8sClient, err := initK8sClient()
					if err != nil {
						return false
					}
					if err := deleteRBACsForNamespace(e.ObjectOld.GetName(), k8sClient); err != nil {
						log.Error(err, fmt.Sprintf("failed to delete RBACs for namespace: %s", e.ObjectOld.GetName()))
					} else {
						log.Info(fmt.Sprintf("Successfully removed the RBACs for namespace: %s", e.ObjectOld.GetName()))
					}

					// Delete namespace from cluster secret of previously managing argocd instance
					if err = deleteManagedNamespaceFromClusterSecret(valOld, e.ObjectOld.GetName(), k8sClient); err != nil {
						log.Error(err, fmt.Sprintf("unable to delete namespace %s from cluster secret", e.ObjectOld.GetName()))
					} else {
						log.Info(fmt.Sprintf("Successfully deleted namespace %s from cluster secret", e.ObjectOld.GetName()))
					}
				}
				return true
			}
			// This checks if the old meta had the label, if it did, delete the RBACs for the namespace
			// which were created when the label was added to the namespace.
			if ns, ok := e.ObjectOld.GetLabels()[common.ArgoCDManagedByLabel]; ok && ns != "" {
				k8sClient, err := initK8sClient()
				if err != nil {
					return false
				}
				if err := deleteRBACsForNamespace(e.ObjectOld.GetName(), k8sClient); err != nil {
					log.Error(err, fmt.Sprintf("failed to delete RBACs for namespace: %s", e.ObjectOld.GetName()))
				} else {
					log.Info(fmt.Sprintf("Successfully removed the RBACs for namespace: %s", e.ObjectOld.GetName()))
				}

				// Delete managed namespace from cluster secret
				if err = deleteManagedNamespaceFromClusterSecret(ns, e.ObjectOld.GetName(), k8sClient); err != nil {
					log.Error(err, fmt.Sprintf("unable to delete namespace %s from cluster secret", e.ObjectOld.GetName()))
				} else {
					log.Info(fmt.Sprintf("Successfully deleted namespace %s from cluster secret", e.ObjectOld.GetName()))
				}

			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if ns, ok := e.Object.GetLabels()[common.ArgoCDManagedByLabel]; ok && ns != "" {
				k8sClient, err := initK8sClient()

				if err != nil {
					return false
				}
				// Delete managed namespace from cluster secret
				err = deleteManagedNamespaceFromClusterSecret(ns, e.Object.GetName(), k8sClient)
				if err != nil {
					log.Error(err, fmt.Sprintf("unable to delete namespace %s from cluster secret", e.Object.GetName()))
				} else {
					log.Info(fmt.Sprintf("Successfully deleted namespace %s from cluster secret", e.Object.GetName()))
				}
			}

			// if a namespace is deleted, remove it from deprecationEventEmissionTracker (if exists) so that if a namespace with the same name
			// is created in the future and contains an Argo CD instance, it will be tracked appropriately
			delete(DeprecationEventEmissionTracker, e.Object.GetName())
			return false
		},
	}
}

// deleteRBACsForNamespace deletes the RBACs when the label from the namespace is removed.
func deleteRBACsForNamespace(sourceNS string, k8sClient kubernetes.Interface) error {
	log.Info(fmt.Sprintf("Removing the RBACs created for the namespace: %s", sourceNS))

	// List all the roles created for ArgoCD using the label selector
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{common.ArgoCDKeyPartOf: common.ArgoCDAppName}}
	roles, err := k8sClient.RbacV1().Roles(sourceNS).List(context.TODO(), metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to list roles for namespace: %s", sourceNS))
		return err
	}

	// Delete all the retrieved roles
	for _, role := range roles.Items {
		err = k8sClient.RbacV1().Roles(sourceNS).Delete(context.TODO(), role.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Error(err, fmt.Sprintf("failed to delete roles for namespace: %s", sourceNS))
		}
	}

	// List all the roles bindings created for ArgoCD using the label selector
	roleBindings, err := k8sClient.RbacV1().RoleBindings(sourceNS).List(context.TODO(), metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to list role bindings for namespace: %s", sourceNS))
		return err
	}

	// Delete all the retrieved role bindings
	for _, roleBinding := range roleBindings.Items {
		err = k8sClient.RbacV1().RoleBindings(sourceNS).Delete(context.TODO(), roleBinding.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Error(err, fmt.Sprintf("failed to delete role binding for namespace: %s", sourceNS))
		}
	}

	return nil
}

func deleteManagedNamespaceFromClusterSecret(ownerNS, sourceNS string, k8sClient kubernetes.Interface) error {

	// Get the cluster secret used for configuring ArgoCD
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{common.ArgoCDSecretTypeLabel: "cluster"}}
	secrets, err := k8sClient.CoreV1().Secrets(ownerNS).List(context.TODO(), metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to retrieve secrets for namespace: %s", ownerNS))
		return err
	}
	for _, secret := range secrets.Items {
		if string(secret.Data["server"]) != common.ArgoCDDefaultServer {
			continue
		}
		if namespaces, ok := secret.Data["namespaces"]; ok {
			namespaceList := strings.Split(string(namespaces), ",")
			var result []string

			for _, n := range namespaceList {
				// remove the namespace from the list of namespaces
				if strings.TrimSpace(n) == sourceNS {
					continue
				}
				result = append(result, strings.TrimSpace(n))
				sort.Strings(result)
				secret.Data["namespaces"] = []byte(strings.Join(result, ","))
			}
			// Update the secret with the updated list of namespaces
			if _, err = k8sClient.CoreV1().Secrets(ownerNS).Update(context.TODO(), &secret, metav1.UpdateOptions{}); err != nil {
				log.Error(err, fmt.Sprintf("failed to update cluster permission secret for namespace: %s", ownerNS))
				return err
			}
		}
	}
	return nil
}

// removeUnmanagedSourceNamespaceResources cleansup resources from SourceNamespaces if namespace is not managed by argocd instance.
// It also removes the managed-by-cluster-argocd label from the namespace
func (r *ReconcileArgoCD) removeUnmanagedSourceNamespaceResources(cr *argoproj.ArgoCD) error {

	for ns := range r.ManagedSourceNamespaces {
		managedNamespace := false
		if cr.GetDeletionTimestamp() == nil {
			for _, namespace := range cr.Spec.SourceNamespaces {
				if namespace == ns {
					managedNamespace = true
					break
				}
			}
		}

		if !managedNamespace {
			if err := r.cleanupUnmanagedSourceNamespaceResources(cr, ns); err != nil {
				log.Error(err, fmt.Sprintf("error cleaning up resources for namespace %s", ns))
				continue
			}
			delete(r.ManagedSourceNamespaces, ns)
		}
	}
	return nil
}

func (r *ReconcileArgoCD) cleanupUnmanagedSourceNamespaceResources(cr *argoproj.ArgoCD, ns string) error {
	namespace := corev1.Namespace{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: ns}, &namespace); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}
	// Remove managed-by-cluster-argocd from the namespace
	delete(namespace.Labels, common.ArgoCDManagedByClusterArgoCDLabel)
	if err := r.Client.Update(context.TODO(), &namespace); err != nil {
		log.Error(err, fmt.Sprintf("failed to remove label from namespace [%s]", namespace.Name))
	}

	// Delete Roles for SourceNamespaces
	existingRole := v1.Role{}
	roleName := getRoleNameForApplicationSourceNamespaces(namespace.Name, cr)
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleName, Namespace: namespace.Name}, &existingRole); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to fetch the role for the service account associated with %s : %s", common.ArgoCDServerComponent, err)
		}
	}
	if existingRole.Name != "" {
		if err := r.Client.Delete(context.TODO(), &existingRole); err != nil {
			return err
		}
	}
	// Delete RoleBindings for SourceNamespaces
	existingRoleBinding := &v1.RoleBinding{}
	roleBindingName := getRoleBindingNameForSourceNamespaces(cr.Name, namespace.Name)
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBindingName, Namespace: namespace.Name}, existingRoleBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", common.ArgoCDServerComponent, err)
		}
	}
	if existingRoleBinding.Name != "" {
		if err := r.Client.Delete(context.TODO(), existingRoleBinding); err != nil {
			return err
		}
	}
	return nil
}

// isSecretOfInterest returns true if the name of the given secret matches one of the
// well-known tls secrets used to secure communication amongst the Argo CD components.
func isSecretOfInterest(o client.Object) bool {
	if strings.HasSuffix(o.GetName(), "-repo-server-tls") {
		return true
	}
	if o.GetName() == common.ArgoCDRedisServerTLSSecretName {
		return true
	}
	return false
}

// isOwnerOfInterest returns true if the given owner is one of the Argo CD services that
// may have been made the owner of the tls secret created by the OpenShift service CA, used
// to secure communication amongst the Argo CD components.
func isOwnerOfInterest(owner metav1.OwnerReference) bool {
	if owner.Kind != "Service" {
		return false
	}
	if strings.HasSuffix(owner.Name, "-repo-server") {
		return true
	}
	if strings.HasSuffix(owner.Name, "-redis") {
		return true
	}
	return false
}

// namespaceResourceMapper maps a watch event on a namespace, back to the
// ArgoCD object that we want to reconcile.
func (r *ReconcileArgoCD) namespaceResourceMapper(ctx context.Context, o client.Object) []reconcile.Request {
	var result = []reconcile.Request{}

	labels := o.GetLabels()
	if v, ok := labels[common.ArgoCDManagedByLabel]; ok {
		argocds := &argoproj.ArgoCDList{}
		if err := r.Client.List(context.TODO(), argocds, &client.ListOptions{Namespace: v}); err != nil {
			return result
		}

		if len(argocds.Items) != 1 {
			return result
		}

		argocd := argocds.Items[0]
		namespacedName := client.ObjectKey{
			Name:      argocd.Name,
			Namespace: argocd.Namespace,
		}
		result = []reconcile.Request{
			{NamespacedName: namespacedName},
		}
	}

	return result
}

var (
	mutex sync.RWMutex
	hooks = []Hook{}
)

// Hook changes resources as they are created or updated by the reconciler.
type Hook func(*argoproj.ArgoCD, interface{}, string) error

// Register adds a modifier for updating resources during reconciliation.
func Register(h ...Hook) {
	mutex.Lock()
	defer mutex.Unlock()
	hooks = append(hooks, h...)
}

// nolint:unparam
func applyReconcilerHook(cr *argoproj.ArgoCD, i interface{}, hint string) error {
	mutex.Lock()
	defer mutex.Unlock()
	for _, v := range hooks {
		if err := v(cr, i, hint); err != nil {
			return err
		}
	}
	return nil
}

// newStatefulSet returns a new StatefulSet instance for the given ArgoCD instance.
func newStatefulSet(cr *argoproj.ArgoCD) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}
}

// newStatefulSetWithName returns a new StatefulSet instance for the given ArgoCD using the given name.
func newStatefulSetWithName(name string, component string, cr *argoproj.ArgoCD) *appsv1.StatefulSet {
	ss := newStatefulSet(cr)
	ss.ObjectMeta.Name = name

	lbls := ss.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyComponent] = component
	ss.ObjectMeta.Labels = lbls

	ss.Spec = appsv1.StatefulSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				common.ArgoCDKeyName: name,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					common.ArgoCDKeyName: name,
				},
			},
			Spec: corev1.PodSpec{
				NodeSelector: common.DefaultNodeSelector(),
			},
		},
	}
	if cr.Spec.NodePlacement != nil {
		ss.Spec.Template.Spec.NodeSelector = argoutil.AppendStringMap(ss.Spec.Template.Spec.NodeSelector, cr.Spec.NodePlacement.NodeSelector)
		ss.Spec.Template.Spec.Tolerations = cr.Spec.NodePlacement.Tolerations
	}
	ss.Spec.ServiceName = name

	return ss
}

// newStatefulSetWithSuffix returns a new StatefulSet instance for the given ArgoCD using the given suffix.
func newStatefulSetWithSuffix(suffix string, component string, cr *argoproj.ArgoCD) *appsv1.StatefulSet {
	return newStatefulSetWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), component, cr)
}

const (
	maxLabelLength    = 63
	maxHostnameLength = 253
	minFirstLabelSize = 20
)

// The algorithm used by this function is:
// - If the FIRST label ("console-openshift-console" in the above case) is longer than 63 characters, shorten (truncate the end) it to 63.
// - If any other label is longer than 63 characters, return an error
// - After all the labels are 63 characters or less, check the length of the overall hostname:
//   - If the overall hostname is > 253, then shorten the FIRST label until the host name is < 253
//   - After the FIRST label has been shortened, if it is < 20, then return an error (this is a sanity test to ensure the label is likely to be unique)
func shortenHostname(hostname string) (string, error) {
	if hostname == "" {
		return "", nil
	}

	// Return the hostname as it is if hostname is already within the size limit
	if len(hostname) <= maxHostnameLength {
		return hostname, nil
	}

	// Split the hostname into labels
	labels := strings.Split(hostname, ".")

	// Check and truncate the FIRST label if longer than 63 characters
	if len(labels[0]) > maxLabelLength {
		labels[0] = labels[0][:maxLabelLength]
	}

	// Check other labels and return an error if any is longer than 63 characters
	for _, label := range labels[1:] {
		if len(label) > maxLabelLength {
			return "", fmt.Errorf("label length exceeds 63 characters")
		}
	}

	// Join the labels back into a hostname
	resultHostname := strings.Join(labels, ".")

	// Check and shorten the overall hostname
	if len(resultHostname) > maxHostnameLength {
		// Shorten the first label until the length is less than 253
		for len(resultHostname) > maxHostnameLength && len(labels[0]) > 20 {
			labels[0] = labels[0][:len(labels[0])-1]
			resultHostname = strings.Join(labels, ".")
		}

		// Check if the first label is still less than 20 characters
		if len(labels[0]) < minFirstLabelSize {
			return "", fmt.Errorf("shortened first label is less than 20 characters")
		}
	}
	return resultHostname, nil
}

// newService returns a new Service for the given ArgoCD instance.
func newService(cr *argoproj.ArgoCD) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}
}

// newServiceWithName returns a new Service instance for the given ArgoCD using the given name.
func newServiceWithName(name string, component string, cr *argoproj.ArgoCD) *corev1.Service {
	svc := newService(cr)
	svc.ObjectMeta.Name = name

	lbls := svc.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyComponent] = component
	svc.ObjectMeta.Labels = lbls

	return svc
}

// newServiceWithSuffix returns a new Service instance for the given ArgoCD using the given suffix.
func newServiceWithSuffix(suffix string, component string, cr *argoproj.ArgoCD) *corev1.Service {
	return newServiceWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), component, cr)
}

// reconcileServices will ensure that all Services are present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileServices(cr *argoproj.ArgoCD) error {

	if err := r.reconcileDexService(cr); err != nil {
		log.Error(err, "error reconciling dex service")
	}

	err := r.reconcileGrafanaService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileMetricsService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRedisHAServices(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRedisService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRepoService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileServerMetricsService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileServerService(cr)
	if err != nil {
		return err
	}
	return nil
}

// reconcileCertificateAuthority will reconcile all Certificate Authority resources.
func (r *ReconcileArgoCD) reconcileCertificateAuthority(cr *argoproj.ArgoCD) error {
	log.Info("reconciling CA secret")
	if err := r.reconcileClusterCASecret(cr); err != nil {
		return err
	}

	log.Info("reconciling CA config map")
	if err := r.reconcileCAConfigMap(cr); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileArgoCD) clusterResourceMapper(ctx context.Context, o client.Object) []reconcile.Request {
	crbAnnotations := o.GetAnnotations()
	namespacedArgoCDObject := client.ObjectKey{}

	for k, v := range crbAnnotations {
		if k == common.AnnotationName {
			namespacedArgoCDObject.Name = v
		} else if k == common.AnnotationNamespace {
			namespacedArgoCDObject.Namespace = v
		}
	}

	var result = []reconcile.Request{}
	if namespacedArgoCDObject.Name != "" && namespacedArgoCDObject.Namespace != "" {
		result = []reconcile.Request{
			{NamespacedName: namespacedArgoCDObject},
		}
	}
	return result
}

// tlsSecretMapper maps a watch event on a secret of type TLS back to the
// ArgoCD object that we want to reconcile.
func (r *ReconcileArgoCD) tlsSecretMapper(ctx context.Context, o client.Object) []reconcile.Request {
	var result = []reconcile.Request{}

	if !isSecretOfInterest(o) {
		return result
	}
	namespacedArgoCDObject := client.ObjectKey{}

	secretOwnerRefs := o.GetOwnerReferences()
	if len(secretOwnerRefs) > 0 {
		// OpenShift service CA makes the owner reference for the TLS secret to the
		// service, which in turn is owned by the controller. This method performs
		// a lookup of the controller through the intermediate owning service.
		for _, secretOwner := range secretOwnerRefs {
			if isOwnerOfInterest(secretOwner) {
				key := client.ObjectKey{Name: secretOwner.Name, Namespace: o.GetNamespace()}
				svc := &corev1.Service{}

				// Get the owning object of the secret
				err := r.Client.Get(context.TODO(), key, svc)
				if err != nil {
					log.Error(err, fmt.Sprintf("could not get owner of secret %s", o.GetName()))
					return result
				}

				// If there's an object of kind ArgoCD in the owner's list,
				// this will be our reconciled object.
				serviceOwnerRefs := svc.GetOwnerReferences()
				for _, serviceOwner := range serviceOwnerRefs {
					if serviceOwner.Kind == "ArgoCD" {
						namespacedArgoCDObject.Name = serviceOwner.Name
						namespacedArgoCDObject.Namespace = svc.ObjectMeta.Namespace
						result = []reconcile.Request{
							{NamespacedName: namespacedArgoCDObject},
						}
						return result
					}
				}
			}
		}
	} else {
		// For secrets without owner (i.e. manually created), we apply some
		// heuristics. This may not be as accurate (e.g. if the user made a
		// typo in the resource's name), but should be good enough for now.
		secret, ok := o.(*corev1.Secret)
		if !ok {
			return result
		}
		if owner, ok := secret.Annotations[common.AnnotationName]; ok {
			namespacedArgoCDObject.Name = owner
			namespacedArgoCDObject.Namespace = o.GetNamespace()
			result = []reconcile.Request{
				{NamespacedName: namespacedArgoCDObject},
			}
		}
	}

	return result
}

// clusterSecretResourceMapper maps a watch event on a namespace, back to the
// ArgoCD object that we want to reconcile.
func (r *ReconcileArgoCD) clusterSecretResourceMapper(ctx context.Context, o client.Object) []reconcile.Request {
	var result = []reconcile.Request{}

	labels := o.GetLabels()
	if v, ok := labels[common.ArgoCDSecretTypeLabel]; ok && v == "cluster" {
		argocds := &argoproj.ArgoCDList{}
		if err := r.Client.List(context.TODO(), argocds, &client.ListOptions{Namespace: o.GetNamespace()}); err != nil {
			return result
		}

		if len(argocds.Items) != 1 {
			return result
		}

		argocd := argocds.Items[0]
		namespacedName := client.ObjectKey{
			Name:      argocd.Name,
			Namespace: argocd.Namespace,
		}
		result = []reconcile.Request{
			{NamespacedName: namespacedName},
		}
	}

	return result
}

// applicationSetSCMTLSConfigMapMapper maps a watch event on a configmap with name "argocd-appset-gitlab-scm-tls-certs-cm",
// back to the ArgoCD object that we want to reconcile.
func (r *ReconcileArgoCD) applicationSetSCMTLSConfigMapMapper(ctx context.Context, o client.Object) []reconcile.Request {
	var result = []reconcile.Request{}

	if o.GetName() == common.ArgoCDAppSetGitlabSCMTLSCertsConfigMapName {
		argocds := &argoproj.ArgoCDList{}
		if err := r.Client.List(context.TODO(), argocds, &client.ListOptions{Namespace: o.GetNamespace()}); err != nil {
			return result
		}

		if len(argocds.Items) != 1 {
			return result
		}

		argocd := argocds.Items[0]
		namespacedName := client.ObjectKey{
			Name:      argocd.Name,
			Namespace: argocd.Namespace,
		}
		result = []reconcile.Request{
			{NamespacedName: namespacedName},
		}
	}

	return result
}

func (r *ReconcileArgoCD) deleteClusterResources(cr *argoproj.ArgoCD) error {
	selector, err := argocdInstanceSelector(cr.Name)
	if err != nil {
		return err
	}

	clusterRoleList := &v1.ClusterRoleList{}
	if err := filterObjectsBySelector(r.Client, clusterRoleList, selector); err != nil {
		return fmt.Errorf("failed to filter ClusterRoles for %s: %w", cr.Name, err)
	}

	if err := deleteClusterRoles(r.Client, clusterRoleList); err != nil {
		return err
	}

	clusterBindingsList := &v1.ClusterRoleBindingList{}
	if err := filterObjectsBySelector(r.Client, clusterBindingsList, selector); err != nil {
		return fmt.Errorf("failed to filter ClusterRoleBindings for %s: %w", cr.Name, err)
	}

	if err := deleteClusterRoleBindings(r.Client, clusterBindingsList); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileArgoCD) removeManagedByLabelFromNamespaces(namespace string) error {
	nsList := &corev1.NamespaceList{}
	listOption := client.MatchingLabels{
		common.ArgoCDManagedByLabel: namespace,
	}
	if err := r.Client.List(context.TODO(), nsList, listOption); err != nil {
		return err
	}

	nsList.Items = append(nsList.Items, corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	for _, n := range nsList.Items {
		ns := &corev1.Namespace{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: n.Name}, ns); err != nil {
			return err
		}

		if ns.Labels == nil {
			continue
		}

		if n, ok := ns.Labels[common.ArgoCDManagedByLabel]; !ok || n != namespace {
			continue
		}
		delete(ns.Labels, common.ArgoCDManagedByLabel)
		if err := r.Client.Update(context.TODO(), ns); err != nil {
			log.Error(err, fmt.Sprintf("failed to remove label from namespace [%s]", ns.Name))
		}
	}
	return nil
}

func argocdInstanceSelector(name string) (labels.Selector, error) {
	selector := labels.NewSelector()
	requirement, err := labels.NewRequirement(common.ArgoCDKeyManagedBy, selection.Equals, []string{name})
	if err != nil {
		return nil, fmt.Errorf("failed to create a requirement for %w", err)
	}
	return selector.Add(*requirement), nil
}

func (r *ReconcileArgoCD) removeDeletionFinalizer(argocd *argoproj.ArgoCD) error {
	argocd.Finalizers = removeString(argocd.GetFinalizers(), common.ArgoCDDeletionFinalizer)
	if err := r.Client.Update(context.TODO(), argocd); err != nil {
		return fmt.Errorf("failed to remove deletion finalizer from %s: %w", argocd.Name, err)
	}
	return nil
}

// reconcileRoutes will ensure that all ArgoCD Routes are present.
func (r *ReconcileArgoCD) reconcileRoutes(cr *argoproj.ArgoCD) error {
	if err := r.reconcileGrafanaRoute(cr); err != nil {
		return err
	}

	if err := r.reconcilePrometheusRoute(cr); err != nil {
		return err
	}

	if err := r.reconcileServerRoute(cr); err != nil {
		return err
	}

	if err := r.reconcileApplicationSetControllerWebhookRoute(cr); err != nil {
		return err
	}

	return nil
}

// getArgoServerPath will return the Ingress Path for the Argo CD component.
func getPathOrDefault(path string) string {
	result := common.ArgoCDDefaultIngressPath
	if len(path) > 0 {
		result = path
	}
	return result
}

// newIngress returns a new Ingress instance for the given ArgoCD.
func newIngress(cr *argoproj.ArgoCD) *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}
}

// newIngressWithName returns a new Ingress with the given name and ArgoCD.
func newIngressWithName(name string, cr *argoproj.ArgoCD) *networkingv1.Ingress {
	ingress := newIngress(cr)
	ingress.ObjectMeta.Name = name

	lbls := ingress.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	ingress.ObjectMeta.Labels = lbls

	return ingress
}

// newIngressWithSuffix returns a new Ingress with the given name suffix for the ArgoCD.
func newIngressWithSuffix(suffix string, cr *argoproj.ArgoCD) *networkingv1.Ingress {
	return newIngressWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), cr)
}

// reconcileIngresses will ensure that all ArgoCD Ingress resources are present.
func (r *ReconcileArgoCD) reconcileIngresses(cr *argoproj.ArgoCD) error {
	if err := r.reconcileArgoServerIngress(cr); err != nil {
		return err
	}

	if err := r.reconcileArgoServerGRPCIngress(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaIngress(cr); err != nil {
		return err
	}

	if err := r.reconcilePrometheusIngress(cr); err != nil {
		return err
	}

	if err := r.reconcileApplicationSetControllerIngress(cr); err != nil {
		return err
	}

	return nil
}

func getPolicyRuleList(client client.Client) []struct {
	name       string
	policyRule []v1.PolicyRule
} {
	return []struct {
		name       string
		policyRule []v1.PolicyRule
	}{
		{
			name:       common.ArgoCDApplicationControllerComponent,
			policyRule: policyRuleForApplicationController(),
		}, {
			name:       common.ArgoCDDexServerComponent,
			policyRule: policyRuleForDexServer(),
		}, {
			name:       common.ArgoCDServerComponent,
			policyRule: policyRuleForServer(),
		}, {
			name:       common.ArgoCDRedisHAComponent,
			policyRule: policyRuleForRedisHa(client),
		}, {
			name:       common.ArgoCDRedisComponent,
			policyRule: policyRuleForRedis(client),
		},
	}
}

func getPolicyRuleClusterRoleList() []struct {
	name       string
	policyRule []v1.PolicyRule
} {
	return []struct {
		name       string
		policyRule []v1.PolicyRule
	}{
		{
			name:       common.ArgoCDApplicationControllerComponent,
			policyRule: policyRuleForApplicationController(),
		}, {
			name:       common.ArgoCDServerComponent,
			policyRule: policyRuleForServerClusterRole(),
		},
	}
}

// newRole returns a new Role instance.
func newRole(name string, rules []v1.PolicyRule, cr *argoproj.ArgoCD) *v1.Role {
	return &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateResourceName(name, cr),
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
		Rules: rules,
	}
}

func newRoleForApplicationSourceNamespaces(namespace string, rules []v1.PolicyRule, cr *argoproj.ArgoCD) *v1.Role {
	return &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getRoleNameForApplicationSourceNamespaces(namespace, cr),
			Namespace: namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
		Rules: rules,
	}
}

func generateResourceName(argoComponentName string, cr *argoproj.ArgoCD) string {
	return cr.Name + "-" + argoComponentName
}

// GenerateUniqueResourceName generates unique names for cluster scoped resources
func GenerateUniqueResourceName(argoComponentName string, cr *argoproj.ArgoCD) string {
	return cr.Name + "-" + cr.Namespace + "-" + argoComponentName
}

func newClusterRole(name string, rules []v1.PolicyRule, cr *argoproj.ArgoCD) *v1.ClusterRole {
	return &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:        GenerateUniqueResourceName(name, cr),
			Labels:      argoutil.LabelsForCluster(cr),
			Annotations: argoutil.AnnotationsForCluster(cr),
		},
		Rules: rules,
	}
}

// reconcileRoles will ensure that all ArgoCD Service Accounts are configured.
func (r *ReconcileArgoCD) reconcileRoles(cr *argoproj.ArgoCD) error {
	params := getPolicyRuleList(r.Client)

	for _, param := range params {
		if _, err := r.reconcileRole(param.name, param.policyRule, cr); err != nil {
			return err
		}
	}

	clusterParams := getPolicyRuleClusterRoleList()

	for _, clusterParam := range clusterParams {
		if _, err := r.reconcileClusterRole(clusterParam.name, clusterParam.policyRule, cr); err != nil {
			return err
		}
	}

	log.Info("reconciling roles for source namespaces")
	policyRuleForApplicationSourceNamespaces := policyRuleForServerApplicationSourceNamespaces()
	// reconcile roles is source namespaces for ArgoCD Server
	if err := r.reconcileRoleForApplicationSourceNamespaces(common.ArgoCDServerComponent, policyRuleForApplicationSourceNamespaces, cr); err != nil {
		return err
	}

	log.Info("performing cleanup for source namespaces")
	// remove resources for namespaces not part of SourceNamespaces
	if err := r.removeUnmanagedSourceNamespaceResources(cr); err != nil {
		return err
	}

	return nil
}

// reconcileRole, reconciles the policy rules for different ArgoCD components, for each namespace
// Managed by a single instance of ArgoCD.
func (r *ReconcileArgoCD) reconcileRole(name string, policyRules []v1.PolicyRule, cr *argoproj.ArgoCD) ([]*v1.Role, error) {
	var roles []*v1.Role

	// create policy rules for each namespace
	for _, namespace := range r.ManagedNamespaces.Items {
		// If encountering a terminating namespace remove managed-by label from it and skip reconciliation - This should trigger
		// clean-up of roles/rolebindings and removal of namespace from cluster secret
		if namespace.DeletionTimestamp != nil {
			if _, ok := namespace.Labels[common.ArgoCDManagedByLabel]; ok {
				delete(namespace.Labels, common.ArgoCDManagedByLabel)
				_ = r.Client.Update(context.TODO(), &namespace)
			}
			continue
		}

		list := &argoproj.ArgoCDList{}
		listOption := &client.ListOptions{Namespace: namespace.Name}
		err := r.Client.List(context.TODO(), list, listOption)
		if err != nil {
			return nil, err
		}
		// only skip creation of dex and redisHa roles for namespaces that no argocd instance is deployed in
		if len(list.Items) < 1 {
			// namespace doesn't contain argocd instance, so skipe all the ArgoCD internal roles
			if cr.ObjectMeta.Namespace != namespace.Name && (name != common.ArgoCDApplicationControllerComponent && name != common.ArgoCDServerComponent) {
				continue
			}
		}
		customRole := getCustomRoleName(name)
		role := newRole(name, policyRules, cr)
		if err := applyReconcilerHook(cr, role, ""); err != nil {
			return nil, err
		}
		role.Namespace = namespace.Name
		existingRole := v1.Role{}
		err = r.Client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, &existingRole)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("failed to reconcile the role for the service account associated with %s : %s", name, err)
			}
			if customRole != "" {
				continue // skip creating default role if custom cluster role is provided
			}
			roles = append(roles, role)

			if name == common.ArgoCDDexServerComponent && !UseDex(cr) {

				continue // Dex installation not requested, do nothing
			}

			// Only set ownerReferences for roles in same namespace as ArgoCD CR
			if cr.Namespace == role.Namespace {
				if err = controllerutil.SetControllerReference(cr, role, r.Scheme); err != nil {
					return nil, fmt.Errorf("failed to set ArgoCD CR \"%s\" as owner for role \"%s\": %s", cr.Name, role.Name, err)
				}
			}

			log.Info(fmt.Sprintf("creating role %s for Argo CD instance %s in namespace %s", role.Name, cr.Name, cr.Namespace))
			if err := r.Client.Create(context.TODO(), role); err != nil {
				return nil, err
			}
			continue
		}

		// Delete the existing default role if custom role is specified
		// or if there is an existing Role created for Dex but dex is disabled or not configured
		if customRole != "" ||
			(name == common.ArgoCDDexServerComponent && !UseDex(cr)) {

			log.Info("deleting the existing Dex role because dex is not configured")
			if err := r.Client.Delete(context.TODO(), &existingRole); err != nil {
				return nil, err
			}
			continue
		}

		// if the Rules differ, update the Role
		if !reflect.DeepEqual(existingRole.Rules, role.Rules) {
			existingRole.Rules = role.Rules
			if err := r.Client.Update(context.TODO(), &existingRole); err != nil {
				return nil, err
			}
		}
		roles = append(roles, &existingRole)
	}
	return roles, nil
}

func (r *ReconcileArgoCD) reconcileRoleForApplicationSourceNamespaces(name string, policyRules []v1.PolicyRule, cr *argoproj.ArgoCD) error {

	// create policy rules for each source namespace for ArgoCD Server
	for _, sourceNamespace := range cr.Spec.SourceNamespaces {

		namespace := &corev1.Namespace{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: sourceNamespace}, namespace); err != nil {
			return err
		}

		// do not reconcile roles for namespaces already containing managed-by label
		// as it already contains roles with permissions to manipulate application resources
		// reconciled during reconcilation of ManagedNamespaces
		if value, ok := namespace.Labels[common.ArgoCDManagedByLabel]; ok && value != "" {
			log.Info(fmt.Sprintf("Skipping reconciling resources for namespace %s as it is already managed-by namespace %s.", namespace.Name, value))
			// if managed-by-cluster-argocd label is also present, remove the namespace from the ManagedSourceNamespaces.
			if val, ok1 := namespace.Labels[common.ArgoCDManagedByClusterArgoCDLabel]; ok1 && val == cr.Namespace {
				delete(r.ManagedSourceNamespaces, namespace.Name)
				if err := r.cleanupUnmanagedSourceNamespaceResources(cr, namespace.Name); err != nil {
					log.Error(err, fmt.Sprintf("error cleaning up resources for namespace %s", namespace.Name))
				}
			}
			continue
		}

		log.Info(fmt.Sprintf("Reconciling role for %s", namespace.Name))

		role := newRoleForApplicationSourceNamespaces(namespace.Name, policyRules, cr)
		if err := applyReconcilerHook(cr, role, ""); err != nil {
			return err
		}
		role.Namespace = namespace.Name
		existingRole := v1.Role{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: namespace.Name}, &existingRole)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to reconcile the role for the service account associated with %s : %s", name, err)
			}
		}

		// do not reconcile roles for namespaces already containing managed-by-cluster-argocd label
		// as it already contains roles reconciled during reconcilation of ManagedNamespaces
		if _, ok := r.ManagedSourceNamespaces[sourceNamespace]; ok {
			// If sourceNamespace includes the name but role is missing in the namespace, create the role
			if reflect.DeepEqual(existingRole, v1.Role{}) {
				log.Info(fmt.Sprintf("creating role %s for Argo CD instance %s in namespace %s", role.Name, cr.Name, namespace))
				if err := r.Client.Create(context.TODO(), role); err != nil {
					return err
				}
			}
			continue
		}

		// reconcile roles only if another ArgoCD instance is not already set as value for managed-by-cluster-argocd label
		if value, ok := namespace.Labels[common.ArgoCDManagedByClusterArgoCDLabel]; ok && value != "" {
			log.Info(fmt.Sprintf("Namespace already has label set to argocd instance %s. Thus, skipping namespace %s", value, namespace.Name))
			continue
		}

		// Get the latest value of namespace before updating it
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: namespace.Name}, namespace); err != nil {
			return err
		}
		// Update namespace with managed-by-cluster-argocd label
		namespace.Labels[common.ArgoCDManagedByClusterArgoCDLabel] = cr.Namespace
		if err := r.Client.Update(context.TODO(), namespace); err != nil {
			log.Error(err, fmt.Sprintf("failed to add label from namespace [%s]", namespace.Name))
		}
		// if the Rules differ, update the Role
		if !reflect.DeepEqual(existingRole.Rules, role.Rules) {
			existingRole.Rules = role.Rules
			if err := r.Client.Update(context.TODO(), &existingRole); err != nil {
				return err
			}
		}

		if _, ok := r.ManagedSourceNamespaces[sourceNamespace]; !ok {
			r.ManagedSourceNamespaces[sourceNamespace] = ""
		}

	}
	return nil
}

func (r *ReconcileArgoCD) reconcileClusterRole(name string, policyRules []v1.PolicyRule, cr *argoproj.ArgoCD) (*v1.ClusterRole, error) {
	allowed := false
	if allowedNamespace(cr.Namespace, os.Getenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")) {
		allowed = true
	}
	clusterRole := newClusterRole(name, policyRules, cr)
	if err := applyReconcilerHook(cr, clusterRole, ""); err != nil {
		return nil, err
	}

	existingClusterRole := &v1.ClusterRole{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRole.Name}, existingClusterRole)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the cluster role for the service account associated with %s : %s", name, err)
		}
		if !allowed {
			// Do Nothing
			return nil, nil
		}
		return clusterRole, r.Client.Create(context.TODO(), clusterRole)
	}

	if !allowed {
		return nil, r.Client.Delete(context.TODO(), existingClusterRole)
	}

	// if the Rules differ, update the Role
	if !reflect.DeepEqual(existingClusterRole.Rules, clusterRole.Rules) {
		existingClusterRole.Rules = clusterRole.Rules
		if err := r.Client.Update(context.TODO(), existingClusterRole); err != nil {
			return nil, err
		}
	}
	return existingClusterRole, nil
}

func deleteClusterRoles(c client.Client, clusterRoleList *v1.ClusterRoleList) error {
	for _, clusterRole := range clusterRoleList.Items {
		if err := c.Delete(context.TODO(), &clusterRole); err != nil {
			return fmt.Errorf("failed to delete ClusterRole %q during cleanup: %w", clusterRole.Name, err)
		}
	}
	return nil
}

// newClusterRoleBinding returns a new ClusterRoleBinding instance.
func newClusterRoleBinding(cr *argoproj.ArgoCD) *v1.ClusterRoleBinding {
	return &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cr.Name,
			Labels:      argoutil.LabelsForCluster(cr),
			Annotations: argoutil.AnnotationsForCluster(cr),
		},
	}
}

// newClusterRoleBindingWithname creates a new ClusterRoleBinding with the given name for the given ArgCD.
func newClusterRoleBindingWithname(name string, cr *argoproj.ArgoCD) *v1.ClusterRoleBinding {
	roleBinding := newClusterRoleBinding(cr)
	roleBinding.Name = GenerateUniqueResourceName(name, cr)

	labels := roleBinding.ObjectMeta.Labels
	labels[common.ArgoCDKeyName] = name
	roleBinding.ObjectMeta.Labels = labels

	return roleBinding
}

// newRoleBinding returns a new RoleBinding instance.
func newRoleBinding(cr *argoproj.ArgoCD) *v1.RoleBinding {
	return &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cr.Name,
			Labels:      argoutil.LabelsForCluster(cr),
			Annotations: argoutil.AnnotationsForCluster(cr),
			Namespace:   cr.Namespace,
		},
	}
}

// newRoleBindingForSupportNamespaces returns a new RoleBinding instance.
func newRoleBindingForSupportNamespaces(cr *argoproj.ArgoCD, namespace string) *v1.RoleBinding {
	return &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        getRoleBindingNameForSourceNamespaces(cr.Name, namespace),
			Labels:      argoutil.LabelsForCluster(cr),
			Annotations: argoutil.AnnotationsForCluster(cr),
			Namespace:   namespace,
		},
	}
}

func getRoleBindingNameForSourceNamespaces(argocdName, targetNamespace string) string {
	return fmt.Sprintf("%s_%s", argocdName, targetNamespace)
}

// newRoleBindingWithname creates a new RoleBinding with the given name for the given ArgCD.
func newRoleBindingWithname(name string, cr *argoproj.ArgoCD) *v1.RoleBinding {
	roleBinding := newRoleBinding(cr)
	roleBinding.ObjectMeta.Name = fmt.Sprintf("%s-%s", cr.Name, name)

	labels := roleBinding.ObjectMeta.Labels
	labels[common.ArgoCDKeyName] = name
	roleBinding.ObjectMeta.Labels = labels

	return roleBinding
}

// reconcileRoleBindings will ensure that all ArgoCD RoleBindings are configured.
func (r *ReconcileArgoCD) reconcileRoleBindings(cr *argoproj.ArgoCD) error {
	params := getPolicyRuleList(r.Client)

	for _, param := range params {
		if err := r.reconcileRoleBinding(param.name, param.policyRule, cr); err != nil {
			return fmt.Errorf("error reconciling roleBinding for %q: %w", param.name, err)
		}
	}

	return nil
}

// reconcileRoleBinding, creates RoleBindings for every role and associates it with the right ServiceAccount.
// This would create RoleBindings for all the namespaces managed by the ArgoCD instance.
func (r *ReconcileArgoCD) reconcileRoleBinding(name string, rules []v1.PolicyRule, cr *argoproj.ArgoCD) error {
	var sa *corev1.ServiceAccount
	var error error

	if sa, error = r.reconcileServiceAccount(name, cr); error != nil {
		return error
	}

	if _, error = r.reconcileRole(name, rules, cr); error != nil {
		return error
	}

	for _, namespace := range r.ManagedNamespaces.Items {
		// If encountering a terminating namespace remove managed-by label from it and skip reconciliation - This should trigger
		// clean-up of roles/rolebindings and removal of namespace from cluster secret
		if namespace.DeletionTimestamp != nil {
			if _, ok := namespace.Labels[common.ArgoCDManagedByLabel]; ok {
				delete(namespace.Labels, common.ArgoCDManagedByLabel)
				_ = r.Client.Update(context.TODO(), &namespace)
			}
			continue
		}

		list := &argoproj.ArgoCDList{}
		listOption := &client.ListOptions{Namespace: namespace.Name}
		err := r.Client.List(context.TODO(), list, listOption)
		if err != nil {
			return err
		}
		// only skip creation of dex and redisHa rolebindings for namespaces that no argocd instance is deployed in
		if len(list.Items) < 1 {
			// namespace doesn't contain argocd instance, so skipe all the ArgoCD internal roles
			if cr.ObjectMeta.Namespace != namespace.Name && (name != common.ArgoCDApplicationControllerComponent && name != common.ArgoCDServerComponent) {
				continue
			}
		}
		// get expected name
		roleBinding := newRoleBindingWithname(name, cr)
		roleBinding.Namespace = namespace.Name

		// fetch existing rolebinding by name
		existingRoleBinding := &v1.RoleBinding{}
		err = r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: roleBinding.Namespace}, existingRoleBinding)
		roleBindingExists := true
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to get the rolebinding associated with %s : %s", name, err)
			}

			if name == common.ArgoCDDexServerComponent && !UseDex(cr) {
				continue // Dex installation is not requested, do nothing
			}

			roleBindingExists = false
		}

		roleBinding.Subjects = []v1.Subject{
			{
				Kind:      v1.ServiceAccountKind,
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		}

		customRoleName := getCustomRoleName(name)
		if customRoleName != "" {
			roleBinding.RoleRef = v1.RoleRef{
				APIGroup: v1.GroupName,
				Kind:     "ClusterRole",
				Name:     customRoleName,
			}
		} else {
			roleBinding.RoleRef = v1.RoleRef{
				APIGroup: v1.GroupName,
				Kind:     "Role",
				Name:     generateResourceName(name, cr),
			}
		}

		if roleBindingExists {
			if name == common.ArgoCDDexServerComponent && !UseDex(cr) {
				// Delete any existing RoleBinding created for Dex since dex uninstallation is requested
				log.Info("deleting the existing Dex roleBinding because dex uninstallation is requested")
				if err = r.Client.Delete(context.TODO(), existingRoleBinding); err != nil {
					return err
				}
				continue
			}

			// if the RoleRef changes, delete the existing role binding and create a new one
			if !reflect.DeepEqual(roleBinding.RoleRef, existingRoleBinding.RoleRef) {
				if err = r.Client.Delete(context.TODO(), existingRoleBinding); err != nil {
					return err
				}
			} else {
				// if the Subjects differ, update the role bindings
				if !reflect.DeepEqual(roleBinding.Subjects, existingRoleBinding.Subjects) {
					existingRoleBinding.Subjects = roleBinding.Subjects
					if err = r.Client.Update(context.TODO(), existingRoleBinding); err != nil {
						return err
					}
				}
				continue
			}
		}

		// Only set ownerReferences for role bindings in same namespaces as Argo CD CR
		if cr.Namespace == roleBinding.Namespace {
			if err = controllerutil.SetControllerReference(cr, roleBinding, r.Scheme); err != nil {
				return fmt.Errorf("failed to set ArgoCD CR \"%s\" as owner for roleBinding \"%s\": %s", cr.Name, roleBinding.Name, err)
			}
		}

		log.Info(fmt.Sprintf("creating rolebinding %s for Argo CD instance %s in namespace %s", roleBinding.Name, cr.Name, cr.Namespace))
		if err = r.Client.Create(context.TODO(), roleBinding); err != nil {
			return err
		}
	}

	// reconcile rolebindings only for ArgoCDServerComponent
	if name == common.ArgoCDServerComponent {

		// reconcile rolebindings for all source namespaces for argocd-server
		for _, sourceNamespace := range cr.Spec.SourceNamespaces {
			namespace := &corev1.Namespace{}
			if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: sourceNamespace}, namespace); err != nil {
				return err
			}

			// do not reconcile rolebindings for namespaces already containing managed-by label
			// as it already contains rolebindings with permissions to manipulate application resources
			// reconciled during reconcilation of ManagedNamespaces
			if value, ok := namespace.Labels[common.ArgoCDManagedByLabel]; ok {
				log.Info(fmt.Sprintf("Skipping reconciling resources for namespace %s as it is already managed-by namespace %s.", namespace.Name, value))
				continue
			}

			list := &argoproj.ArgoCDList{}
			listOption := &client.ListOptions{Namespace: namespace.Name}
			err := r.Client.List(context.TODO(), list, listOption)
			if err != nil {
				log.Info(err.Error())
				return err
			}

			// get expected name
			roleBinding := newRoleBindingWithNameForApplicationSourceNamespaces(namespace.Name, cr)
			roleBinding.Namespace = namespace.Name

			roleBinding.RoleRef = v1.RoleRef{
				APIGroup: v1.GroupName,
				Kind:     "Role",
				Name:     getRoleNameForApplicationSourceNamespaces(namespace.Name, cr),
			}

			// fetch existing rolebinding by name
			existingRoleBinding := &v1.RoleBinding{}
			err = r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: roleBinding.Namespace}, existingRoleBinding)
			roleBindingExists := true
			if err != nil {
				if !apierrors.IsNotFound(err) {
					return fmt.Errorf("failed to get the rolebinding associated with %s : %s", name, err)
				}
				log.Info(fmt.Sprintf("Existing rolebinding %s", err.Error()))
				roleBindingExists = false
			}

			roleBinding.Subjects = []v1.Subject{
				{
					Kind:      v1.ServiceAccountKind,
					Name:      getServiceAccountName(cr.Name, common.ArgoCDServerComponent),
					Namespace: sa.Namespace,
				},
				{
					Kind:      v1.ServiceAccountKind,
					Name:      getServiceAccountName(cr.Name, common.ArgoCDApplicationControllerComponent),
					Namespace: sa.Namespace,
				},
			}

			if roleBindingExists {
				// reconcile role bindings for namespaces already containing managed-by-cluster-argocd label only
				if n, ok := namespace.Labels[common.ArgoCDManagedByClusterArgoCDLabel]; !ok || n == cr.Namespace {
					continue
				}
				// if the RoleRef changes, delete the existing role binding and create a new one
				if !reflect.DeepEqual(roleBinding.RoleRef, existingRoleBinding.RoleRef) {
					if err = r.Client.Delete(context.TODO(), existingRoleBinding); err != nil {
						return err
					}
				} else {
					// if the Subjects differ, update the role bindings
					if !reflect.DeepEqual(roleBinding.Subjects, existingRoleBinding.Subjects) {
						existingRoleBinding.Subjects = roleBinding.Subjects
						if err = r.Client.Update(context.TODO(), existingRoleBinding); err != nil {
							return err
						}
					}
					continue
				}
			}

			log.Info(fmt.Sprintf("creating rolebinding %s for Argo CD instance %s in namespace %s", roleBinding.Name, cr.Name, namespace))
			if err = r.Client.Create(context.TODO(), roleBinding); err != nil {
				return err
			}
		}
	}
	return nil
}

func getCustomRoleName(name string) string {
	if name == common.ArgoCDApplicationControllerComponent {
		return os.Getenv(common.ArgoCDControllerClusterRoleEnvName)
	}
	if name == common.ArgoCDServerComponent {
		return os.Getenv(server.ArgoCDServerClusterRoleEnvVar)
	}
	return ""
}

// Returns the name of the role for the source namespaces for ArgoCDServer in the format of "sourceNamespace_targetNamespace_argocd-server"
func getRoleNameForApplicationSourceNamespaces(targetNamespace string, cr *argoproj.ArgoCD) string {
	return fmt.Sprintf("%s_%s", cr.Name, targetNamespace)
}

// newRoleBindingWithNameForApplicationSourceNamespaces creates a new RoleBinding with the given name for the source namespaces of ArgoCD Server.
func newRoleBindingWithNameForApplicationSourceNamespaces(namespace string, cr *argoproj.ArgoCD) *v1.RoleBinding {
	roleBinding := newRoleBindingForSupportNamespaces(cr, namespace)

	labels := roleBinding.ObjectMeta.Labels
	labels[common.ArgoCDKeyName] = roleBinding.ObjectMeta.Name
	roleBinding.ObjectMeta.Labels = labels

	return roleBinding
}

func (r *ReconcileArgoCD) reconcileClusterRoleBinding(name string, role *v1.ClusterRole, cr *argoproj.ArgoCD) error {

	// get expected name
	roleBinding := newClusterRoleBindingWithname(name, cr)
	// fetch existing rolebinding by name
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name}, roleBinding)
	roleBindingExists := true
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		roleBindingExists = false
		roleBinding = newClusterRoleBindingWithname(name, cr)
	}

	if roleBindingExists && role == nil {
		return r.Client.Delete(context.TODO(), roleBinding)
	}

	if !roleBindingExists && role == nil {
		// DO Nothing
		return nil
	}

	roleBinding.Subjects = []v1.Subject{
		{
			Kind:      v1.ServiceAccountKind,
			Name:      generateResourceName(name, cr),
			Namespace: cr.Namespace,
		},
	}
	roleBinding.RoleRef = v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     "ClusterRole",
		Name:     GenerateUniqueResourceName(name, cr),
	}

	if cr.Namespace == roleBinding.Namespace {
		if err = controllerutil.SetControllerReference(cr, roleBinding, r.Scheme); err != nil {
			return fmt.Errorf("failed to set ArgoCD CR \"%s\" as owner for roleBinding \"%s\": %s", cr.Name, roleBinding.Name, err)
		}
	}

	if roleBindingExists {
		return r.Client.Update(context.TODO(), roleBinding)
	}
	return r.Client.Create(context.TODO(), roleBinding)
}

func deleteClusterRoleBindings(c client.Client, clusterBindingList *v1.ClusterRoleBindingList) error {
	for _, clusterBinding := range clusterBindingList.Items {
		if err := c.Delete(context.TODO(), &clusterBinding); err != nil {
			return fmt.Errorf("failed to delete ClusterRoleBinding %q during cleanup: %w", clusterBinding.Name, err)
		}
	}
	return nil
}

// newServiceAccount returns a new ServiceAccount instance.
func newServiceAccount(cr *argoproj.ArgoCD) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}
}

// newServiceAccountWithName creates a new ServiceAccount with the given name for the given ArgCD.
func newServiceAccountWithName(name string, cr *argoproj.ArgoCD) *corev1.ServiceAccount {
	sa := newServiceAccount(cr)
	sa.ObjectMeta.Name = getServiceAccountName(cr.Name, name)

	lbls := sa.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	sa.ObjectMeta.Labels = lbls

	return sa
}

func getServiceAccountName(crName, name string) string {
	return fmt.Sprintf("%s-%s", crName, name)
}

// reconcileServiceAccounts will ensure that all ArgoCD Service Accounts are configured.
func (r *ReconcileArgoCD) reconcileServiceAccounts(cr *argoproj.ArgoCD) error {
	params := getPolicyRuleList(r.Client)

	for _, param := range params {
		if err := r.reconcileServiceAccountPermissions(param.name, param.policyRule, cr); err != nil {
			return err
		}
	}

	clusterParams := getPolicyRuleClusterRoleList()

	for _, clusterParam := range clusterParams {
		if err := r.reconcileServiceAccountClusterPermissions(clusterParam.name, clusterParam.policyRule, cr); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileArgoCD) reconcileServiceAccountClusterPermissions(name string, rules []v1.PolicyRule, cr *argoproj.ArgoCD) error {
	var role *v1.ClusterRole
	var err error

	_, err = r.reconcileServiceAccount(name, cr)
	if err != nil {
		return err
	}

	if role, err = r.reconcileClusterRole(name, rules, cr); err != nil {
		return err
	}

	return r.reconcileClusterRoleBinding(name, role, cr)
}

func (r *ReconcileArgoCD) reconcileServiceAccountPermissions(name string, rules []v1.PolicyRule, cr *argoproj.ArgoCD) error {
	return r.reconcileRoleBinding(name, rules, cr)
}

func (r *ReconcileArgoCD) reconcileServiceAccount(name string, cr *argoproj.ArgoCD) (*corev1.ServiceAccount, error) {
	sa := newServiceAccountWithName(name, cr)

	exists := true
	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}

		if name == common.ArgoCDDexServerComponent && !UseDex(cr) {
			return sa, nil // Dex installation not requested, do nothing
		}
		exists = false
	}
	if exists {
		if name == common.ArgoCDDexServerComponent && !UseDex(cr) {
			// Delete any existing Service Account created for Dex since dex is disabled
			log.Info("deleting the existing Dex service account because dex uninstallation requested")
			return sa, r.Client.Delete(context.TODO(), sa)
		}
		return sa, nil
	}

	if err := controllerutil.SetControllerReference(cr, sa, r.Scheme); err != nil {
		return nil, err
	}

	log.Info(fmt.Sprintf("creating serviceaccount %s for Argo CD instance %s in namespace %s", sa.Name, cr.Name, cr.Namespace))

	err := r.Client.Create(context.TODO(), sa)
	if err != nil {
		return nil, err
	}

	return sa, nil
}
