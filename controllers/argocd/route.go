// Copyright 2019 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package argocd

import (
	"context"
	"fmt"
	"strings"

	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

const (
	maxLabelLength    = 63
	maxHostnameLength = 253
	minFirstLabelSize = 20
)

var routeAPIFound = false

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

// reconcileGrafanaRoute will ensure that the ArgoCD Grafana Route is present.
func (r *ReconcileArgoCD) reconcileGrafanaRoute(cr *argoproj.ArgoCD) error {
	route := newRouteWithSuffix("grafana", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, route.Name, route) {
		//nolint:staticcheck
		if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Route.Enabled {
			// Route exists but enabled flag has been set to false, delete the Route
			return r.Client.Delete(context.TODO(), route)
		}
		log.Info(grafanaDeprecatedWarning)
		return nil // Route found, do nothing
	}

	//nolint:staticcheck
	if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Route.Enabled {
		return nil // Grafana itself or Route not enabled, do nothing.
	}

	log.Info(grafanaDeprecatedWarning)

	return nil
}

// reconcilePrometheusRoute will ensure that the ArgoCD Prometheus Route is present.
func (r *ReconcileArgoCD) reconcilePrometheusRoute(cr *argoproj.ArgoCD) error {
	route := newRouteWithSuffix("prometheus", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, route.Name, route) {
		if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Route.Enabled {
			// Route exists but enabled flag has been set to false, delete the Route
			return r.Client.Delete(context.TODO(), route)
		}
		return nil // Route found, do nothing
	}

	if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Route.Enabled {
		return nil // Prometheus itself or Route not enabled, do nothing.
	}

	// Allow override of the Annotations for the Route.
	if len(cr.Spec.Prometheus.Route.Annotations) > 0 {
		route.Annotations = cr.Spec.Prometheus.Route.Annotations
	}

	// Allow override of the Labels for the Route.
	if len(cr.Spec.Prometheus.Route.Labels) > 0 {
		labels := route.Labels
		for key, val := range cr.Spec.Prometheus.Route.Labels {
			labels[key] = val
		}
		route.Labels = labels
	}

	// Allow override of the Host for the Route.
	if len(cr.Spec.Prometheus.Host) > 0 {
		route.Spec.Host = cr.Spec.Prometheus.Host // TODO: What additional role needed for this?
	}

	route.Spec.Port = &routev1.RoutePort{
		TargetPort: intstr.FromString("web"),
	}

	// Allow override of TLS options for the Route
	if cr.Spec.Prometheus.Route.TLS != nil {
		err := r.overrideRouteTLS(cr.Spec.Prometheus.Route.TLS, route)
		if err != nil {
			return err
		}
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = "prometheus-operated"

	// Allow override of the WildcardPolicy for the Route
	if cr.Spec.Prometheus.Route.WildcardPolicy != nil && len(*cr.Spec.Prometheus.Route.WildcardPolicy) > 0 {
		route.Spec.WildcardPolicy = *cr.Spec.Prometheus.Route.WildcardPolicy
	}

	if err := controllerutil.SetControllerReference(cr, route, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), route)
}

// reconcileServerRoute will ensure that the ArgoCD Server Route is present.
func (r *ReconcileArgoCD) reconcileServerRoute(cr *argoproj.ArgoCD) error {

	route := newRouteWithSuffix("server", cr)
	found := argoutil.IsObjectFound(r.Client, cr.Namespace, route.Name, route)
	if found {
		if !cr.Spec.Server.Route.Enabled {
			// Route exists but enabled flag has been set to false, delete the Route
			return r.Client.Delete(context.TODO(), route)
		}
	}

	if !cr.Spec.Server.Route.Enabled {
		return nil // Route not enabled, move along...
	}

	// Allow override of the Annotations for the Route.
	if len(cr.Spec.Server.Route.Annotations) > 0 {
		route.Annotations = cr.Spec.Server.Route.Annotations
	}

	// Allow override of the Labels for the Route.
	if len(cr.Spec.Server.Route.Labels) > 0 {
		labels := route.Labels
		for key, val := range cr.Spec.Server.Route.Labels {
			labels[key] = val
		}
		route.Labels = labels
	}

	// Allow override of the Host for the Route.
	if len(cr.Spec.Server.Host) > 0 {
		route.Spec.Host = cr.Spec.Server.Host // TODO: What additional role needed for this?
	}

	hostname, err := shortenHostname(route.Spec.Host)
	if err != nil {
		return err
	}

	route.Spec.Host = hostname

	if cr.Spec.Server.Insecure {
		// Disable TLS and rely on the cluster certificate.
		route.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("http"),
		}
		route.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationEdge,
		}
	} else {
		route.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("https"),
		}

		tlsSecret := &corev1.Secret{}
		isTLSSecretFound := argoutil.IsObjectFound(r.Client, cr.Namespace, common.ArgoCDServerTLSSecretName, tlsSecret)
		// Since Passthrough was the default policy in the previous versions of the operator, we don't want to
		// break users who have already configured a TLS secret for Passthrough.
		// We continue with Passthrough if we find a TLS secret that was manually configured
		// by the user and not by the OpenShift Service CA.
		if cr.Spec.Server.Route.TLS == nil && isTLSSecretFound && !isCreatedByServiceCA(cr.Name, *tlsSecret) {
			route.Spec.TLS = &routev1.TLSConfig{
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
				Termination:                   routev1.TLSTerminationPassthrough,
			}
		} else {
			route.Spec.TLS = &routev1.TLSConfig{
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
				Termination:                   routev1.TLSTerminationReencrypt,
			}
		}
	}

	// Allow override of TLS options for the Route
	if cr.Spec.Server.Route.TLS != nil {
		err := r.overrideRouteTLS(cr.Spec.Server.Route.TLS, route)
		if err != nil {
			return err
		}
	}

	log.Info(fmt.Sprintf("Using %s termination policy for the Server Route", string(route.Spec.TLS.Termination)))

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = nameWithSuffix("server", cr)

	// Allow override of the WildcardPolicy for the Route
	if cr.Spec.Server.Route.WildcardPolicy != nil && len(*cr.Spec.Server.Route.WildcardPolicy) > 0 {
		route.Spec.WildcardPolicy = *cr.Spec.Server.Route.WildcardPolicy
	}

	if err := controllerutil.SetControllerReference(cr, route, r.Scheme); err != nil {
		return err
	}
	if !found {
		return r.Client.Create(context.TODO(), route)
	}
	return r.Client.Update(context.TODO(), route)
}

// isCreatedByServiceCA checks if the secret was created by the OpenShift Service CA
func isCreatedByServiceCA(crName string, secret corev1.Secret) bool {
	serviceName := fmt.Sprintf("%s-%s", crName, "server")
	serviceAnnFound := false
	if secret.Annotations != nil {
		value, ok := secret.Annotations["service.beta.openshift.io/originating-service-name"]
		if ok && value == serviceName {
			serviceAnnFound = true
		}
	}

	if !serviceAnnFound {
		return false
	}

	for _, ref := range secret.OwnerReferences {
		if ref.Kind == "Service" && ref.Name == serviceName {
			return true
		}
	}

	return false
}

// reconcileApplicationSetControllerWebhookRoute will ensure that the ArgoCD Server Route is present.
func (r *ReconcileArgoCD) reconcileApplicationSetControllerWebhookRoute(cr *argoproj.ArgoCD) error {
	name := fmt.Sprintf("%s-%s", common.ApplicationSetServiceNameSuffix, "webhook")
	route := newRouteWithSuffix(name, cr)
	found := argoutil.IsObjectFound(r.Client, cr.Namespace, route.Name, route)
	if found {
		if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.WebhookServer.Route.Enabled {
			// Route exists but enabled flag has been set to false, delete the Route
			return r.Client.Delete(context.TODO(), route)
		}
	}

	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.WebhookServer.Route.Enabled {
		return nil // Route not enabled, move along...
	}

	// Allow override of the Annotations for the Route.
	if len(cr.Spec.ApplicationSet.WebhookServer.Route.Annotations) > 0 {
		route.Annotations = cr.Spec.ApplicationSet.WebhookServer.Route.Annotations
	}

	// Allow override of the Labels for the Route.
	if len(cr.Spec.ApplicationSet.WebhookServer.Route.Labels) > 0 {
		labels := route.Labels
		for key, val := range cr.Spec.ApplicationSet.WebhookServer.Route.Labels {
			labels[key] = val
		}
		route.Labels = labels
	}

	// Allow override of the Host for the Route.
	if len(cr.Spec.ApplicationSet.WebhookServer.Host) > 0 {
		route.Spec.Host = cr.Spec.ApplicationSet.WebhookServer.Host
	}

	hostname, err := shortenHostname(route.Spec.Host)
	if err != nil {
		return err
	}

	route.Spec.Host = hostname

	route.Spec.Port = &routev1.RoutePort{
		TargetPort: intstr.FromString("webhook"),
	}

	// Allow override of TLS options for the Route
	if cr.Spec.ApplicationSet.WebhookServer.Route.TLS != nil {
		tls := &routev1.TLSConfig{}

		// Set Certificate & Key
		routeCopy := route.DeepCopy()
		err := r.overrideRouteTLS(cr.Spec.ApplicationSet.WebhookServer.Route.TLS, routeCopy)
		if err != nil {
			return err
		}
		tls = routeCopy.Spec.TLS

		// Set Termination
		if cr.Spec.ApplicationSet.WebhookServer.Route.TLS.Termination != "" {
			tls.Termination = cr.Spec.ApplicationSet.WebhookServer.Route.TLS.Termination
		} else {
			tls.Termination = routev1.TLSTerminationEdge
		}

		// Set CACertificate
		if cr.Spec.ApplicationSet.WebhookServer.Route.TLS.CACertificate != "" {
			tls.CACertificate = cr.Spec.ApplicationSet.WebhookServer.Route.TLS.CACertificate
		}

		// Set DestinationCACertificate
		if cr.Spec.ApplicationSet.WebhookServer.Route.TLS.DestinationCACertificate != "" {
			tls.DestinationCACertificate = cr.Spec.ApplicationSet.WebhookServer.Route.TLS.DestinationCACertificate
		}

		// Set InsecureEdgeTerminationPolicy
		if cr.Spec.ApplicationSet.WebhookServer.Route.TLS.InsecureEdgeTerminationPolicy != "" {
			tls.InsecureEdgeTerminationPolicy = cr.Spec.ApplicationSet.WebhookServer.Route.TLS.InsecureEdgeTerminationPolicy
		} else {
			tls.InsecureEdgeTerminationPolicy = routev1.InsecureEdgeTerminationPolicyRedirect
		}

		route.Spec.TLS = tls
	} else {
		// Disable TLS and rely on the cluster certificate.
		route.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationEdge,
		}
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = nameWithSuffix(common.ApplicationSetServiceNameSuffix, cr)

	// Allow override of the WildcardPolicy for the Route
	if cr.Spec.ApplicationSet.WebhookServer.Route.WildcardPolicy != nil && len(*cr.Spec.ApplicationSet.WebhookServer.Route.WildcardPolicy) > 0 {
		route.Spec.WildcardPolicy = *cr.Spec.ApplicationSet.WebhookServer.Route.WildcardPolicy
	}

	if err := controllerutil.SetControllerReference(cr, route, r.Scheme); err != nil {
		return err
	}
	if !found {
		return r.Client.Create(context.TODO(), route)
	}
	return r.Client.Update(context.TODO(), route)
}

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

// overrideRouteTLS modifies the Route's TLS settings to match the configurations specified in the ArgoCD CR.
// It updates the Route's TLS configuration either by using the fields directly in the TLSConfig or by referencing
// a Kubernetes TLS secret if provided via the SecretName field.
func (r *ReconcileArgoCD) overrideRouteTLS(tls *argoproj.ArgoCDRouteTLS, route *routev1.Route) error {

	route.Spec.TLS = &tls.TLSConfig
	if tls.TLSConfig.Key != "" || tls.TLSConfig.Certificate != "" {
		// TODO: Emit a Kubernetes event to notify users about the deprecated `.tls.key` and `.tls.certificate` fields.
		// These fields are deprecated in favor of using `.tls.secretName` to reference a Kubernetes TLS secret.
		log.Info("Deprecated: Using `.tls.key` and `.tls.certificate` in ArgoCD CR is not recommended. Use `.tls.secretName` to reference a TLS secret instead.")
	}

	// Populate the Route's `tls.key` and `tls.certificate` fields with data from the specified Kubernetes TLS secret.
	// This secret must be of type `kubernetes.io/tls` and contain `tls.key` and `tls.crt` data.
	// TODO: Investigate use of `route.spec.tls.externalCertificate` to configure external secrets
	// for TLS once this feature reaches GA. See OpenShift documentation:
	// https://docs.openshift.com/container-platform/4.16/networking/routes/secured-routes.html#nw-ingress-route-secret-load-external-cert_secured-routes
	if tls.SecretName != "" {
		secret := &corev1.Secret{}
		err := argoutil.FetchObject(r.Client, route.ObjectMeta.Namespace, tls.SecretName, secret)
		if err != nil {
			return err
		}
		if secret.Type != corev1.SecretTypeTLS {
			return fmt.Errorf("secret %s in namespace %s is not of type kubernetes.io/tls",
				secret.ObjectMeta.Name, secret.ObjectMeta.Namespace)
		}

		// No need to perform further checks on the secret data, as Kubernetes will reject
		// the TLS secret if it does not contain both `tls.key` and `tls.crt` keys.
		route.Spec.TLS.Certificate = string(secret.Data[corev1.TLSCertKey])
		route.Spec.TLS.Key = string(secret.Data[corev1.TLSPrivateKeyKey])
	}

	return nil
}
