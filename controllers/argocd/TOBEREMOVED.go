package argocd

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"

	argopass "github.com/argoproj/argo-cd/v2/util/password"

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
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

// getNotificationsResources will return the ResourceRequirements for the Notifications container.
func getNotificationsResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Notifications.Resources != nil {
		resources = *cr.Spec.Notifications.Resources
	}

	return resources
}

func getNotificationsCommand(cr *argoproj.ArgoCD) []string {

	cmd := make([]string, 0)
	cmd = append(cmd, "argocd-notifications")

	cmd = append(cmd, "--loglevel")
	cmd = append(cmd, getLogLevel(cr.Spec.Notifications.LogLevel))

	return cmd
}

// reconcileNotificationsConfigMap only creates/deletes the argocd-notifications-cm based on whether notifications is enabled/disabled in the CR
// It does not reconcile/overwrite any fields or information in the configmap itself
func (r *ReconcileArgoCD) reconcileNotificationsConfigMap(cr *argoproj.ArgoCD) error {

	desiredConfigMap := newConfigMapWithName("argocd-notifications-cm", cr)
	desiredConfigMap.Data = getDefaultNotificationsConfig()

	cmExists := true
	existingConfigMap := &corev1.ConfigMap{}
	if err := argoutil.FetchObject(r.Client, cr.Namespace, desiredConfigMap.Name, existingConfigMap); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the configmap associated with %s : %s", desiredConfigMap.Name, err)
		}
		cmExists = false
	}

	if cmExists {
		// CM exists but shouldn't, so it should be deleted
		if !cr.Spec.Notifications.Enabled {
			log.Info(fmt.Sprintf("Deleting configmap %s as notifications is disabled", existingConfigMap.Name))
			return r.Client.Delete(context.TODO(), existingConfigMap)
		}

		// CM exists and should, nothing to do here
		return nil
	}

	// CM doesn't exist and shouldn't, nothing to do here
	if !cr.Spec.Notifications.Enabled {
		return nil
	}

	// CM doesn't exist but should, so it should be created
	if err := controllerutil.SetControllerReference(cr, desiredConfigMap, r.Scheme); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Creating configmap %s", desiredConfigMap.Name))
	err := r.Client.Create(context.TODO(), desiredConfigMap)
	if err != nil {
		return err
	}

	return nil
}

// reconcileNotificationsSecret only creates/deletes the argocd-notifications-secret based on whether notifications is enabled/disabled in the CR
// It does not reconcile/overwrite any fields or information in the secret itself
func (r *ReconcileArgoCD) reconcileNotificationsSecret(cr *argoproj.ArgoCD) error {

	desiredSecret := argoutil.NewSecretWithName(cr, "argocd-notifications-secret")

	secretExists := true
	existingSecret := &corev1.Secret{}
	if err := argoutil.FetchObject(r.Client, cr.Namespace, desiredSecret.Name, existingSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the secret associated with %s : %s", desiredSecret.Name, err)
		}
		secretExists = false
	}

	if secretExists {
		// secret exists but shouldn't, so it should be deleted
		if !cr.Spec.Notifications.Enabled {
			log.Info(fmt.Sprintf("Deleting secret %s as notifications is disabled", existingSecret.Name))
			return r.Client.Delete(context.TODO(), existingSecret)
		}

		// secret exists and should, nothing to do here
		return nil
	}

	// secret doesn't exist and shouldn't, nothing to do here
	if !cr.Spec.Notifications.Enabled {
		return nil
	}

	// secret doesn't exist but should, so it should be created
	if err := controllerutil.SetControllerReference(cr, desiredSecret, r.Scheme); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Creating secret %s", desiredSecret.Name))
	err := r.Client.Create(context.TODO(), desiredSecret)
	if err != nil {
		return err
	}

	return nil
}

func (r *ReconcileArgoCD) reconcileNotificationsController(cr *argoproj.ArgoCD) error {

	log.Info("reconciling notifications serviceaccount")
	sa, err := r.reconcileNotificationsServiceAccount(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling notifications role")
	role, err := r.reconcileNotificationsRole(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling notifications role binding")
	if err := r.reconcileNotificationsRoleBinding(cr, role, sa); err != nil {
		return err
	}

	log.Info("reconciling notifications configmap")
	if err := r.reconcileNotificationsConfigMap(cr); err != nil {
		return err
	}

	log.Info("reconciling notifications secret")
	if err := r.reconcileNotificationsSecret(cr); err != nil {
		return err
	}

	log.Info("reconciling notifications deployment")
	if err := r.reconcileNotificationsDeployment(cr, sa); err != nil {
		return err
	}

	return nil
}

// The code to create/delete notifications resources is written within the reconciliation logic itself. However, these functions must be called
// in the right order depending on whether resources are getting created or deleted. During creation we must create the role and sa first.
// RoleBinding and deployment are dependent on these resouces. During deletion the order is reversed.
// Deployment and RoleBinding must be deleted before the role and sa. deleteNotificationsResources will only be called during
// delete events, so we don't need to worry about duplicate, recurring reconciliation calls
func (r *ReconcileArgoCD) deleteNotificationsResources(cr *argoproj.ArgoCD) error {

	sa := &corev1.ServiceAccount{}
	role := &rbacv1.Role{}

	if err := argoutil.FetchObject(r.Client, cr.Namespace, fmt.Sprintf("%s-%s", cr.Name, common.ArgoCDNotificationsControllerComponent), sa); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	if err := argoutil.FetchObject(r.Client, cr.Namespace, fmt.Sprintf("%s-%s", cr.Name, common.ArgoCDNotificationsControllerComponent), role); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	log.Info("reconciling notifications deployment")
	if err := r.reconcileNotificationsDeployment(cr, sa); err != nil {
		return err
	}

	log.Info("reconciling notifications secret")
	if err := r.reconcileNotificationsSecret(cr); err != nil {
		return err
	}

	log.Info("reconciling notifications configmap")
	if err := r.reconcileNotificationsConfigMap(cr); err != nil {
		return err
	}

	log.Info("reconciling notifications role binding")
	if err := r.reconcileNotificationsRoleBinding(cr, role, sa); err != nil {
		return err
	}

	log.Info("reconciling notifications role")
	_, err := r.reconcileNotificationsRole(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling notifications serviceaccount")
	_, err = r.reconcileNotificationsServiceAccount(cr)
	if err != nil {
		return err
	}

	return nil
}

func (r *ReconcileArgoCD) reconcileNotificationsServiceAccount(cr *argoproj.ArgoCD) (*corev1.ServiceAccount, error) {

	sa := newServiceAccountWithName(common.ArgoCDNotificationsControllerComponent, cr)

	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get the serviceAccount associated with %s : %s", sa.Name, err)
		}

		// SA doesn't exist and shouldn't, nothing to do here
		if !cr.Spec.Notifications.Enabled {
			return nil, nil
		}

		// SA doesn't exist but should, so it should be created
		if err := controllerutil.SetControllerReference(cr, sa, r.Scheme); err != nil {
			return nil, err
		}

		log.Info(fmt.Sprintf("Creating serviceaccount %s", sa.Name))
		err := r.Client.Create(context.TODO(), sa)
		if err != nil {
			return nil, err
		}
	}

	// SA exists but shouldn't, so it should be deleted
	if !cr.Spec.Notifications.Enabled {
		log.Info(fmt.Sprintf("Deleting serviceaccount %s as notifications is disabled", sa.Name))
		return nil, r.Client.Delete(context.TODO(), sa)
	}

	return sa, nil
}

func (r *ReconcileArgoCD) reconcileNotificationsRole(cr *argoproj.ArgoCD) (*rbacv1.Role, error) {

	policyRules := policyRuleForNotificationsController()
	desiredRole := newRole(common.ArgoCDNotificationsControllerComponent, policyRules, cr)

	existingRole := &rbacv1.Role{}
	if err := argoutil.FetchObject(r.Client, cr.Namespace, desiredRole.Name, existingRole); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get the role associated with %s : %s", desiredRole.Name, err)
		}

		// role does not exist and shouldn't, nothing to do here
		if !cr.Spec.Notifications.Enabled {
			return nil, nil
		}

		// role does not exist but should, so it should be created
		if err := controllerutil.SetControllerReference(cr, desiredRole, r.Scheme); err != nil {
			return nil, err
		}

		log.Info(fmt.Sprintf("Creating role %s", desiredRole.Name))
		err := r.Client.Create(context.TODO(), desiredRole)
		if err != nil {
			return nil, err
		}
		return desiredRole, nil
	}

	// role exists but shouldn't, so it should be deleted
	if !cr.Spec.Notifications.Enabled {
		log.Info(fmt.Sprintf("Deleting role %s as notifications is disabled", existingRole.Name))
		return nil, r.Client.Delete(context.TODO(), existingRole)
	}

	// role exists and should. Reconcile role if changed
	if !reflect.DeepEqual(existingRole.Rules, desiredRole.Rules) {
		existingRole.Rules = desiredRole.Rules
		if err := controllerutil.SetControllerReference(cr, existingRole, r.Scheme); err != nil {
			return nil, err
		}
		return existingRole, r.Client.Update(context.TODO(), existingRole)
	}

	return desiredRole, nil
}

func (r *ReconcileArgoCD) reconcileNotificationsRoleBinding(cr *argoproj.ArgoCD, role *rbacv1.Role, sa *corev1.ServiceAccount) error {

	desiredRoleBinding := newRoleBindingWithname(common.ArgoCDNotificationsControllerComponent, cr)
	desiredRoleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}

	desiredRoleBinding.Subjects = []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
	}

	// fetch existing rolebinding by name
	existingRoleBinding := &rbacv1.RoleBinding{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: desiredRoleBinding.Name, Namespace: cr.Namespace}, existingRoleBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", desiredRoleBinding.Name, err)
		}

		// roleBinding does not exist and shouldn't, nothing to do here
		if !cr.Spec.Notifications.Enabled {
			return nil
		}

		// roleBinding does not exist but should, so it should be created
		if err := controllerutil.SetControllerReference(cr, desiredRoleBinding, r.Scheme); err != nil {
			return err
		}

		log.Info(fmt.Sprintf("Creating roleBinding %s", desiredRoleBinding.Name))
		return r.Client.Create(context.TODO(), desiredRoleBinding)
	}

	// roleBinding exists but shouldn't, so it should be deleted
	if !cr.Spec.Notifications.Enabled {
		log.Info(fmt.Sprintf("Deleting roleBinding %s as notifications is disabled", existingRoleBinding.Name))
		return r.Client.Delete(context.TODO(), existingRoleBinding)
	}

	// roleBinding exists and should. Reconcile roleBinding if changed
	if !reflect.DeepEqual(existingRoleBinding.RoleRef, desiredRoleBinding.RoleRef) {
		// if the RoleRef changes, delete the existing role binding and create a new one
		if err := r.Client.Delete(context.TODO(), existingRoleBinding); err != nil {
			return err
		}
	} else if !reflect.DeepEqual(existingRoleBinding.Subjects, desiredRoleBinding.Subjects) {
		existingRoleBinding.Subjects = desiredRoleBinding.Subjects
		if err := controllerutil.SetControllerReference(cr, existingRoleBinding, r.Scheme); err != nil {
			return err
		}
		return r.Client.Update(context.TODO(), existingRoleBinding)
	}

	return nil
}

func (r *ReconcileArgoCD) reconcileNotificationsDeployment(cr *argoproj.ArgoCD, sa *corev1.ServiceAccount) error {

	desiredDeployment := newDeploymentWithSuffix("notifications-controller", "controller", cr)

	desiredDeployment.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RecreateDeploymentStrategyType,
	}

	if replicas := getArgoCDNotificationsControllerReplicas(cr); replicas != nil {
		desiredDeployment.Spec.Replicas = replicas
	}

	notificationEnv := cr.Spec.Notifications.Env
	// Let user specify their own environment first
	notificationEnv = argoutil.EnvMerge(notificationEnv, proxyEnvVars(), false)

	podSpec := &desiredDeployment.Spec.Template.Spec
	podSpec.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot: boolPtr(true),
	}
	AddSeccompProfileForOpenShift(r.Client, podSpec)
	podSpec.ServiceAccountName = sa.ObjectMeta.Name
	podSpec.Volumes = []corev1.Volume{
		{
			Name: "tls-certs",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDTLSCertsConfigMapName,
					},
				},
			},
		},
		{
			Name: "argocd-repo-server-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRepoServerTLSSecretName,
					Optional:   boolPtr(true),
				},
			},
		},
	}

	podSpec.Containers = []corev1.Container{{
		Command:         getNotificationsCommand(cr),
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            common.ArgoCDNotificationsControllerComponent,
		Env:             notificationEnv,
		Resources:       getNotificationsResources(cr),
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.IntOrString{
						IntVal: int32(9001),
					},
				},
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "tls-certs",
				MountPath: "/app/config/tls",
			},
			{
				Name:      "argocd-repo-server-tls",
				MountPath: "/app/config/reposerver/tls",
			},
		},
		WorkingDir: "/app",
	}}

	// fetch existing deployment by name
	deploymentChanged := false
	existingDeployment := &appsv1.Deployment{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: desiredDeployment.Name, Namespace: cr.Namespace}, existingDeployment); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the deployment associated with %s : %s", existingDeployment.Name, err)
		}

		// deployment does not exist and shouldn't, nothing to do here
		if !cr.Spec.Notifications.Enabled {
			return nil
		}

		// deployment does not exist but should, so it should be created
		if err := controllerutil.SetControllerReference(cr, desiredDeployment, r.Scheme); err != nil {
			return err
		}

		log.Info(fmt.Sprintf("Creating deployment %s", desiredDeployment.Name))
		return r.Client.Create(context.TODO(), desiredDeployment)
	}

	// deployment exists but shouldn't, so it should be deleted
	if !cr.Spec.Notifications.Enabled {
		log.Info(fmt.Sprintf("Deleting deployment %s as notifications is disabled", existingDeployment.Name))
		return r.Client.Delete(context.TODO(), existingDeployment)
	}

	// deployment exists and should. Reconcile deployment if changed
	updateNodePlacement(existingDeployment, desiredDeployment, &deploymentChanged)

	if existingDeployment.Spec.Template.Spec.Containers[0].Image != desiredDeployment.Spec.Template.Spec.Containers[0].Image {
		existingDeployment.Spec.Template.Spec.Containers[0].Image = desiredDeployment.Spec.Template.Spec.Containers[0].Image
		existingDeployment.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Template.Spec.Containers[0].Command, desiredDeployment.Spec.Template.Spec.Containers[0].Command) {
		existingDeployment.Spec.Template.Spec.Containers[0].Command = desiredDeployment.Spec.Template.Spec.Containers[0].Command
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Template.Spec.Containers[0].Env,
		desiredDeployment.Spec.Template.Spec.Containers[0].Env) {
		existingDeployment.Spec.Template.Spec.Containers[0].Env = desiredDeployment.Spec.Template.Spec.Containers[0].Env
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Template.Spec.Volumes, desiredDeployment.Spec.Template.Spec.Volumes) {
		existingDeployment.Spec.Template.Spec.Volumes = desiredDeployment.Spec.Template.Spec.Volumes
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Replicas, desiredDeployment.Spec.Replicas) {
		existingDeployment.Spec.Replicas = desiredDeployment.Spec.Replicas
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, desiredDeployment.Spec.Template.Spec.Containers[0].VolumeMounts) {
		existingDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = desiredDeployment.Spec.Template.Spec.Containers[0].VolumeMounts
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Template.Spec.Containers[0].Resources, desiredDeployment.Spec.Template.Spec.Containers[0].Resources) {
		existingDeployment.Spec.Template.Spec.Containers[0].Resources = desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Template.Spec.ServiceAccountName, desiredDeployment.Spec.Template.Spec.ServiceAccountName) {
		existingDeployment.Spec.Template.Spec.ServiceAccountName = desiredDeployment.Spec.Template.Spec.ServiceAccountName
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Labels, desiredDeployment.Labels) {
		existingDeployment.Labels = desiredDeployment.Labels
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Template.Labels, desiredDeployment.Spec.Template.Labels) {
		existingDeployment.Spec.Template.Labels = desiredDeployment.Spec.Template.Labels
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Selector, desiredDeployment.Spec.Selector) {
		existingDeployment.Spec.Selector = desiredDeployment.Spec.Selector
		deploymentChanged = true
	}

	if deploymentChanged {
		return r.Client.Update(context.TODO(), existingDeployment)
	}

	return nil

}

// getDefaultNotificationsConfig returns a map that contains default triggers and template configurations for argocd-notifications-cm
func getDefaultNotificationsConfig() map[string]string {

	notificationsConfig := make(map[string]string)

	// configure default notifications templates

	notificationsConfig["template.app-created"] = `email:
  subject: Application {{.app.metadata.name}} has been created.
message: Application {{.app.metadata.name}} has been created.
teams:
  title: Application {{.app.metadata.name}} has been created.`

	notificationsConfig["template.app-deleted"] = `email:
  subject: Application {{.app.metadata.name}} has been deleted.
message: Application {{.app.metadata.name}} has been deleted.
teams:
  title: Application {{.app.metadata.name}} has been deleted.`

	notificationsConfig["template.app-deployed"] = `email:
  subject: New version of an application {{.app.metadata.name}} is up and running.
message: |
  {{if eq .serviceType "slack"}}:white_check_mark:{{end}} Application {{.app.metadata.name}} is now running new version of deployments manifests.
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#18be52",
      "fields": [
      {
        "title": "Sync Status",
        "value": "{{.app.status.sync.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      },
      {
        "title": "Revision",
        "value": "{{.app.status.sync.revision}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Sync Status",
      "value": "{{.app.status.sync.status}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    },
    {
      "name": "Revision",
      "value": "{{.app.status.sync.revision}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |-
    [{
      "@type":"OpenUri",
      "name":"Operation Application",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  themeColor: '#000080'
  title: New version of an application {{.app.metadata.name}} is up and running.`

	notificationsConfig["template.app-health-degraded"] = `email:
  subject: Application {{.app.metadata.name}} has degraded.
message: |
  {{if eq .serviceType "slack"}}:exclamation:{{end}} Application {{.app.metadata.name}} has degraded.
  Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link": "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#f4c030",
      "fields": [
      {
        "title": "Health Status",
        "value": "{{.app.status.health.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Health Status",
      "value": "{{.app.status.health.status}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |
    [{
      "@type":"OpenUri",
      "name":"Open Application",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  themeColor: '#FF0000'
  title: Application {{.app.metadata.name}} has degraded.`

	notificationsConfig["template.app-sync-failed"] = `email:
  subject: Failed to sync application {{.app.metadata.name}}.
message: |
  {{if eq .serviceType "slack"}}:exclamation:{{end}}  The sync operation of application {{.app.metadata.name}} has failed at {{.app.status.operationState.finishedAt}} with the following error: {{.app.status.operationState.message}}
  Sync operation details are available at: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true .
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#E96D76",
      "fields": [
      {
        "title": "Sync Status",
        "value": "{{.app.status.sync.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Sync Status",
      "value": "{{.app.status.sync.status}}"
    },
    {
      "name": "Failed at",
      "value": "{{.app.status.operationState.finishedAt}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |-
    [{
      "@type":"OpenUri",
      "name":"Open Operation",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  themeColor: '#FF0000'
  title: Failed to sync application {{.app.metadata.name}}.`

	notificationsConfig["template.app-sync-running"] = `email:
  subject: Start syncing application {{.app.metadata.name}}.
message: |
  The sync operation of application {{.app.metadata.name}} has started at {{.app.status.operationState.startedAt}}.
  Sync operation details are available at: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true .
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#0DADEA",
      "fields": [
      {
        "title": "Sync Status",
        "value": "{{.app.status.sync.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Sync Status",
      "value": "{{.app.status.sync.status}}"
    },
    {
      "name": "Started at",
      "value": "{{.app.status.operationState.startedAt}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |-
    [{
      "@type":"OpenUri",
      "name":"Open Operation",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  title: Start syncing application {{.app.metadata.name}}.`

	notificationsConfig["template.app-sync-status-unknown"] = `email:
  subject: Application {{.app.metadata.name}} sync status is 'Unknown'
message: |
  {{if eq .serviceType "slack"}}:exclamation:{{end}} Application {{.app.metadata.name}} sync is 'Unknown'.
  Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.
  {{if ne .serviceType "slack"}}
  {{range $c := .app.status.conditions}}
      * {{$c.message}}
  {{end}}
  {{end}}
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#E96D76",
      "fields": [
      {
        "title": "Sync Status",
        "value": "{{.app.status.sync.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Sync Status",
      "value": "{{.app.status.sync.status}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |-
    [{
      "@type":"OpenUri",
      "name":"Open Application",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  title: Application {{.app.metadata.name}} sync status is 'Unknown'`

	notificationsConfig["template.app-sync-succeeded"] = `email:
  subject: Application {{.app.metadata.name}} has been successfully synced.
message: |
  {{if eq .serviceType "slack"}}:white_check_mark:{{end}} Application {{.app.metadata.name}} has been successfully synced at {{.app.status.operationState.finishedAt}}.
  Sync operation details are available at: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true .
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#18be52",
      "fields": [
      {
        "title": "Sync Status",
        "value": "{{.app.status.sync.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Sync Status",
      "value": "{{.app.status.sync.status}}"
    },
    {
      "name": "Synced at",
      "value": "{{.app.status.operationState.finishedAt}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |-
    [{
      "@type":"OpenUri",
      "name":"Operation Details",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  themeColor: '#000080'
  title: Application {{.app.metadata.name}} has been successfully synced`

	// configure default notifications triggers

	notificationsConfig["trigger.on-created"] = `- description: Application is created.
  oncePer: app.metadata.name
  send:
  - app-created
  when: "true"`

	notificationsConfig["trigger.on-deleted"] = `- description: Application is deleted.
  oncePer: app.metadata.name
  send:
  - app-deleted
  when: app.metadata.deletionTimestamp != nil`

	notificationsConfig["trigger.on-deployed"] = `- description: Application is synced and healthy. Triggered once per commit.
  oncePer: app.status.operationState.syncResult.revision
  send:
  - app-deployed
  when: app.status.operationState.phase in ['Succeeded'] and app.status.health.status
      == 'Healthy'`

	notificationsConfig["trigger.on-health-degraded"] = `- description: Application has degraded
  send:
  - app-health-degraded
  when: app.status.health.status == 'Degraded'`

	notificationsConfig["trigger.on-sync-failed"] = `- description: Application syncing has failed
  send:
  - app-sync-failed
  when: app.status.operationState.phase in ['Error', 'Failed']`

	notificationsConfig["trigger.on-sync-running"] = `- description: Application is being synced
  send:
  - app-sync-running
  when: app.status.operationState.phase in ['Running']`

	notificationsConfig["trigger.on-sync-status-unknown"] = `- description: Application status is 'Unknown'
  send:
  - app-sync-status-unknown
  when: app.status.sync.status == 'Unknown'`

	notificationsConfig["trigger.on-sync-succeeded"] = `- description: Application syncing has succeeded
  send:
  - app-sync-succeeded
  when: app.status.operationState.phase in ['Succeeded']`

	return notificationsConfig
}

// getArgoCDNotificationsControllerReplicas will return the size value for the argocd-notifications-controller replica count if it
// has been set in argocd CR. Otherwise, nil is returned if the replicas is not set in the argocd CR or
// replicas value is < 0.
func getArgoCDNotificationsControllerReplicas(cr *argoproj.ArgoCD) *int32 {
	if cr.Spec.Notifications.Replicas != nil && *cr.Spec.Notifications.Replicas >= 0 {
		return cr.Spec.Notifications.Replicas
	}

	return nil
}

const (
	ApplicationSetGitlabSCMTlsCertPath = "/app/tls/scm/cert"
)

// getArgoApplicationSetCommand will return the command for the ArgoCD ApplicationSet component.
func getArgoApplicationSetCommand(cr *argoproj.ArgoCD) []string {
	cmd := make([]string, 0)

	cmd = append(cmd, "entrypoint.sh")
	cmd = append(cmd, "argocd-applicationset-controller")

	if cr.Spec.Repo.IsEnabled() {
		cmd = append(cmd, "--argocd-repo-server", getRepoServerAddress(cr))
	} else {
		log.Info("Repo Server is disabled. This would affect the functioning of ApplicationSet Controller.")
	}

	cmd = append(cmd, "--loglevel")
	cmd = append(cmd, getLogLevel(cr.Spec.ApplicationSet.LogLevel))

	if cr.Spec.ApplicationSet.SCMRootCAConfigMap != "" {
		cmd = append(cmd, "--scm-root-ca-path")
		cmd = append(cmd, ApplicationSetGitlabSCMTlsCertPath)
	}

	// ApplicationSet command arguments provided by the user
	extraArgs := cr.Spec.ApplicationSet.ExtraCommandArgs
	err := isMergable(extraArgs, cmd)
	if err != nil {
		return cmd
	}

	cmd = append(cmd, extraArgs...)

	return cmd
}

func (r *ReconcileArgoCD) reconcileApplicationSetController(cr *argoproj.ArgoCD) error {

	log.Info("reconciling applicationset serviceaccounts")
	sa, err := r.reconcileApplicationSetServiceAccount(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling applicationset roles")
	role, err := r.reconcileApplicationSetRole(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling applicationset role bindings")
	if err := r.reconcileApplicationSetRoleBinding(cr, role, sa); err != nil {
		return err
	}

	log.Info("reconciling applicationset deployments")
	if err := r.reconcileApplicationSetDeployment(cr, sa); err != nil {
		return err
	}

	log.Info("reconciling applicationset service")
	if err := r.reconcileApplicationSetService(cr); err != nil {
		return err
	}

	return nil
}

// reconcileApplicationControllerDeployment will ensure the Deployment resource is present for the ArgoCD Application Controller component.
func (r *ReconcileArgoCD) reconcileApplicationSetDeployment(cr *argoproj.ArgoCD, sa *corev1.ServiceAccount) error {
	deploy := newDeploymentWithSuffix("applicationset-controller", "controller", cr)

	setAppSetLabels(&deploy.ObjectMeta)

	podSpec := &deploy.Spec.Template.Spec

	// sa would be nil when spec.applicationset.enabled = false
	if sa != nil {
		podSpec.ServiceAccountName = sa.ObjectMeta.Name
	}
	podSpec.Volumes = []corev1.Volume{
		{
			Name: "ssh-known-hosts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDKnownHostsConfigMapName,
					},
				},
			},
		},
		{
			Name: "tls-certs",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDTLSCertsConfigMapName,
					},
				},
			},
		},
		{
			Name: "gpg-keys",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDGPGKeysConfigMapName,
					},
				},
			},
		},
		{
			Name: "gpg-keyring",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "tmp",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	addSCMGitlabVolumeMount := false
	if scmRootCAConfigMapName := getSCMRootCAConfigMapName(cr); scmRootCAConfigMapName != "" {
		cm := newConfigMapWithName(scmRootCAConfigMapName, cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, cr.Spec.ApplicationSet.SCMRootCAConfigMap, cm) {
			addSCMGitlabVolumeMount = true
			podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
				Name: "appset-gitlab-scm-tls-cert",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: common.ArgoCDAppSetGitlabSCMTLSCertsConfigMapName,
						},
					},
				},
			})
		}
	}

	podSpec.Containers = []corev1.Container{
		applicationSetContainer(cr, addSCMGitlabVolumeMount),
	}
	AddSeccompProfileForOpenShift(r.Client, podSpec)

	if existing := newDeploymentWithSuffix("applicationset-controller", "controller", cr); argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {

		if cr.Spec.ApplicationSet != nil && !cr.Spec.ApplicationSet.IsEnabled() {
			err := r.Client.Delete(context.TODO(), existing)
			return err
		}

		existingSpec := existing.Spec.Template.Spec

		deploymentsDifferent := !reflect.DeepEqual(existingSpec.Containers[0], podSpec.Containers) ||
			!reflect.DeepEqual(existingSpec.Volumes, podSpec.Volumes) ||
			existingSpec.ServiceAccountName != podSpec.ServiceAccountName ||
			!reflect.DeepEqual(existing.Labels, deploy.Labels) ||
			!reflect.DeepEqual(existing.Spec.Template.Labels, deploy.Spec.Template.Labels) ||
			!reflect.DeepEqual(existing.Spec.Selector, deploy.Spec.Selector) ||
			!reflect.DeepEqual(existing.Spec.Template.Spec.NodeSelector, deploy.Spec.Template.Spec.NodeSelector) ||
			!reflect.DeepEqual(existing.Spec.Template.Spec.Tolerations, deploy.Spec.Template.Spec.Tolerations)

		// If the Deployment already exists, make sure the values we care about are up-to-date
		if deploymentsDifferent {
			existing.Spec.Template.Spec.Containers = podSpec.Containers
			existing.Spec.Template.Spec.Volumes = podSpec.Volumes
			existing.Spec.Template.Spec.ServiceAccountName = podSpec.ServiceAccountName
			existing.Labels = deploy.Labels
			existing.Spec.Template.Labels = deploy.Spec.Template.Labels
			existing.Spec.Selector = deploy.Spec.Selector
			existing.Spec.Template.Spec.NodeSelector = deploy.Spec.Template.Spec.NodeSelector
			existing.Spec.Template.Spec.Tolerations = deploy.Spec.Template.Spec.Tolerations
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if !cr.Spec.ApplicationSet.IsEnabled() {
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)

}

func applicationSetContainer(cr *argoproj.ArgoCD, addSCMGitlabVolumeMount bool) corev1.Container {
	// Global proxy env vars go first
	appSetEnv := []corev1.EnvVar{{
		Name: "NAMESPACE",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	}}

	// Merge ApplicationSet env vars provided by the user
	// User should be able to override the default NAMESPACE environmental variable
	appSetEnv = argoutil.EnvMerge(cr.Spec.ApplicationSet.Env, appSetEnv, true)
	// Environment specified in the CR take precedence over everything else
	appSetEnv = argoutil.EnvMerge(appSetEnv, proxyEnvVars(), false)

	container := corev1.Container{
		Command:         getArgoApplicationSetCommand(cr),
		Env:             appSetEnv,
		Image:           getApplicationSetContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "argocd-applicationset-controller",
		Resources:       getApplicationSetResources(cr),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "ssh-known-hosts",
				MountPath: "/app/config/ssh",
			},
			{
				Name:      "tls-certs",
				MountPath: "/app/config/tls",
			},
			{
				Name:      "gpg-keys",
				MountPath: "/app/config/gpg/source",
			},
			{
				Name:      "gpg-keyring",
				MountPath: "/app/config/gpg/keys",
			},
			{
				Name:      "tmp",
				MountPath: "/tmp",
			},
		},
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 7000,
				Name:          "webhook",
			},
			{
				ContainerPort: 8080,
				Name:          "metrics",
			},
		},
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			AllowPrivilegeEscalation: boolPtr(false),
			ReadOnlyRootFilesystem:   boolPtr(true),
			RunAsNonRoot:             boolPtr(true),
		},
	}
	if addSCMGitlabVolumeMount {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "appset-gitlab-scm-tls-cert",
			MountPath: ApplicationSetGitlabSCMTlsCertPath,
		})
	}
	return container
}

func (r *ReconcileArgoCD) reconcileApplicationSetServiceAccount(cr *argoproj.ArgoCD) (*corev1.ServiceAccount, error) {

	sa := newServiceAccountWithName("applicationset-controller", cr)
	setAppSetLabels(&sa.ObjectMeta)

	exists := true
	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		exists = false
	}

	if exists {
		if cr.Spec.ApplicationSet != nil && !cr.Spec.ApplicationSet.IsEnabled() {
			err := r.Client.Delete(context.TODO(), sa)
			return nil, err
		}
		return sa, nil
	}

	if err := controllerutil.SetControllerReference(cr, sa, r.Scheme); err != nil {
		return nil, err
	}

	if cr.Spec.ApplicationSet != nil && !cr.Spec.ApplicationSet.IsEnabled() {
		return nil, nil
	}

	err := r.Client.Create(context.TODO(), sa)
	if err != nil {
		return nil, err
	}

	return sa, err
}

func (r *ReconcileArgoCD) reconcileApplicationSetRole(cr *argoproj.ArgoCD) (*rbacv1.Role, error) {

	policyRules := []rbacv1.PolicyRule{

		// ApplicationSet
		{
			APIGroups: []string{"argoproj.io"},
			Resources: []string{
				"applications",
				"applicationsets",
				"appprojects",
				"applicationsets/finalizers",
			},
			Verbs: []string{
				"create",
				"delete",
				"get",
				"list",
				"patch",
				"update",
				"watch",
			},
		},
		// ApplicationSet Status
		{
			APIGroups: []string{"argoproj.io"},
			Resources: []string{
				"applicationsets/status",
			},
			Verbs: []string{
				"get",
				"patch",
				"update",
			},
		},

		// Events
		{
			APIGroups: []string{""},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"create",
				"delete",
				"get",
				"list",
				"patch",
				"update",
				"watch",
			},
		},

		// Read Secrets/ConfigMaps
		{
			APIGroups: []string{""},
			Resources: []string{
				"secrets",
				"configmaps",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},

		// Read Deployments
		{
			APIGroups: []string{"apps", "extensions"},
			Resources: []string{
				"deployments",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
	}

	role := newRole("applicationset-controller", policyRules, cr)
	setAppSetLabels(&role.ObjectMeta)

	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: cr.Namespace}, role)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the role for the service account associated with %s : %s", role.Name, err)
		}
		if apierrors.IsNotFound(err) && cr.Spec.ApplicationSet != nil && !cr.Spec.ApplicationSet.IsEnabled() {
			return nil, nil
		}
		if err = controllerutil.SetControllerReference(cr, role, r.Scheme); err != nil {
			return nil, err
		}
		return role, r.Client.Create(context.TODO(), role)
	}
	if cr.Spec.ApplicationSet != nil && !cr.Spec.ApplicationSet.IsEnabled() {
		return nil, r.Client.Delete(context.TODO(), role)
	}

	role.Rules = policyRules
	if err = controllerutil.SetControllerReference(cr, role, r.Scheme); err != nil {
		return nil, err
	}
	return role, r.Client.Update(context.TODO(), role)
}

func (r *ReconcileArgoCD) reconcileApplicationSetRoleBinding(cr *argoproj.ArgoCD, role *rbacv1.Role, sa *corev1.ServiceAccount) error {

	name := "applicationset-controller"

	// get expected name
	roleBinding := newRoleBindingWithname(name, cr)

	// fetch existing rolebinding by name
	roleBindingExists := true
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: cr.Namespace}, roleBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", name, err)
		}
		if apierrors.IsNotFound(err) && cr.Spec.ApplicationSet != nil && !cr.Spec.ApplicationSet.IsEnabled() {
			return nil
		}
		roleBindingExists = false
	}

	if cr.Spec.ApplicationSet != nil && !cr.Spec.ApplicationSet.IsEnabled() {
		return r.Client.Delete(context.TODO(), roleBinding)
	}

	setAppSetLabels(&roleBinding.ObjectMeta)

	roleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}

	roleBinding.Subjects = []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
	}

	if err := controllerutil.SetControllerReference(cr, roleBinding, r.Scheme); err != nil {
		return err
	}

	if roleBindingExists {
		return r.Client.Update(context.TODO(), roleBinding)
	}

	return r.Client.Create(context.TODO(), roleBinding)
}

func getApplicationSetContainerImage(cr *argoproj.ArgoCD) string {
	defaultImg, defaultTag := false, false

	img := ""
	tag := ""

	// First pull from spec, if it exists
	if cr.Spec.ApplicationSet != nil {
		img = cr.Spec.ApplicationSet.Image
		tag = cr.Spec.ApplicationSet.Version
	}

	// If spec is empty, use the defaults
	if img == "" {
		img = common.ArgoCDDefaultArgoImage
		defaultImg = true
	}
	if tag == "" {
		tag = common.ArgoCDDefaultArgoVersion
		defaultTag = true
	}

	// If an env var is specified then use that, but don't override the spec values (if they are present)
	if e := os.Getenv(common.ArgoCDImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}
	return argoutil.CombineImageTag(img, tag)
}

// getApplicationSetResources will return the ResourceRequirements for the Application Sets container.
func getApplicationSetResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.ApplicationSet.Resources != nil {
		resources = *cr.Spec.ApplicationSet.Resources
	}

	return resources
}

func setAppSetLabels(obj *metav1.ObjectMeta) {
	obj.Labels["app.kubernetes.io/name"] = "argocd-applicationset-controller"
	obj.Labels["app.kubernetes.io/part-of"] = "argocd-applicationset"
	obj.Labels["app.kubernetes.io/component"] = "controller"
}

// reconcileApplicationSetService will ensure that the Service is present for the ApplicationSet webhook and metrics component.
func (r *ReconcileArgoCD) reconcileApplicationSetService(cr *argoproj.ArgoCD) error {
	log.Info("reconciling applicationset service")

	svc := newServiceWithSuffix(common.ApplicationSetServiceNameSuffix, common.ApplicationSetServiceNameSuffix, cr)
	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.IsEnabled() {

		if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
			err := argoutil.FetchObject(r.Client, cr.Namespace, svc.Name, svc)
			if err != nil {
				return err
			}
			log.Info(fmt.Sprintf("Deleting applicationset controller service %s as applicationset is disabled", svc.Name))
			err = r.Delete(context.TODO(), svc)
			if err != nil {
				return err
			}
		}
		return nil
	} else {
		if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
			return nil // Service found, do nothing
		}
	}
	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "webhook",
			Port:       7000,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(7000),
		}, {
			Name:       "metrics",
			Port:       8080,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8080),
		},
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix(common.ApplicationSetServiceNameSuffix, cr),
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
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

func (r *ReconcileArgoCD) redisShouldUseTLS(cr *argoproj.ArgoCD) bool {
	var tlsSecretObj corev1.Secret
	tlsSecretName := types.NamespacedName{Namespace: cr.Namespace, Name: common.ArgoCDRedisServerTLSSecretName}
	err := r.Client.Get(context.TODO(), tlsSecretName, &tlsSecretObj)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "error looking up redis tls secret")
		}
		return false
	}

	secretOwnerRefs := tlsSecretObj.GetOwnerReferences()
	if len(secretOwnerRefs) > 0 {
		// OpenShift service CA makes the owner reference for the TLS secret to the
		// service, which in turn is owned by the controller. This method performs
		// a lookup of the controller through the intermediate owning service.
		for _, secretOwner := range secretOwnerRefs {
			if isOwnerOfInterest(secretOwner) {
				key := client.ObjectKey{Name: secretOwner.Name, Namespace: tlsSecretObj.GetNamespace()}
				svc := &corev1.Service{}

				// Get the owning object of the secret
				err := r.Client.Get(context.TODO(), key, svc)
				if err != nil {
					log.Error(err, fmt.Sprintf("could not get owner of secret %s", tlsSecretObj.GetName()))
					return false
				}

				// If there's an object of kind ArgoCD in the owner's list,
				// this will be our reconciled object.
				serviceOwnerRefs := svc.GetOwnerReferences()
				for _, serviceOwner := range serviceOwnerRefs {
					if serviceOwner.Kind == "ArgoCD" {
						return true
					}
				}
			}
		}
	} else {
		// For secrets without owner (i.e. manually created), we apply some
		// heuristics. This may not be as accurate (e.g. if the user made a
		// typo in the resource's name), but should be good enough for now.
		if _, ok := tlsSecretObj.Annotations[common.AnnotationName]; ok {
			return true
		}
	}
	return false
}

// getRedisHAProxyAddress will return the Redis HA Proxy service address for the given ArgoCD.
func getRedisHAProxyAddress(cr *argoproj.ArgoCD) string {
	return fqdnServiceRef("redis-ha-haproxy", common.ArgoCDDefaultRedisPort, cr)
}

// getRedisServerAddress will return the Redis service address for the given ArgoCD.
func getRedisServerAddress(cr *argoproj.ArgoCD) string {
	if cr.Spec.Redis.Remote != nil && *cr.Spec.Redis.Remote != "" {
		return *cr.Spec.Redis.Remote
	}
	if cr.Spec.HA.Enabled {
		return getRedisHAProxyAddress(cr)
	}
	return fqdnServiceRef(common.ArgoCDDefaultRedisSuffix, common.ArgoCDDefaultRedisPort, cr)
}

// getRedisContainerImage will return the container image for the Redis server.
func getRedisContainerImage(cr *argoproj.ArgoCD) string {
	defaultImg, defaultTag := false, false
	img := cr.Spec.Redis.Image
	if img == "" {
		img = common.ArgoCDDefaultRedisImage
		defaultImg = true
	}
	tag := cr.Spec.Redis.Version
	if tag == "" {
		tag = common.ArgoCDDefaultRedisVersion
		defaultTag = true
	}
	if e := os.Getenv(common.ArgoCDRedisImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}
	return argoutil.CombineImageTag(img, tag)
}

// getRedisHAContainerImage will return the container image for the Redis server in HA mode.
func getRedisHAContainerImage(cr *argoproj.ArgoCD) string {
	defaultImg, defaultTag := false, false
	img := cr.Spec.Redis.Image
	if img == "" {
		img = common.ArgoCDDefaultRedisImage
		defaultImg = true
	}
	tag := cr.Spec.Redis.Version
	if tag == "" {
		tag = common.ArgoCDDefaultRedisVersionHA
		defaultTag = true
	}
	if e := os.Getenv(common.ArgoCDRedisHAImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}
	return argoutil.CombineImageTag(img, tag)
}

// getRedisHAProxyContainerImage will return the container image for the Redis HA Proxy.
func getRedisHAProxyContainerImage(cr *argoproj.ArgoCD) string {
	defaultImg, defaultTag := false, false
	img := cr.Spec.HA.RedisProxyImage
	if len(img) <= 0 {
		img = common.ArgoCDDefaultRedisHAProxyImage
		defaultImg = true
	}

	tag := cr.Spec.HA.RedisProxyVersion
	if len(tag) <= 0 {
		tag = common.ArgoCDDefaultRedisHAProxyVersion
		defaultTag = true
	}

	if e := os.Getenv(common.ArgoCDRedisHAProxyImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}

	return argoutil.CombineImageTag(img, tag)
}

// getRedisConfigPath will return the path for the Redis configuration templates.
func getRedisConfigPath() string {
	path := os.Getenv("REDIS_CONFIG_PATH")
	if len(path) > 0 {
		return path
	}
	return common.ArgoCDDefaultRedisConfigPath
}

// getRedisHAProxySConfig will load the Redis HA Proxy configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisHAProxyConfig(cr *argoproj.ArgoCD, useTLSForRedis bool) string {
	path := fmt.Sprintf("%s/haproxy.cfg.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": nameWithSuffix("redis-ha", cr),
		"UseTLS":      strconv.FormatBool(useTLSForRedis),
	}

	script, err := loadTemplateFile(path, vars)
	if err != nil {
		log.Error(err, "unable to load redis haproxy configuration")
		return ""
	}
	return script
}

// getRedisHAProxyScript will load the Redis HA Proxy init script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisHAProxyScript(cr *argoproj.ArgoCD) string {
	path := fmt.Sprintf("%s/haproxy_init.sh.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": nameWithSuffix("redis-ha", cr),
	}

	script, err := loadTemplateFile(path, vars)
	if err != nil {
		log.Error(err, "unable to load redis haproxy init script")
		return ""
	}
	return script
}

// getRedisResources will return the ResourceRequirements for the Redis container.
func getRedisResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Redis.Resources != nil {
		resources = *cr.Spec.Redis.Resources
	}

	return resources
}

// getRedisHAResources will return the ResourceRequirements for the Redis HA.
func getRedisHAResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.HA.Resources != nil {
		resources = *cr.Spec.HA.Resources
	}

	return resources
}

// getRedisInitScript will load the redis configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisConf(useTLSForRedis bool) string {
	path := fmt.Sprintf("%s/redis.conf.tpl", getRedisConfigPath())
	params := map[string]string{
		"UseTLS": strconv.FormatBool(useTLSForRedis),
	}
	conf, err := loadTemplateFile(path, params)
	if err != nil {
		log.Error(err, "unable to load redis configuration")
		return ""
	}
	return conf
}

// getRedisInitScript will load the redis init script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisInitScript(cr *argoproj.ArgoCD, useTLSForRedis bool) string {
	path := fmt.Sprintf("%s/init.sh.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": nameWithSuffix("redis-ha", cr),
		"UseTLS":      strconv.FormatBool(useTLSForRedis),
	}

	script, err := loadTemplateFile(path, vars)
	if err != nil {
		log.Error(err, "unable to load redis init-script")
		return ""
	}
	return script
}

// getRedisSentinelConf will load the redis sentinel configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisSentinelConf(useTLSForRedis bool) string {
	path := fmt.Sprintf("%s/sentinel.conf.tpl", getRedisConfigPath())
	params := map[string]string{
		"UseTLS": strconv.FormatBool(useTLSForRedis),
	}
	conf, err := loadTemplateFile(path, params)
	if err != nil {
		log.Error(err, "unable to load redis sentinel configuration")
		return ""
	}
	return conf
}

// getRedisLivenessScript will load the redis liveness script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisLivenessScript(useTLSForRedis bool) string {
	path := fmt.Sprintf("%s/redis_liveness.sh.tpl", getRedisConfigPath())
	params := map[string]string{
		"UseTLS": strconv.FormatBool(useTLSForRedis),
	}
	conf, err := loadTemplateFile(path, params)
	if err != nil {
		log.Error(err, "unable to load redis liveness script")
		return ""
	}
	return conf
}

// getRedisReadinessScript will load the redis readiness script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisReadinessScript(useTLSForRedis bool) string {
	path := fmt.Sprintf("%s/redis_readiness.sh.tpl", getRedisConfigPath())
	params := map[string]string{
		"UseTLS": strconv.FormatBool(useTLSForRedis),
	}
	conf, err := loadTemplateFile(path, params)
	if err != nil {
		log.Error(err, "unable to load redis readiness script")
		return ""
	}
	return conf
}

// getSentinelLivenessScript will load the redis liveness script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getSentinelLivenessScript(useTLSForRedis bool) string {
	path := fmt.Sprintf("%s/sentinel_liveness.sh.tpl", getRedisConfigPath())
	params := map[string]string{
		"UseTLS": strconv.FormatBool(useTLSForRedis),
	}
	conf, err := loadTemplateFile(path, params)
	if err != nil {
		log.Error(err, "unable to load sentinel liveness script")
		return ""
	}
	return conf
}

func isRedisTLSVerificationDisabled(cr *argoproj.ArgoCD) bool {
	return cr.Spec.Redis.DisableTLSVerification
}

// reconcileRedisHAConfigMap will ensure that the Redis HA ConfigMap is present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileRedisHAConfigMap(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	cm := newConfigMapWithName(common.ArgoCDRedisHAConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if !cr.Spec.HA.Enabled {
			// ConfigMap exists but HA enabled flag has been set to false, delete the ConfigMap
			return r.Client.Delete(context.TODO(), cm)
		}
		return nil // ConfigMap found with nothing changed, move along...
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	cm.Data = map[string]string{
		"haproxy.cfg":     getRedisHAProxyConfig(cr, useTLSForRedis),
		"haproxy_init.sh": getRedisHAProxyScript(cr),
		"init.sh":         getRedisInitScript(cr, useTLSForRedis),
		"redis.conf":      getRedisConf(useTLSForRedis),
		"sentinel.conf":   getRedisSentinelConf(useTLSForRedis),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileRedisHAConfigMap will ensure that the Redis HA Health ConfigMap is present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileRedisHAHealthConfigMap(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	cm := newConfigMapWithName(common.ArgoCDRedisHAHealthConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if !cr.Spec.HA.Enabled {
			// ConfigMap exists but HA enabled flag has been set to false, delete the ConfigMap
			return r.Client.Delete(context.TODO(), cm)
		}
		return nil // ConfigMap found with nothing changed, move along...
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	cm.Data = map[string]string{
		"redis_liveness.sh":    getRedisLivenessScript(useTLSForRedis),
		"redis_readiness.sh":   getRedisReadinessScript(useTLSForRedis),
		"sentinel_liveness.sh": getSentinelLivenessScript(useTLSForRedis),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileRedisConfiguration will ensure that all of the Redis ConfigMaps are present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileRedisConfiguration(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	if err := r.reconcileRedisHAConfigMap(cr, useTLSForRedis); err != nil {
		return err
	}
	if err := r.reconcileRedisHAHealthConfigMap(cr, useTLSForRedis); err != nil {
		return err
	}
	return nil
}

func getArgoRedisArgs(useTLS bool) []string {
	args := make([]string, 0)

	args = append(args, "--save", "")
	args = append(args, "--appendonly", "no")

	if useTLS {
		args = append(args, "--tls-port", "6379")
		args = append(args, "--port", "0")

		args = append(args, "--tls-cert-file", "/app/config/redis/tls/tls.crt")
		args = append(args, "--tls-key-file", "/app/config/redis/tls/tls.key")
		args = append(args, "--tls-auth-clients", "no")
	}

	return args
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

// reconcileRedisHAAnnounceServices will ensure that the announce Services are present for Redis when running in HA mode.
func (r *ReconcileArgoCD) reconcileRedisHAAnnounceServices(cr *argoproj.ArgoCD) error {
	for i := int32(0); i < common.ArgoCDDefaultRedisHAReplicas; i++ {
		svc := newServiceWithSuffix(fmt.Sprintf("redis-ha-announce-%d", i), "redis", cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
			if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
				return r.Client.Delete(context.TODO(), svc)
			}
			return nil // Service found, do nothing
		}

		if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
			return nil //return as Ha is not enabled do nothing
		}

		svc.ObjectMeta.Annotations = map[string]string{
			common.ArgoCDKeyTolerateUnreadyEndpounts: "true",
		}

		svc.Spec.PublishNotReadyAddresses = true

		svc.Spec.Selector = map[string]string{
			common.ArgoCDKeyName:               nameWithSuffix("redis-ha", cr),
			common.ArgoCDKeyStatefulSetPodName: nameWithSuffix(fmt.Sprintf("redis-ha-server-%d", i), cr),
		}

		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "server",
				Port:       common.ArgoCDDefaultRedisPort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromString("redis"),
			}, {
				Name:       "sentinel",
				Port:       common.ArgoCDDefaultRedisSentinelPort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromString("sentinel"),
			},
		}

		if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
			return err
		}

		if err := r.Client.Create(context.TODO(), svc); err != nil {
			return err
		}
	}
	return nil
}

// reconcileRedisHAMasterService will ensure that the "master" Service is present for Redis when running in HA mode.
func (r *ReconcileArgoCD) reconcileRedisHAMasterService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("redis-ha", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
			return r.Client.Delete(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
		return nil //return as Ha is not enabled do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("redis-ha", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "server",
			Port:       common.ArgoCDDefaultRedisPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString("redis"),
		}, {
			Name:       "sentinel",
			Port:       common.ArgoCDDefaultRedisSentinelPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString("sentinel"),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileRedisHAProxyService will ensure that the HA Proxy Service is present for Redis when running in HA mode.
func (r *ReconcileArgoCD) reconcileRedisHAProxyService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("redis-ha-haproxy", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {

		if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
			return r.Client.Delete(context.TODO(), svc)
		}

		if ensureAutoTLSAnnotation(svc, common.ArgoCDRedisServerTLSSecretName, cr.Spec.Redis.WantsAutoTLS()) {
			return r.Client.Update(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
		return nil //return as Ha is not enabled do nothing
	}

	ensureAutoTLSAnnotation(svc, common.ArgoCDRedisServerTLSSecretName, cr.Spec.Redis.WantsAutoTLS())

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("redis-ha-haproxy", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "haproxy",
			Port:       common.ArgoCDDefaultRedisPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString("redis"),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileRedisHAServices will ensure that all required Services are present for Redis when running in HA mode.
func (r *ReconcileArgoCD) reconcileRedisHAServices(cr *argoproj.ArgoCD) error {

	if err := r.reconcileRedisHAAnnounceServices(cr); err != nil {
		return err
	}

	if err := r.reconcileRedisHAMasterService(cr); err != nil {
		return err
	}

	if err := r.reconcileRedisHAProxyService(cr); err != nil {
		return err
	}
	return nil
}

// reconcileRedisService will ensure that the Service for Redis is present.
func (r *ReconcileArgoCD) reconcileRedisService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("redis", "redis", cr)

	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		if !cr.Spec.Redis.IsEnabled() {
			return r.Client.Delete(context.TODO(), svc)
		}
		if ensureAutoTLSAnnotation(svc, common.ArgoCDRedisServerTLSSecretName, cr.Spec.Redis.WantsAutoTLS()) {
			return r.Client.Update(context.TODO(), svc)
		}
		if cr.Spec.HA.Enabled {
			return r.Client.Delete(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	if cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
		return nil //return as Ha is enabled do nothing
	}

	ensureAutoTLSAnnotation(svc, common.ArgoCDRedisServerTLSSecretName, cr.Spec.Redis.WantsAutoTLS())

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("redis", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "tcp-redis",
			Port:       common.ArgoCDDefaultRedisPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultRedisPort),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileRedisTLSSecret checks whether the argocd-operator-redis-tls secret
// has changed since our last reconciliation loop. It does so by comparing the
// checksum of tls.crt and tls.key in the status of the ArgoCD CR against the
// values calculated from the live state in the cluster.
func (r *ReconcileArgoCD) reconcileRedisTLSSecret(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	var tlsSecretObj corev1.Secret
	var sha256sum string

	log.Info("reconciling redis-server TLS secret")

	tlsSecretName := types.NamespacedName{Namespace: cr.Namespace, Name: common.ArgoCDRedisServerTLSSecretName}
	err := r.Client.Get(context.TODO(), tlsSecretName, &tlsSecretObj)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	} else if tlsSecretObj.Type != corev1.SecretTypeTLS {
		// We only process secrets of type kubernetes.io/tls
		return nil
	} else {
		// We do the checksum over a concatenated byte stream of cert + key
		crt, crtOk := tlsSecretObj.Data[corev1.TLSCertKey]
		key, keyOk := tlsSecretObj.Data[corev1.TLSPrivateKeyKey]
		if crtOk && keyOk {
			var sumBytes []byte
			sumBytes = append(sumBytes, crt...)
			sumBytes = append(sumBytes, key...)
			sha256sum = fmt.Sprintf("%x", sha256.Sum256(sumBytes))
		}
	}

	// The content of the TLS secret has changed since we last looked if the
	// calculated checksum doesn't match the one stored in the status.
	if cr.Status.RedisTLSChecksum != sha256sum {
		// We store the value early to prevent a possible restart loop, for the
		// cost of a possibly missed restart when we cannot update the status
		// field of the resource.
		cr.Status.RedisTLSChecksum = sha256sum
		err = r.Client.Status().Update(context.TODO(), cr)
		if err != nil {
			return err
		}

		// Trigger rollout of redis
		if cr.Spec.HA.Enabled {
			err = r.recreateRedisHAConfigMap(cr, useTLSForRedis)
			if err != nil {
				return err
			}
			err = r.recreateRedisHAHealthConfigMap(cr, useTLSForRedis)
			if err != nil {
				return err
			}
			haProxyDepl := newDeploymentWithSuffix("redis-ha-haproxy", "redis", cr)
			err = r.triggerRollout(haProxyDepl, "redis.tls.cert.changed")
			if err != nil {
				return err
			}
			// If we use triggerRollout on the redis stateful set, kubernetes will attempt to restart the  pods
			// one at a time, and the first one to restart (which will be using tls) will hang as it tries to
			// communicate with the existing pods (which are not using tls) to establish which is the master.
			// So instead we delete the stateful set, which will delete all the pods.
			redisSts := newStatefulSetWithSuffix("redis-ha-server", "redis", cr)
			if argoutil.IsObjectFound(r.Client, redisSts.Namespace, redisSts.Name, redisSts) {
				err = r.Client.Delete(context.TODO(), redisSts)
				if err != nil {
					return err
				}
			}
		} else {
			redisDepl := newDeploymentWithSuffix("redis", "redis", cr)
			err = r.triggerRollout(redisDepl, "redis.tls.cert.changed")
			if err != nil {
				return err
			}
		}

		// Trigger rollout of API server
		apiDepl := newDeploymentWithSuffix("server", "server", cr)
		err = r.triggerRollout(apiDepl, "redis.tls.cert.changed")
		if err != nil {
			return err
		}

		// Trigger rollout of repository server
		repoDepl := newDeploymentWithSuffix("repo-server", "repo-server", cr)
		err = r.triggerRollout(repoDepl, "redis.tls.cert.changed")
		if err != nil {
			return err
		}

		// Trigger rollout of application controller
		controllerSts := newStatefulSetWithSuffix("application-controller", "application-controller", cr)
		err = r.triggerRollout(controllerSts, "redis.tls.cert.changed")
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileArgoCD) recreateRedisHAConfigMap(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	cm := newConfigMapWithName(common.ArgoCDRedisHAConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if err := r.Client.Delete(context.TODO(), cm); err != nil {
			return err
		}
	}
	return r.reconcileRedisHAConfigMap(cr, useTLSForRedis)
}

func (r *ReconcileArgoCD) recreateRedisHAHealthConfigMap(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	cm := newConfigMapWithName(common.ArgoCDRedisHAHealthConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if err := r.Client.Delete(context.TODO(), cm); err != nil {
			return err
		}
	}
	return r.reconcileRedisHAHealthConfigMap(cr, useTLSForRedis)
}

// reconcileStatusRedis will ensure that the Redis status is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusRedis(cr *argoproj.ArgoCD) error {
	status := "Unknown"

	if !cr.Spec.HA.Enabled {
		deploy := newDeploymentWithSuffix("redis", "redis", cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
			status = "Pending"

			if deploy.Spec.Replicas != nil {
				if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
					status = "Running"
				}
			}
		}
	} else {
		ss := newStatefulSetWithSuffix("redis-ha-server", "redis-ha-server", cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, ss.Name, ss) {
			status = "Pending"

			if ss.Status.ReadyReplicas == *ss.Spec.Replicas {
				status = "Running"
			}
		}
		// TODO: Add check for HA proxy deployment here as well?
	}

	if cr.Status.Redis != status {
		cr.Status.Redis = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
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

func policyRuleForRedis(client client.Client) []v1.PolicyRule {
	rules := []v1.PolicyRule{}

	// Need additional policy rules if we are running on openshift, else the stateful set won't have the right
	// permissions to start
	rules = appendOpenShiftNonRootSCC(rules, client)

	return rules
}

func policyRuleForRedisHa(client client.Client) []v1.PolicyRule {

	rules := []v1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"endpoints",
			},
			Verbs: []string{
				"get",
			},
		},
	}

	// Need additional policy rules if we are running on openshift, else the stateful set won't have the right
	// permissions to start
	rules = appendOpenShiftNonRootSCC(rules, client)

	return rules
}
