package secret

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	argopass "github.com/argoproj/argo-cd/v2/util/password"
	"github.com/go-logr/logr"
	tlsutil "github.com/operator-framework/operator-sdk/pkg/tls"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	amerr "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type SecretReconciler struct {
	Client            client.Client
	Scheme            *runtime.Scheme
	Instance          *argoproj.ArgoCD
	ClusterScoped     bool
	Logger            logr.Logger
	ManagedNamespaces map[string]string
}

func (sr *SecretReconciler) Reconcile() error {
	var reconciliationErrors []error
	sr.Logger = ctrl.Log.WithName(SecretsControllerName).WithValues("instance", sr.Instance.Name, "instance-namespace", sr.Instance.Namespace)

	if err := sr.reconcileCredentialsSecret(); err != nil {
		reconciliationErrors = append(reconciliationErrors, err)
	}

	if err := sr.reconcileClusterPermissionsSecret(); err != nil {
		reconciliationErrors = append(reconciliationErrors, err)
	}

	if err := sr.reconcileCASecret(); err != nil {
		return err
	}

	if err := sr.reconcileTLSSecret(); err != nil {
		return err
	}

	if err := sr.reconcileArgoCDSecret(); err != nil {
		return err
	}

	// TO DO : skipping grafana for now - it should either be in its own controller
	// or preferably skipped altogether by the operator

	return amerr.NewAggregate(reconciliationErrors)
}

// reconcileArgoCDSecret creates and updates the "argocd-secret" secret for the given
// Argo CD instance. It requires the credentials secret, CA secret and TLS secret
// to be present on the cluster already
func (sr *SecretReconciler) reconcileArgoCDSecret() error {
	credSecretName := util.NameWithSuffix(sr.Instance.Name, DefaultClusterCredentialsSuffix)
	credsSecret, err := workloads.GetSecret(credSecretName, sr.Instance.Namespace, sr.Client)
	if err != nil {
		sr.Logger.Error(err, "reconcileArgoCDSecret: failed to retrieve secret", "name", credSecretName, "namespace", sr.Instance.Namespace)
		return err
	}

	tlsSecretName := util.NameWithSuffix(sr.Instance.Name, common.ArgoCDTLSSuffix)
	tlsSecret, err := workloads.GetSecret(tlsSecretName, sr.Instance.Namespace, sr.Client)
	if err != nil {
		sr.Logger.Error(err, "reconcileArgoCDSecret: failed to retrieve secret", "name", tlsSecretName, "namespace", sr.Instance.Namespace)
		return err
	}

	argocdSecretTmpl := sr.getDesiredSecretTmplObj(ArgoCDSecretName)
	hashedPassword, err := argopass.HashPassword(string(credsSecret.Data[common.ArgoCDKeyAdminPassword]))
	if err != nil {
		sr.Logger.Error(err, "reconcileArgoCDSecret: failed to hash admin credentials")
		return err
	}
	sessionKey, err := argocdcommon.GenerateArgoServerSessionKey()
	if err != nil {
		sr.Logger.Error(err, "reconcileArgoCDSecret: failed to generate session key")
		return err
	}
	argocdSecretTmpl.Data = map[string][]byte{
		common.ArgoCDKeyAdminPassword:      []byte(hashedPassword),
		common.ArgoCDKeyAdminPasswordMTime: util.NowBytes(),
		common.ArgoCDKeyServerSecretKey:    sessionKey,
		corev1.TLSCertKey:                  tlsSecret.Data[corev1.TLSCertKey],
		corev1.TLSPrivateKeyKey:            tlsSecret.Data[corev1.TLSPrivateKeyKey],
	}

	// TO DO: Let dex controller populate dex oauth client secret information instead of doing it here

	existingArgoCDSecret, err := workloads.GetSecret(ArgoCDSecretName, sr.Instance.Namespace, sr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			sr.Logger.Error(err, "reconcileArgoCDSecret: failed to retrieve secret", "name", ArgoCDSecretName, "namespace", sr.Instance.Namespace)
			return err
		}

		argocdSecretReq := sr.getSecretRequest(*argocdSecretTmpl)
		argocdSecret, err := workloads.RequestSecret(argocdSecretReq)
		if err != nil {
			sr.Logger.Error(err, "reconcileArgoCDSecret: failed to request secret", "name", ArgoCDSecretName, "namespace", sr.Instance.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(sr.Instance, argocdSecret, sr.Scheme); err != nil {
			sr.Logger.Error(err, "reconcileClusterPermissionsSecret: failed to set owner reference for secret", "name", argocdSecret.Name, "namespace", argocdSecret.Namespace)
		}

		if err = workloads.CreateSecret(argocdSecret, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileClusterPermissionsSecret: failed to create secret", "name", argocdSecret.Name, "namespace", argocdSecret.Namespace)
			return err
		}
		sr.Logger.V(0).Info("reconcileClusterPermissionsSecret: secret created", "name", argocdSecret.Name, "namespace", argocdSecret.Namespace)
		return nil
	}

	argocdSecretChanged := false
	if existingArgoCDSecret.Data == nil {
		existingArgoCDSecret.Data = make(map[string][]byte)
	}
	if existingArgoCDSecret.Data[common.ArgoCDKeyServerSecretKey] == nil {
		sessionKey, err := argocdcommon.GenerateArgoServerSessionKey()
		if err != nil {
			sr.Logger.Error(err, "reconcileArgoCDSecret: failed to generate session key")
			return err
		}
		existingArgoCDSecret.Data[common.ArgoCDKeyServerSecretKey] = sessionKey
		argocdSecretChanged = true
	}

	if ArgoAdminPasswordChanged(existingArgoCDSecret, credsSecret) {
		sr.Logger.V(1).Info("admin password changed, updating secret", "name", existingArgoCDSecret.Name, "namespace", existingArgoCDSecret.Namespace)
		pwBytes, ok := credsSecret.Data[common.ArgoCDKeyAdminPassword]
		if ok {
			hashedPassword, err := argopass.HashPassword(strings.TrimRight(string(pwBytes), "\n"))
			if err != nil {
				sr.Logger.Error(err, "reconcileArgoCDSecret: failed to hash admin credentials")
				return err
			}
			existingArgoCDSecret.Data[common.ArgoCDKeyAdminPassword] = []byte(hashedPassword)
			existingArgoCDSecret.Data[common.ArgoCDKeyAdminPasswordMTime] = util.NowBytes()
			argocdSecretChanged = true
		}
	}

	if ArgoTLSChanged(existingArgoCDSecret, tlsSecret) {
		sr.Logger.V(1).Info("TLS key or certificate changed, updating secret", "name", existingArgoCDSecret.Name, "namespace", existingArgoCDSecret.Namespace)
		existingArgoCDSecret.Data[corev1.TLSCertKey] = tlsSecret.Data[corev1.TLSCertKey]
		existingArgoCDSecret.Data[corev1.TLSPrivateKeyKey] = tlsSecret.Data[corev1.TLSPrivateKeyKey]
		argocdSecretChanged = true
	}

	// TO DO: Let dex controller populate dex oauth client secret information instead of doing it here

	if argocdSecretChanged {
		if err = workloads.UpdateSecret(existingArgoCDSecret, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileArgoCDSecret: failed to update secret", "name", existingArgoCDSecret.Name, "namespace", existingArgoCDSecret.Namespace)
			return err
		}
		sr.Logger.V(0).Info("reconcileArgoCDSecret: secret updated", "name", existingArgoCDSecret.Name, "namespace", existingArgoCDSecret.Namespace)
		return nil
	}

	// nothing to do
	return nil
}

// reconcileClusterPermissionsSecret creates and updates the cluster secret for the host
// cluster of the given Argo CD instance ("in-cluster" secret). It updates the list
// of managed namespaces depending on the scope of the instance
func (sr *SecretReconciler) reconcileClusterPermissionsSecret() error {
	clusterPermSecretName := util.NameWithSuffix(sr.Instance.Name, DefaultClusterConfigSuffix)

	managedNsList, _ := util.ConvertMapToSlices(sr.ManagedNamespaces)
	sort.Strings(managedNsList)

	existingSecret, err := workloads.GetSecret(clusterPermSecretName, sr.Instance.Namespace, sr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			sr.Logger.Error(err, "reconcileClusterPermissionsSecret: failed to retrieve secret", "name", clusterPermSecretName, "namespace", sr.Instance.Namespace)
			return err
		}

		clusterPermSecretTmpl := sr.getDesiredSecretTmplObj(clusterPermSecretName)
		clusterPermSecretTmpl.Labels[common.ArgoCDArgoprojKeySecretType] = ClusterSecretType
		dataBytes, _ := json.Marshal(map[string]interface{}{
			"tlsClientConfig": map[string]interface{}{
				"insecure": false,
			},
		})

		clusterPermSecretTmpl.Data = map[string][]byte{
			"config":     dataBytes,
			"name":       []byte(InClusterSecretName),
			"server":     []byte(common.ArgoCDDefaultServer),
			"namespaces": []byte(strings.Join(managedNsList, ",")),
		}
		if sr.ClusterScoped {
			delete(clusterPermSecretTmpl.Data, "namespaces")
		}
		clusterPermSecretReq := sr.getSecretRequest(*clusterPermSecretTmpl)
		clusterPermSecret, err := workloads.RequestSecret(clusterPermSecretReq)
		if err != nil {
			sr.Logger.Error(err, "reconcileClusterPermissionsSecret: failed to request secret", "name", clusterPermSecretName, "namespace", sr.Instance.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(sr.Instance, clusterPermSecret, sr.Scheme); err != nil {
			sr.Logger.Error(err, "reconcileClusterPermissionsSecret: failed to set owner reference for secret", "name", clusterPermSecret.Name, "namespace", clusterPermSecret.Namespace)
		}

		if err = workloads.CreateSecret(clusterPermSecret, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileClusterPermissionsSecret: failed to create secret", "name", clusterPermSecret.Name, "namespace", clusterPermSecret.Namespace)
			return err
		}
		sr.Logger.V(0).Info("reconcileClusterPermissionsSecret: secret created", "name", clusterPermSecret.Name, "namespace", clusterPermSecret.Namespace)
		return nil
	}

	clusterPermSecretChanged := false

	if sr.ClusterScoped {
		delete(existingSecret.Data, "namespaces")
		clusterPermSecretChanged = true
	} else {
		nsListChanged := false
		newNsList := []string{}

		if existingNsListBytes, ok := existingSecret.Data["namespaces"]; ok {
			existingNsList := strings.Split(string(existingNsListBytes), ",")
			sort.Strings(existingNsList)
			updatedNsList, _ := argocdcommon.UpdateIfChangedSlice(existingNsList, managedNsList, nil, &nsListChanged)
			if updatedList, ok := updatedNsList.([]string); ok {
				sort.Strings(updatedList)
				newNsList = updatedList
			}
		} else {
			newNsList = managedNsList
			nsListChanged = true
		}

		if nsListChanged {
			sort.Strings(newNsList)
			existingSecret.Data["namespaces"] = []byte(strings.Join(newNsList, ","))
			clusterPermSecretChanged = true
		}
	}

	if clusterPermSecretChanged {
		if err = workloads.UpdateSecret(existingSecret, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileClusterPermissionsSecret: failed to update secret", "name", existingSecret.Name, "namespace", existingSecret.Namespace)
			return err
		}
		sr.Logger.V(0).Info("reconcileClusterPermissionsSecret: secret updated", "name", existingSecret.Name, "namespace", existingSecret.Namespace)
		return nil
	}

	// nothing to do
	return nil
}

// reconcileCredentialsSecret will ensure that the default Argo CD admin credentials Secret is always present on the cluster.
func (sr *SecretReconciler) reconcileCredentialsSecret() error {
	credSecretName := util.NameWithSuffix(sr.Instance.Name, DefaultClusterCredentialsSuffix)

	_, err := workloads.GetSecret(credSecretName, sr.Instance.Namespace, sr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			sr.Logger.Error(err, "reconcileClusterCredentialsSecret: failed to retrieve secret", "name", credSecretName, "namespace", sr.Instance.Namespace)
			return err
		}

		adminPassword, err := argocdcommon.GenerateArgoAdminPassword()
		if err != nil {
			sr.Logger.Error(err, "reconcileClusterCredentialsSecret: failed to generate admin password")
			return err
		}

		credSecretTmpl := sr.getDesiredSecretTmplObj(credSecretName)
		credSecretTmpl.Type = corev1.SecretTypeOpaque
		credSecretTmpl.Data = map[string][]byte{
			common.ArgoCDKeyAdminPassword: adminPassword,
		}
		secretReq := sr.getSecretRequest(*credSecretTmpl)
		clusterCredsSecret, err := workloads.RequestSecret(secretReq)
		if err != nil {
			sr.Logger.Error(err, "reconcileClusterCredentialsSecret: failed to request secret", "name", credSecretName, "namespace", sr.Instance.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(sr.Instance, clusterCredsSecret, sr.Scheme); err != nil {
			sr.Logger.Error(err, "reconcileClusterCredentialsSecret: failed to set owner reference for secret", "name", clusterCredsSecret.Name, "namespace", clusterCredsSecret.Namespace)
		}

		if err = workloads.CreateSecret(clusterCredsSecret, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileClusterCredentialsSecret: failed to create secret", "name", clusterCredsSecret.Name, "namespace", clusterCredsSecret.Namespace)
			return err
		}
		sr.Logger.V(0).Info("reconcileClusterCredentialsSecret: secret created", "name", clusterCredsSecret.Name, "namespace", clusterCredsSecret.Namespace)
		return nil
	}

	// secret exists, nothing to do
	return nil
}

// reconcileCASecret ensures that CA secret is always present on the cluster
func (sr *SecretReconciler) reconcileCASecret() error {
	caSecretName := util.NameWithSuffix(sr.Instance.Name, common.ArgoCDCASuffix)

	_, err := workloads.GetSecret(caSecretName, sr.Instance.Namespace, sr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			sr.Logger.Error(err, "reconcileClusterCASecret: failed to retrieve secret", "name", caSecretName, "namespace", sr.Instance.Namespace)
			return err
		}

		caSecret, err := sr.getDesiredCASecret(caSecretName)
		if err != nil {
			sr.Logger.Error(err, "reconcileClusterCASecret: failed to generate secret", "name", caSecretName)
			return err
		}

		if err = controllerutil.SetControllerReference(sr.Instance, caSecret, sr.Scheme); err != nil {
			sr.Logger.Error(err, "reconcileClusterCASecret: failed to set owner reference for secret", "name", caSecret.Name, "namespace", caSecret.Namespace)
		}

		if err = workloads.CreateSecret(caSecret, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileClusterCASecret: failed to create secret", "name", caSecret.Name, "namespace", caSecret.Namespace)
			return err
		}
		sr.Logger.V(0).Info("reconcileClusterCASecret: secret created", "name", caSecret.Name, "namespace", caSecret.Namespace)
		return nil
	}

	// secret exists, nothing to do
	return nil
}

// reconcileTLSSecret ensures that cluster TLS secret is always present on the cluster
func (sr *SecretReconciler) reconcileTLSSecret() error {
	tlsSecretName := util.NameWithSuffix(sr.Instance.Name, common.ArgoCDTLSSuffix)

	_, err := workloads.GetSecret(tlsSecretName, sr.Instance.Namespace, sr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			sr.Logger.Error(err, "reconcileClusterTLSSecret: failed to retrieve secret", "name", tlsSecretName, "namespace", sr.Instance.Namespace)
			return err
		}

		caSecretName := util.NameWithSuffix(sr.Instance.Name, common.ArgoCDCASuffix)
		caSecret, err := workloads.GetSecret(caSecretName, sr.Instance.Namespace, sr.Client)
		if err != nil {
			if !errors.IsNotFound(err) {
				sr.Logger.Error(err, "reconcileClusterTLSSecret: failed to retrieve secret", "name", caSecretName, "namespace", sr.Instance.Namespace)
				return err
			}

			sr.Logger.Error(err, "reconcileClusterTLSSecret: CA secret required, but not found", "name", caSecretName, "namespace", sr.Instance.Namespace)
			return err
		}

		caCert, err := util.ParsePEMEncodedCert(caSecret.Data[corev1.TLSCertKey])
		if err != nil {
			sr.Logger.Error(err, "reconcileClusterTLSSecret: failed to parse encoded cert")
			return err
		}

		caKey, err := util.ParsePEMEncodedPrivateKey(caSecret.Data[corev1.TLSPrivateKeyKey])
		if err != nil {
			sr.Logger.Error(err, "reconcileClusterTLSSecret: failed to parse encoded private key")
			return err
		}

		tlsSecret, err := sr.getDesiredTLSCertSecret(tlsSecretName, caCert, caKey)
		if err != nil {
			sr.Logger.Error(err, "reconcileClusterTLSSecret: failed to generate secret", "name", tlsSecretName)
		}

		if err = controllerutil.SetControllerReference(sr.Instance, caSecret, sr.Scheme); err != nil {
			sr.Logger.Error(err, "reconcileClusterTLSSecret: failed to set owner reference for secret", "name", tlsSecret.Name, "namespace", tlsSecret.Namespace)
		}

		if err = workloads.CreateSecret(tlsSecret, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileClusterTLSSecret: failed to create secret", "name", tlsSecret.Name, "namespace", tlsSecret.Namespace)
			return err
		}
		sr.Logger.V(0).Info("reconcileClusterTLSSecret: secret created", "name", tlsSecret.Name, "namespace", tlsSecret.Namespace)
		return nil
	}

	// secret exists, nothing to do
	return nil
}

// getDesiredCASecret generates a new private key and self signed certificate for the CA,
// creates a TLS secret and injects the encoded cert and key pem artifacts into the secret
func (sr *SecretReconciler) getDesiredCASecret(caSecretName string) (*corev1.Secret, error) {
	key, err := util.NewPrivateKey()
	if err != nil {
		sr.Logger.Error(err, "getDesiredCASecret: failed to generate private key")
		return nil, err
	}

	cert, err := util.NewSelfSignedCACertificate(sr.Instance.Name, key)
	if err != nil {
		sr.Logger.Error(err, "getDesiredCASecret: failed to generate self signed CA certificate")
		return nil, err
	}

	// construct desired secret template, use it to populate a secret request object
	// and receive the constructed CA secret from the workloads package
	caSecretTmpl := sr.getDesiredSecretTmplObj(caSecretName)
	caSecretTmpl.Type = corev1.SecretTypeTLS
	// This puts both ca.crt and tls.crt into the secret request.
	caSecretTmpl.Data = map[string][]byte{
		corev1.TLSCertKey:              util.EncodeCertificatePEM(cert),
		corev1.ServiceAccountRootCAKey: util.EncodeCertificatePEM(cert),
		corev1.TLSPrivateKeyKey:        util.EncodePrivateKeyPEM(key),
	}
	secretReq := sr.getSecretRequest(*caSecretTmpl)
	caSecret, err := workloads.RequestSecret(secretReq)
	if err != nil {
		sr.Logger.Error(err, "getDesiredCASecret: failed to request secret", "name", caSecretName, "namespace", sr.Instance.Namespace)
		return nil, err
	}

	return caSecret, nil
}

// getDesiredTLSCertSecret generates a new private key and signed TLS certificate using given CA cert and key,
// injects it into a new TLS secret and returns it
func (sr *SecretReconciler) getDesiredTLSCertSecret(tlsCertName string, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*corev1.Secret, error) {
	key, err := util.NewPrivateKey()
	if err != nil {
		sr.Logger.Error(err, "getDesiredTLSCertSecret: failed to generate private key")
		return nil, err
	}

	cfg := &tlsutil.CertConfig{
		CertName:     tlsCertName,
		CertType:     tlsutil.ClientAndServingCert,
		CommonName:   tlsCertName,
		Organization: []string{sr.Instance.Namespace},
	}

	dnsNames := []string{
		sr.Instance.Name,
		util.NameWithSuffix(sr.Instance.Name, common.ArgoCDGRPCSuffix),
		fmt.Sprintf("%s.%s.svc.cluster.local", sr.Instance.Name, sr.Instance.Namespace),
	}

	cert, err := util.NewSignedCertificate(cfg, dnsNames, key, caCert, caKey)
	if err != nil {
		sr.Logger.Error(err, "getDesiredTLSCertSecret: failed to generate signed certificate")
		return nil, err
	}

	tlsSecretTmpl := sr.getDesiredSecretTmplObj(tlsCertName)
	tlsSecretTmpl.Type = corev1.SecretTypeTLS
	tlsSecretTmpl.Data = map[string][]byte{
		corev1.TLSCertKey:       util.EncodeCertificatePEM(cert),
		corev1.TLSPrivateKeyKey: util.EncodePrivateKeyPEM(key),
	}
	secretReq := sr.getSecretRequest(*tlsSecretTmpl)
	tlsSecret, err := workloads.RequestSecret(secretReq)
	if err != nil {
		sr.Logger.Error(err, "getDesiredTLSCertSecret: failed to request secret", "name", tlsSecret, "namespace", sr.Instance.Namespace)
		return nil, err
	}

	return tlsSecret, nil
}

// GetClusterSecrets receives a list of secrets carrying the Argo CD cluster-secret label
func (sr *SecretReconciler) GetClusterSecrets() (*corev1.SecretList, error) {
	clusterSecrets := &corev1.SecretList{}
	listOpts := make([]cntrlClient.ListOption, 0)
	listOpts = append(listOpts, cntrlClient.MatchingLabelsSelector{
		Selector: labels.SelectorFromSet(map[string]string{
			common.ArgoCDArgoprojKeySecretType: ClusterSecretType,
		}),
	})

	clusterSecrets, err := workloads.ListSecrets(sr.Instance.Namespace, sr.Client, listOpts)
	if err != nil {
		sr.Logger.Error(err, "GetClusterSecrets: failed to retrieve cluster secrets")
		return nil, err
	}
	return clusterSecrets, nil
}

// getSecretRequest returns a populated secret request object from the given secret template
func (sr *SecretReconciler) getSecretRequest(secretTmpl corev1.Secret) workloads.SecretRequest {
	return workloads.SecretRequest{
		ObjectMeta: secretTmpl.ObjectMeta,
		Data:       secretTmpl.Data,
		Type:       secretTmpl.Type,
		Client:     sr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}
}

func (sr *SecretReconciler) getDesiredSecretTmplObj(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   sr.Instance.Namespace,
			Labels:      common.DefaultLabels(name, sr.Instance.Name, ""),
			Annotations: sr.Instance.Annotations,
		},
	}
}
