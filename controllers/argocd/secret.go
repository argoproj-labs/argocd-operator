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
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	argopass "github.com/argoproj/argo-cd/v2/util/password"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

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

// newCASecret creates a new CA secret with the given suffix for the given ArgoCD.
func newCASecret(cr *argoproj.ArgoCD) (*corev1.Secret, error) {
	secret := argoutil.NewTLSSecret(cr, "ca")

	key, err := argoutil.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	cert, err := argoutil.NewSelfSignedCACertificate(cr.Name, key)
	if err != nil {
		return nil, err
	}

	// This puts both ca.crt and tls.crt into the secret.
	secret.Data = map[string][]byte{
		corev1.TLSCertKey:              argoutil.EncodeCertificatePEM(cert),
		corev1.ServiceAccountRootCAKey: argoutil.EncodeCertificatePEM(cert),
		corev1.TLSPrivateKeyKey:        argoutil.EncodePrivateKeyPEM(key),
	}

	return secret, nil
}

// newCertificateSecret creates a new secret using the given name suffix for the given TLS certificate.
func newCertificateSecret(suffix string, caCert *x509.Certificate, caKey *rsa.PrivateKey, cr *argoproj.ArgoCD) (*corev1.Secret, error) {
	secret := argoutil.NewTLSSecret(cr, suffix)

	key, err := argoutil.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	cfg := &certmanagerv1.CertificateSpec{
		SecretName: secret.Name,
		CommonName: secret.Name,
		Subject: &certmanagerv1.X509Subject{
			Organizations: []string{cr.ObjectMeta.Namespace},
		},
	}

	dnsNames := []string{
		cr.ObjectMeta.Name,
		nameWithSuffix("grpc", cr),
		fmt.Sprintf("%s.%s.svc.cluster.local", cr.ObjectMeta.Name, cr.ObjectMeta.Namespace),
	}

	//lint:ignore SA1019 known to be deprecated
	if cr.Spec.Grafana.Enabled {
		log.Info(grafanaDeprecatedWarning)
	}
	if cr.Spec.Prometheus.Enabled {
		dnsNames = append(dnsNames, getPrometheusHost(cr))
	}

	cert, err := argoutil.NewSignedCertificate(cfg, dnsNames, key, caCert, caKey)
	if err != nil {
		return nil, err
	}

	secret.Data = map[string][]byte{
		corev1.TLSCertKey:       argoutil.EncodeCertificatePEM(cert),
		corev1.TLSPrivateKeyKey: argoutil.EncodePrivateKeyPEM(key),
	}

	return secret, nil
}

// reconcileArgoSecret will ensure that the Argo CD Secret is present.
func (r *ReconcileArgoCD) reconcileArgoSecret(cr *argoproj.ArgoCD) error {
	clusterSecret := argoutil.NewSecretWithSuffix(cr, "cluster")
	secret := argoutil.NewSecretWithName(cr, common.ArgoCDSecretName)

	if !argoutil.IsObjectFound(r.Client, cr.Namespace, clusterSecret.Name, clusterSecret) {
		log.Info(fmt.Sprintf("cluster secret [%s] not found, waiting to reconcile argo secret [%s]", clusterSecret.Name, secret.Name))
		return nil
	}

	tlsSecret := argoutil.NewSecretWithSuffix(cr, "tls")
	if !argoutil.IsObjectFound(r.Client, cr.Namespace, tlsSecret.Name, tlsSecret) {
		log.Info(fmt.Sprintf("tls secret [%s] not found, waiting to reconcile argo secret [%s]", tlsSecret.Name, secret.Name))
		return nil
	}

	if argoutil.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return r.reconcileExistingArgoSecret(cr, secret, clusterSecret, tlsSecret)
	}

	// Secret not found, create it...
	hashedPassword, err := argopass.HashPassword(string(clusterSecret.Data[common.ArgoCDKeyAdminPassword]))
	if err != nil {
		return err
	}

	sessionKey, err := generateArgoServerSessionKey()
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		common.ArgoCDKeyAdminPassword:      []byte(hashedPassword),
		common.ArgoCDKeyAdminPasswordMTime: nowBytes(),
		common.ArgoCDKeyServerSecretKey:    sessionKey,
		common.ArgoCDKeyTLSCert:            tlsSecret.Data[common.ArgoCDKeyTLSCert],
		common.ArgoCDKeyTLSPrivateKey:      tlsSecret.Data[common.ArgoCDKeyTLSPrivateKey],
	}

	if cr.Spec.SSO != nil && cr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeDex {
		dexOIDCClientSecret, err := r.getDexOAuthClientSecret(cr)
		if err != nil {
			return nil
		}
		secret.Data[common.ArgoCDDexSecretKey] = []byte(*dexOIDCClientSecret)
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, secret)
	return r.Client.Create(context.TODO(), secret)
}

// reconcileClusterMainSecret will ensure that the main Secret is present for the Argo CD cluster.
func (r *ReconcileArgoCD) reconcileClusterMainSecret(cr *argoproj.ArgoCD) error {
	secret := argoutil.NewSecretWithSuffix(cr, "cluster")
	if argoutil.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return nil // Secret found, do nothing
	}

	adminPassword, err := generateArgoAdminPassword()
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		common.ArgoCDKeyAdminPassword: adminPassword,
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, secret)
	return r.Client.Create(context.TODO(), secret)
}

// reconcileClusterTLSSecret ensures the TLS Secret is created for the ArgoCD cluster.
func (r *ReconcileArgoCD) reconcileClusterTLSSecret(cr *argoproj.ArgoCD) error {
	secret := argoutil.NewTLSSecret(cr, "tls")
	if argoutil.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return nil // Secret found, do nothing
	}

	caSecret := argoutil.NewSecretWithSuffix(cr, "ca")
	caSecret, err := argoutil.FetchSecret(r.Client, cr.ObjectMeta, caSecret.Name)
	if err != nil {
		return err
	}

	caCert, err := argoutil.ParsePEMEncodedCert(caSecret.Data[corev1.TLSCertKey])
	if err != nil {
		return err
	}

	caKey, err := argoutil.ParsePEMEncodedPrivateKey(caSecret.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		return err
	}

	secret, err = newCertificateSecret("tls", caCert, caKey, cr)
	if err != nil {
		return err
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}

	argoutil.LogResourceCreation(log, secret)
	return r.Client.Create(context.TODO(), secret)
}

// reconcileClusterCASecret ensures the CA Secret is created for the ArgoCD cluster.
func (r *ReconcileArgoCD) reconcileClusterCASecret(cr *argoproj.ArgoCD) error {
	secret := argoutil.NewSecretWithSuffix(cr, "ca")
	if argoutil.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return nil // Secret found, do nothing
	}

	secret, err := newCASecret(cr)
	if err != nil {
		return err
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, secret)
	return r.Client.Create(context.TODO(), secret)
}

// reconcileClusterSecrets will reconcile all Secret resources for the ArgoCD cluster.
func (r *ReconcileArgoCD) reconcileClusterSecrets(cr *argoproj.ArgoCD) error {
	if err := r.reconcileClusterMainSecret(cr); err != nil {
		return err
	}

	if err := r.reconcileRedisInitialPasswordSecret(cr); err != nil {
		return err
	}

	if err := r.reconcileClusterCASecret(cr); err != nil {
		return err
	}

	if err := r.reconcileClusterTLSSecret(cr); err != nil {
		return err
	}

	if err := r.reconcileClusterPermissionsSecret(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaSecret(cr); err != nil {
		return err
	}

	return nil
}

// reconcileExistingArgoSecret will ensure that the Argo CD Secret is up to date.
func (r *ReconcileArgoCD) reconcileExistingArgoSecret(cr *argoproj.ArgoCD, secret *corev1.Secret, clusterSecret *corev1.Secret, tlsSecret *corev1.Secret) error {
	changed := false
	explanation := ""

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}

	if secret.Data[common.ArgoCDKeyServerSecretKey] == nil {
		sessionKey, err := generateArgoServerSessionKey()
		if err != nil {
			return err
		}
		secret.Data[common.ArgoCDKeyServerSecretKey] = sessionKey
	}

	// reset the value to default only when secret.data field is nil
	if hasArgoAdminPasswordChanged(secret, clusterSecret) {
		pwBytes, ok := clusterSecret.Data[common.ArgoCDKeyAdminPassword]
		if ok && secret.Data[common.ArgoCDKeyAdminPassword] == nil {
			hashedPassword, err := argopass.HashPassword(strings.TrimRight(string(pwBytes), "\n"))
			if err != nil {
				return err
			}

			secret.Data[common.ArgoCDKeyAdminPassword] = []byte(hashedPassword)
			secret.Data[common.ArgoCDKeyAdminPasswordMTime] = nowBytes()
			explanation = "argo admin password"
			changed = true
		}
	}

	if hasArgoTLSChanged(secret, tlsSecret) {
		secret.Data[common.ArgoCDKeyTLSCert] = tlsSecret.Data[common.ArgoCDKeyTLSCert]
		secret.Data[common.ArgoCDKeyTLSPrivateKey] = tlsSecret.Data[common.ArgoCDKeyTLSPrivateKey]
		if changed {
			explanation += ", "
		}
		explanation += "argo tls secret"
		changed = true
	}

	if cr.Spec.SSO != nil && cr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeDex {
		dexOIDCClientSecret, err := r.getDexOAuthClientSecret(cr)
		if err != nil {
			return err
		}
		actual := string(secret.Data[common.ArgoCDDexSecretKey])
		if dexOIDCClientSecret != nil {
			expected := *dexOIDCClientSecret
			if actual != expected {
				secret.Data[common.ArgoCDDexSecretKey] = []byte(*dexOIDCClientSecret)
				if changed {
					explanation += ", "
				}
				explanation += "argo dex secret"
				changed = true
			}
		}
	}

	if changed {
		argoutil.LogResourceUpdate(log, secret, "updating", explanation)
		if err := r.Client.Update(context.TODO(), secret); err != nil {
			return err
		}
	}

	return nil
}

// reconcileGrafanaSecret will ensure that the Grafana Secret is present.
func (r *ReconcileArgoCD) reconcileGrafanaSecret(cr *argoproj.ArgoCD) error {
	//lint:ignore SA1019 known to be deprecated
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	log.Info(grafanaDeprecatedWarning)

	return nil
}

// generateSortedManagedNamespaceListForArgoCDCR return a list of namespaces with 'managed-by' label that are managed by this 'cr', and including the namespace containing 'cr'
func generateSortedManagedNamespaceListForArgoCDCR(cr *argoproj.ArgoCD, rClient client.Client) ([]string, error) {
	namespaceList := corev1.NamespaceList{}
	listOption := client.MatchingLabels{
		common.ArgoCDManagedByLabel: cr.Namespace,
	}
	if err := rClient.List(context.TODO(), &namespaceList, listOption); err != nil {
		return nil, err
	}

	var namespaces []string
	for _, namespace := range namespaceList.Items {
		namespaces = append(namespaces, namespace.Name)
	}

	if !containsString(namespaces, cr.Namespace) {
		namespaces = append(namespaces, cr.Namespace)
	}
	sort.Strings(namespaces)
	return namespaces, nil
}

// combineClusterSecretNamespacesWithManagedNamespaces will combine the contents of clusterSecret's .data.namespaces value, with the list of namespaces in 'managedNamespaceList', sorting them and removing duplicates.
func combineClusterSecretNamespacesWithManagedNamespaces(clusterSecret corev1.Secret, managedNamespaceList []string) string {
	namespacesToManageMap := map[string]string{}

	for _, managedNamespace := range managedNamespaceList {
		namespacesToManageMap[managedNamespace] = managedNamespace
	}

	clusterSecretNamespaces := strings.Split(string(clusterSecret.Data["namespaces"]), ",")
	for _, clusterSecretNS := range clusterSecretNamespaces {
		ns := strings.TrimSpace(clusterSecretNS)
		namespacesToManageMap[ns] = ns
	}

	namespacesToManageList := []string{}
	for namespaceToManage := range namespacesToManageMap {
		namespacesToManageList = append(namespacesToManageList, namespaceToManage)
	}
	sort.Strings(namespacesToManageList)

	namespacesToManageString := strings.Join(namespacesToManageList, ",")

	return namespacesToManageString

}

// reconcileClusterPermissionsSecret ensures ArgoCD instance is namespace-scoped
func (r *ReconcileArgoCD) reconcileClusterPermissionsSecret(cr *argoproj.ArgoCD) error {

	managedNamespaceList, err := generateSortedManagedNamespaceListForArgoCDCR(cr, r.Client)
	if err != nil {
		return err
	}

	// isArgoCDAClusterConfigInstance indicates whether 'cr' is a cluster config instance (mentioned in ARGOCD_CLUSTER_CONFIG_NAMESPACES)
	var isArgoCDAClusterConfigInstance bool

	if allowedNamespace(cr.Namespace, os.Getenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")) {
		isArgoCDAClusterConfigInstance = true
	}

	// Get all existing cluster secrets in the namespace
	clusterSecrets, err := r.getClusterSecrets(cr)
	if err != nil {
		return err
	}

	// Find the cluster secret in the list that points to  common.ArgoCDDefaultServer (default server address)
	var localClusterSecret *corev1.Secret
	for x, clusterSecret := range clusterSecrets.Items {

		// check if cluster secret with default server address exists
		if string(clusterSecret.Data["server"]) == common.ArgoCDDefaultServer {
			localClusterSecret = &clusterSecrets.Items[x]
		}
	}

	if localClusterSecret != nil {

		// If the default Cluster Secret already exists

		secretUpdateRequired := false

		// if the cluster belongs to cluster config namespace,
		// remove all namespaces from cluster secret,
		// else update the list of namespaces if value differs.
		var explanation string
		if isArgoCDAClusterConfigInstance {

			if _, exists := localClusterSecret.Data["namespaces"]; exists {
				delete(localClusterSecret.Data, "namespaces")
				explanation = "removing namespaces from cluster secret"
				secretUpdateRequired = true
			}

		} else {

			namespacesToManageString := combineClusterSecretNamespacesWithManagedNamespaces(*localClusterSecret, managedNamespaceList)

			var existingNamespacesValue string
			if localClusterSecret.Data["namespaces"] != nil {
				existingNamespacesValue = string(localClusterSecret.Data["namespaces"])
			}

			if existingNamespacesValue != namespacesToManageString {
				localClusterSecret.Data["namespaces"] = []byte(namespacesToManageString)
				explanation = "updating namespaces in cluster secret"
				secretUpdateRequired = true
			}
		}

		if secretUpdateRequired {
			// We found the Secret, and the field needs to be updated
			argoutil.LogResourceUpdate(log, localClusterSecret, explanation)
			return r.Client.Update(context.TODO(), localClusterSecret)
		}

		// We found the Secret, but the field hasn't changed: no update needed.
		return nil
	}

	// If ArgoCD is configured as a cluster-scoped, no need to create a Namespace containing managed namespaces
	if isArgoCDAClusterConfigInstance {
		// do nothing
		return nil
	}

	// Create the Secret, since we could not find it above
	secret := argoutil.NewSecretWithSuffix(cr, "default-cluster-config")
	secret.Labels[common.ArgoCDSecretTypeLabel] = "cluster"
	dataBytes, _ := json.Marshal(map[string]interface{}{
		"tlsClientConfig": map[string]interface{}{
			"insecure": false,
		},
	})

	secret.Data = map[string][]byte{
		"config":     dataBytes,
		"name":       []byte("in-cluster"),
		"server":     []byte(common.ArgoCDDefaultServer),
		"namespaces": []byte(strings.Join(managedNamespaceList, ",")),
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, secret)
	return r.Client.Create(context.TODO(), secret)
}

// reconcileRepoServerTLSSecret checks whether the argocd-repo-server-tls secret
// has changed since our last reconciliation loop. It does so by comparing the
// checksum of tls.crt and tls.key in the status of the ArgoCD CR against the
// values calculated from the live state in the cluster.
func (r *ReconcileArgoCD) reconcileRepoServerTLSSecret(cr *argoproj.ArgoCD) error {
	var tlsSecretObj corev1.Secret
	var sha256sum string

	log.Info("reconciling repo-server TLS secret")

	tlsSecretName := types.NamespacedName{Namespace: cr.Namespace, Name: common.ArgoCDRepoServerTLSSecretName}
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
	if cr.Status.RepoTLSChecksum != sha256sum {
		// We store the value early to prevent a possible restart loop, for the
		// cost of a possibly missed restart when we cannot update the status
		// field of the resource.
		cr.Status.RepoTLSChecksum = sha256sum
		err = r.Client.Status().Update(context.TODO(), cr)
		if err != nil {
			return err
		}

		// Trigger rollout of API server
		apiDepl := newDeploymentWithSuffix("server", "server", cr)
		err = r.triggerRollout(apiDepl, "repo.tls.cert.changed")
		if err != nil {
			return err
		}

		// Trigger rollout of repository server
		repoDepl := newDeploymentWithSuffix("repo-server", "repo-server", cr)
		err = r.triggerRollout(repoDepl, "repo.tls.cert.changed")
		if err != nil {
			return err
		}

		// Trigger rollout of application controller
		controllerSts := newStatefulSetWithSuffix("application-controller", "application-controller", cr)
		err = r.triggerRollout(controllerSts, "repo.tls.cert.changed")
		if err != nil {
			return err
		}
	}

	return nil
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
				argoutil.LogResourceDeletion(log, redisSts, "to trigger pods to restart")
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

// reconcileSecrets will reconcile all ArgoCD Secret resources.
func (r *ReconcileArgoCD) reconcileSecrets(cr *argoproj.ArgoCD) error {
	if err := r.reconcileClusterSecrets(cr); err != nil {
		return err
	}

	if err := r.reconcileArgoSecret(cr); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileArgoCD) getClusterSecrets(cr *argoproj.ArgoCD) (*corev1.SecretList, error) {

	clusterSecrets := &corev1.SecretList{}
	opts := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			common.ArgoCDSecretTypeLabel: "cluster",
		}),
		Namespace: cr.Namespace,
	}

	if err := r.Client.List(context.TODO(), clusterSecrets, opts); err != nil {
		return nil, err
	}

	return clusterSecrets, nil
}

// reconcileRedisInitialPasswordSecret will ensure that the redis Secret is present for the cluster.
func (r *ReconcileArgoCD) reconcileRedisInitialPasswordSecret(cr *argoproj.ArgoCD) error {
	secret := argoutil.NewSecretWithSuffix(cr, "redis-initial-password")
	if argoutil.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return nil // Secret found, do nothing
	}

	redisInitialPassword, err := generateRedisAdminPassword()
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		"immutable":                   []byte("true"),
		common.ArgoCDKeyAdminPassword: redisInitialPassword,
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, secret)
	return r.Client.Create(context.TODO(), secret)
}
