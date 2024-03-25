package argocd

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	argopass "github.com/argoproj/argo-cd/v2/util/password"
	"github.com/prometheus/client_golang/prometheus"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"

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
	v1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
