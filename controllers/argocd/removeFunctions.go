package argocd

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	json "encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/secret"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	argopass "github.com/argoproj/argo-cd/v2/util/password"
	tlsutil "github.com/operator-framework/operator-sdk/pkg/tls"
	"github.com/sethvargo/go-password/password"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ArgoCDReconciler) getClusterSecrets(cr *argoproj.ArgoCD) (*corev1.SecretList, error) {

	clusterSecrets := &corev1.SecretList{}
	opts := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			common.ArgoCDArgoprojKeySecretType: "cluster",
		}),
		Namespace: cr.Namespace,
	}

	if err := r.Client.List(context.TODO(), clusterSecrets, opts); err != nil {
		return nil, err
	}

	return clusterSecrets, nil
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
	actualCert := string(actual.Data[corev1.TLSCertKey])
	actualKey := string(actual.Data[corev1.TLSPrivateKeyKey])
	expectedCert := string(expected.Data[corev1.TLSCertKey])
	expectedKey := string(expected.Data[corev1.TLSPrivateKeyKey])

	if actualCert != expectedCert || actualKey != expectedKey {
		log.Info("tls secret has changed")
		return true
	}
	return false
}

// reconcileClusterMainSecret will ensure that the main Secret is present for the Argo CD cluster.
func (r *ArgoCDReconciler) reconcileClusterMainSecret(cr *argoproj.ArgoCD) error {
	secret := util.NewSecretWithSuffix(cr, "cluster")
	if util.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
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
	return r.Client.Create(context.TODO(), secret)
}

// newCASecret creates a new CA secret with the given suffix for the given ArgoCD.
func newCASecret(cr *argoproj.ArgoCD) (*corev1.Secret, error) {
	secret := util.NewTLSSecret(cr, "ca")

	key, err := util.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	cert, err := util.NewSelfSignedCACertificate(cr.Name, key)
	if err != nil {
		return nil, err
	}

	// This puts both ca.crt and tls.crt into the secret.
	secret.Data = map[string][]byte{
		corev1.TLSCertKey:              util.EncodeCertificatePEM(cert),
		corev1.ServiceAccountRootCAKey: util.EncodeCertificatePEM(cert),
		corev1.TLSPrivateKeyKey:        util.EncodePrivateKeyPEM(key),
	}

	return secret, nil
}

// reconcileClusterCASecret ensures the CA Secret is created for the ArgoCD cluster.
func (r *ArgoCDReconciler) reconcileClusterCASecret(cr *argoproj.ArgoCD) error {
	secret := util.NewSecretWithSuffix(cr, "ca")
	if util.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return nil // Secret found, do nothing
	}

	secret, err := newCASecret(cr)
	if err != nil {
		return err
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), secret)
}

// reconcileClusterTLSSecret ensures the TLS Secret is created for the ArgoCD cluster.
func (r *ArgoCDReconciler) reconcileClusterTLSSecret(cr *argoproj.ArgoCD) error {
	secret := util.NewTLSSecret(cr, "tls")
	if util.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return nil // Secret found, do nothing
	}

	caSecret := util.NewSecretWithSuffix(cr, "ca")
	caSecret, err := util.FetchSecret(r.Client, cr.ObjectMeta, caSecret.Name)
	if err != nil {
		return err
	}

	caCert, err := util.ParsePEMEncodedCert(caSecret.Data[corev1.TLSCertKey])
	if err != nil {
		return err
	}

	caKey, err := util.ParsePEMEncodedPrivateKey(caSecret.Data[corev1.TLSPrivateKeyKey])
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

	return r.Client.Create(context.TODO(), secret)
}

// newCertificateSecret creates a new secret using the given name suffix for the given TLS certificate.
func newCertificateSecret(suffix string, caCert *x509.Certificate, caKey *rsa.PrivateKey, cr *argoproj.ArgoCD) (*corev1.Secret, error) {
	secret := util.NewTLSSecret(cr, suffix)

	key, err := util.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	cfg := &tlsutil.CertConfig{
		CertName:     secret.Name,
		CertType:     tlsutil.ClientAndServingCert,
		CommonName:   secret.Name,
		Organization: []string{cr.ObjectMeta.Namespace},
	}

	dnsNames := []string{
		cr.ObjectMeta.Name,
		util.NameWithSuffix(cr.Name, "grpc"),
		fmt.Sprintf("%s.%s.svc.cluster.local", cr.ObjectMeta.Name, cr.ObjectMeta.Namespace),
	}

	if cr.Spec.Grafana.Enabled {
		dnsNames = append(dnsNames, getGrafanaHost(cr))
	}
	if cr.Spec.Prometheus.Enabled {
		dnsNames = append(dnsNames, getPrometheusHost(cr))
	}

	cert, err := util.NewSignedCertificate(cfg, dnsNames, key, caCert, caKey)
	if err != nil {
		return nil, err
	}

	secret.Data = map[string][]byte{
		corev1.TLSCertKey:       util.EncodeCertificatePEM(cert),
		corev1.TLSPrivateKeyKey: util.EncodePrivateKeyPEM(key),
	}

	return secret, nil
}

// reconcileClusterPermissionsSecret ensures ArgoCD instance is namespace-scoped
func (r *ArgoCDReconciler) reconcileClusterPermissionsSecret(cr *argoproj.ArgoCD) error {
	var clusterConfigInstance bool
	secret := util.NewSecretWithSuffix(cr, "default-cluster-config")
	secret.Labels[common.ArgoCDArgoprojKeySecretType] = "cluster"
	dataBytes, _ := json.Marshal(map[string]interface{}{
		"tlsClientConfig": map[string]interface{}{
			"insecure": false,
		},
	})

	namespaceList := corev1.NamespaceList{}
	listOption := client.MatchingLabels{
		common.ArgoCDArgoprojKeyManagedBy: cr.Namespace,
	}
	if err := r.Client.List(context.TODO(), &namespaceList, listOption); err != nil {
		return err
	}

	var namespaces []string
	for _, namespace := range namespaceList.Items {
		namespaces = append(namespaces, namespace.Name)
	}

	if !util.ContainsString(namespaces, cr.Namespace) {
		namespaces = append(namespaces, cr.Namespace)
	}
	sort.Strings(namespaces)

	secret.Data = map[string][]byte{
		"config":     dataBytes,
		"name":       []byte("in-cluster"),
		"server":     []byte(common.ArgoCDDefaultServer),
		"namespaces": []byte(strings.Join(namespaces, ",")),
	}

	if allowedNamespace(cr.Namespace, os.Getenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")) {
		clusterConfigInstance = true
	}

	clusterSecrets, err := r.getClusterSecrets(cr)
	if err != nil {
		return err
	}

	for _, s := range clusterSecrets.Items {
		// check if cluster secret with default server address exists
		if string(s.Data["server"]) == common.ArgoCDDefaultServer {
			// if the cluster belongs to cluster config namespace,
			// remove all namespaces from cluster secret,
			// else update the list of namespaces if value differs.
			if clusterConfigInstance {
				delete(s.Data, "namespaces")
			} else {
				ns := strings.Split(string(s.Data["namespaces"]), ",")
				for _, n := range namespaces {
					if !util.ContainsString(ns, strings.TrimSpace(n)) {
						ns = append(ns, strings.TrimSpace(n))
					}
				}
				sort.Strings(ns)
				s.Data["namespaces"] = []byte(strings.Join(ns, ","))
			}
			return r.Client.Update(context.TODO(), &s)
		}
	}

	if clusterConfigInstance {
		// do nothing
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), secret)
}

// reconcileArgoSecret will ensure that the Argo CD Secret is present.
func (r *ArgoCDReconciler) reconcileArgoSecret(cr *argoproj.ArgoCD) error {
	clusterSecret := util.NewSecretWithSuffix(cr, "cluster")
	secret := util.NewSecretWithName(cr, secret.ArgoCDSecretName)

	if !util.IsObjectFound(r.Client, cr.Namespace, clusterSecret.Name, clusterSecret) {
		log.Info(fmt.Sprintf("cluster secret [%s] not found, waiting to reconcile argo secret [%s]", clusterSecret.Name, secret.Name))
		return nil
	}

	tlsSecret := util.NewSecretWithSuffix(cr, "tls")
	if !util.IsObjectFound(r.Client, cr.Namespace, tlsSecret.Name, tlsSecret) {
		log.Info(fmt.Sprintf("tls secret [%s] not found, waiting to reconcile argo secret [%s]", tlsSecret.Name, secret.Name))
		return nil
	}

	if util.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
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
		common.ArgoCDKeyAdminPasswordMTime: util.NowBytes(),
		common.ArgoCDKeyServerSecretKey:    sessionKey,
		corev1.TLSCertKey:                  tlsSecret.Data[corev1.TLSCertKey],
		corev1.TLSPrivateKeyKey:            tlsSecret.Data[corev1.TLSPrivateKeyKey],
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
	return r.Client.Create(context.TODO(), secret)
}

// reconcileExistingArgoSecret will ensure that the Argo CD Secret is up to date.
func (r *ArgoCDReconciler) reconcileExistingArgoSecret(cr *argoproj.ArgoCD, secret *corev1.Secret, clusterSecret *corev1.Secret, tlsSecret *corev1.Secret) error {
	changed := false

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

	if hasArgoAdminPasswordChanged(secret, clusterSecret) {
		pwBytes, ok := clusterSecret.Data[common.ArgoCDKeyAdminPassword]
		if ok {
			hashedPassword, err := argopass.HashPassword(strings.TrimRight(string(pwBytes), "\n"))
			if err != nil {
				return err
			}

			secret.Data[common.ArgoCDKeyAdminPassword] = []byte(hashedPassword)
			secret.Data[common.ArgoCDKeyAdminPasswordMTime] = util.NowBytes()
			changed = true
		}
	}

	if hasArgoTLSChanged(secret, tlsSecret) {
		secret.Data[corev1.TLSCertKey] = tlsSecret.Data[corev1.TLSCertKey]
		secret.Data[corev1.TLSPrivateKeyKey] = tlsSecret.Data[corev1.TLSPrivateKeyKey]
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
				changed = true
			}
		}
	}

	if changed {
		log.Info("updating argo secret")
		if err := r.Client.Update(context.TODO(), secret); err != nil {
			return err
		}
	}

	return nil
}

// reconcileClusterSecrets will reconcile all Secret resources for the ArgoCD cluster.
func (r *ArgoCDReconciler) reconcileClusterSecrets(cr *argoproj.ArgoCD) error {
	if err := r.reconcileClusterMainSecret(cr); err != nil {
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

// reconcileSecrets will reconcile all ArgoCD Secret resources.
func (r *ArgoCDReconciler) reconcileSecrets(cr *argoproj.ArgoCD) error {
	if err := r.reconcileClusterSecrets(cr); err != nil {
		return err
	}

	if err := r.reconcileArgoSecret(cr); err != nil {
		return err
	}

	return nil
}
