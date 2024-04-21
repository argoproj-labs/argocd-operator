package argocd

import (
	json "encoding/json"
	"strings"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/sso"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	argopass "github.com/argoproj/argo-cd/v2/util/password"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	adminCredsResourceName         string
	clusterPermissionsResourceName string
	tlsResourceName                string
)

const (
	clusterConfigSuffix = "default-cluster-config"
)

func (r *ArgoCDReconciler) reconcileSecrets() error {
	var reconErrs util.MultiError
	r.secretVarSetter()

	err := r.reconcileAdminCredentialsSecret()
	reconErrs.Append(err)

	err = r.reconcileCASecret()
	reconErrs.Append(err)

	err = r.reconcileTLSSecret()
	reconErrs.Append(err)

	err = r.reconcileClusterPermissionsSecret()
	reconErrs.Append(err)

	err = r.reconcileArgoCDSecret()
	reconErrs.Append(err)

	return reconErrs.ErrOrNil()
}

func (r *ArgoCDReconciler) deleteSecrets() error {
	var delErrs util.MultiError

	err := r.deleteSecret(common.ArgoCDSecretName, r.Instance.Namespace)
	delErrs.Append(err)

	err = r.deleteSecret(clusterPermissionsResourceName, r.Instance.Namespace)
	delErrs.Append(err)

	err = r.deleteSecret(tlsResourceName, r.Instance.Namespace)
	delErrs.Append(err)

	err = r.deleteSecret(caResourceName, r.Instance.Namespace)
	delErrs.Append(err)

	err = r.deleteSecret(adminCredsResourceName, r.Instance.Namespace)
	delErrs.Append(err)

	return delErrs.ErrOrNil()
}

func (r *ArgoCDReconciler) reconcileArgoCDSecret() error {
	ignoreDrift := false

	clusterPermSecret, err := workloads.GetSecret(adminCredsResourceName, r.Instance.Namespace, r.Client)
	if err != nil {
		return errors.Wrapf(err, "reconcileArgoCDSecret: failed to retrieve secret %s in namespace %s", adminCredsResourceName, r.Instance.Namespace)
	}

	tlsSecret, err := workloads.GetSecret(tlsResourceName, r.Instance.Namespace, r.Client)
	if err != nil {
		return errors.Wrapf(err, "reconcileArgoCDSecret: failed to retrieve tls secret %s in namespace %s", tlsResourceName, r.Instance.Namespace)
	}

	sessionKey, err := argoutil.GenerateArgoServerSessionKey()
	if err != nil {
		return errors.Wrapf(err, "reconcileArgoCDSecret: failed to generate server session key")
	}

	var hashedPw string
	pwBytes, ok := clusterPermSecret.Data[common.ArgoCDKeyAdminPassword]
	if ok {
		hashedPw, err = argopass.HashPassword(strings.TrimRight(string(pwBytes), "\n"))
		if err != nil {
			return errors.Wrapf(err, "reconcileArgoCDSecret: failed to encrypt admin password")
		}
	}

	req := workloads.SecretRequest{
		ObjectMeta: argoutil.GetObjMeta(common.ArgoCDSecretName, r.Instance.Namespace, r.Instance.Name, r.Instance.Namespace, "", util.EmptyMap(), util.EmptyMap()),
		Data: map[string][]byte{
			common.ArgoCDKeyAdminPassword:      []byte(hashedPw),
			common.ArgoCDKeyAdminPasswordMTime: util.NowBytes(),
			common.ArgoCDKeyServerSecretKey:    sessionKey,
			common.ArgoCDKeyTLSCert:            tlsSecret.Data[common.ArgoCDKeyTLSCert],
			common.ArgoCDKeyTLSPrivateKey:      tlsSecret.Data[common.ArgoCDKeyTLSPrivateKey],
			// TO DO: skip dex secret config
		},
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Client:    r.Client,
	}

	updateFn := func(existing, desired *corev1.Secret, changed *bool) error {
		clusterPermSecret, err := workloads.GetSecret(adminCredsResourceName, r.Instance.Namespace, r.Client)
		if err != nil {
			return errors.Wrapf(err, "reconcileArgoCDSecret: failed to retrieve secret %s in namespace %s", adminCredsResourceName, r.Instance.Namespace)
		}

		tlsSecret, err := workloads.GetSecret(tlsResourceName, r.Instance.Namespace, r.Client)
		if err != nil {
			return errors.Wrapf(err, "reconcileArgoCDSecret: failed to retrieve tls secret %s in namespace %s", tlsResourceName, r.Instance.Namespace)
		}

		if existing.Data == nil {
			existing.Data = make(map[string][]byte)
		}

		if existing.Data[common.ArgoCDKeyServerSecretKey] == nil {
			sessionKey, err := argoutil.GenerateArgoServerSessionKey()
			if err != nil {
				return errors.Wrapf(err, "reconcileArgoCDSecret: failed to generate server session key")
			}
			existing.Data[common.ArgoCDKeyServerSecretKey] = sessionKey
		}

		if argoutil.HasArgoAdminPasswordChanged(existing, clusterPermSecret) {
			pwBytes, ok := clusterPermSecret.Data[common.ArgoCDKeyAdminPassword]
			if ok && existing.Data[common.ArgoCDKeyAdminPassword] == nil {
				hashedPassword, err := argopass.HashPassword(strings.TrimRight(string(pwBytes), "\n"))
				if err != nil {
					return errors.Wrapf(err, "reconcileArgoCDSecret: failed to encrypt admin password")
				}

				existing.Data[common.ArgoCDKeyAdminPassword] = []byte(hashedPassword)
				existing.Data[common.ArgoCDKeyAdminPasswordMTime] = util.NowBytes()
				*changed = true
			}
		}

		if argoutil.HasArgoTLSChanged(existing, tlsSecret) {
			existing.Data[common.ArgoCDKeyTLSCert] = tlsSecret.Data[common.ArgoCDKeyTLSCert]
			existing.Data[common.ArgoCDKeyTLSPrivateKey] = tlsSecret.Data[common.ArgoCDKeyTLSPrivateKey]
			*changed = true
		}

		// if Dex is enabled, store/update dex OAuth client secret
		if provider := r.SSOController.GetProvider(r.Instance); provider == argoproj.SSOProviderTypeDex &&
			r.SSOController.GetStatus() != sso.SSOLegalFailed &&
			r.SSOController.GetStatus() != sso.SSOLegalUnknown {
			desiredDexOIDCClientSecret, err := r.SSOController.DexController.GetOAuthClientSecret()
			if err != nil {
				return errors.Wrap(err, "reconcileArgoCDSecret: failed to get dex oidc client secret")
			}
			actual := string(existing.Data[common.ArgoCDDexSecretKey])
			if desiredDexOIDCClientSecret != nil {
				expected := *desiredDexOIDCClientSecret
				if actual != expected {
					existing.Data[common.ArgoCDDexSecretKey] = []byte(*desiredDexOIDCClientSecret)
					*changed = true
				}
			}
		}

		return nil
	}

	return r.reconcileSecret(req, argocdcommon.UpdateFnSecret(updateFn), ignoreDrift)
}

func (r *ArgoCDReconciler) reconcileClusterPermissionsSecret() error {
	ignoreDrift := false
	nsList := util.StringMapKeys(r.ResourceManagedNamespaces)
	dataBytes, _ := json.Marshal(map[string]interface{}{
		"tlsClientConfig": map[string]interface{}{
			"insecure": false,
		},
	})

	req := workloads.SecretRequest{
		ObjectMeta: argoutil.GetObjMeta(clusterPermissionsResourceName, r.Instance.Namespace, r.Instance.Name, r.Instance.Namespace, "", map[string]string{
			common.ArgoCDSecretTypeLabel: common.ArgoCDSecretTypeCluster,
		}, util.EmptyMap()),
		Data: map[string][]byte{
			"config":     dataBytes,
			"name":       []byte("in-cluster"),
			"server":     []byte(common.ArgoCDDefaultServer),
			"namespaces": []byte(strings.Join(nsList, ",")),
		},
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Client:    r.Client,
	}

	if r.ClusterScoped {
		delete(req.Data, "namespaces")
	}

	updateFn := func(existing, desired *corev1.Secret, changed *bool) error {
		fieldsToCompare := func(existing, desired *corev1.Secret) []argocdcommon.FieldToCompare {
			return []argocdcommon.FieldToCompare{
				{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
				{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
				{Existing: &existing.Data, Desired: &desired.Data, ExtraAction: nil},
			}
		}
		argocdcommon.UpdateIfChanged(fieldsToCompare(existing, desired), changed)
		return nil
	}

	return r.reconcileSecret(req, argocdcommon.UpdateFnSecret(updateFn), ignoreDrift)
}

func (r *ArgoCDReconciler) reconcileAdminCredentialsSecret() error {
	ignoreDrift := true
	adminPwd, err := argoutil.GenerateArgoAdminPassword()
	if err != nil {
		return errors.Wrap(err, "reconcileAdminCredentialsSecret: failed to generate admin password")
	}

	req := workloads.SecretRequest{
		ObjectMeta: argoutil.GetObjMeta(adminCredsResourceName, r.Instance.Namespace, r.Instance.Name, r.Instance.Namespace, "", util.EmptyMap(), util.EmptyMap()),
		Data: map[string][]byte{
			common.ArgoCDKeyAdminPassword: adminPwd,
		},
		Type:      corev1.SecretTypeOpaque,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Client:    r.Client,
	}
	return r.reconcileSecret(req, nil, ignoreDrift)
}

// reconcileTLSSecret ensures the TLS Secret is created for the ArgoCD cluster.
// This secret contains the TLS certificate used by Argo CD components. It is generated using the
// CA cert and CA key contained within the cluster CA secret
func (r *ArgoCDReconciler) reconcileTLSSecret() error {
	ignoreDrift := true
	caSecret, err := workloads.GetSecret(caResourceName, r.Instance.Namespace, r.Client)
	if err != nil {
		return errors.Wrapf(err, "reconcileClusterTLSSecret: failed to retrieve ca secret %s in namespace %s", caResourceName, r.Instance.Namespace)
	}

	caCert, err := argoutil.ParsePEMEncodedCert(caSecret.Data[corev1.TLSCertKey])
	if err != nil {
		return errors.Wrapf(err, "reconcileClusterTLSSecret: failed to parse CA certificate")
	}

	caKey, err := argoutil.ParsePEMEncodedPrivateKey(caSecret.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		return errors.Wrapf(err, "reconcileClusterTLSSecret: failed to parse CA key")
	}

	cert, pvtKey, err := argoutil.NewTLSCertAndKey(tlsResourceName, r.Instance.Name, r.Instance.Namespace, caCert, caKey)
	if err != nil {
		return errors.Wrapf(err, "reconcileCLusterCASecret")
	}

	req := workloads.SecretRequest{
		ObjectMeta: argoutil.GetObjMeta(tlsResourceName, r.Instance.Namespace, r.Instance.Name, r.Instance.Namespace, "", util.EmptyMap(), util.EmptyMap()),
		Data: map[string][]byte{
			corev1.TLSCertKey:       argoutil.EncodeCertificatePEM(cert),
			corev1.TLSPrivateKeyKey: argoutil.EncodePrivateKeyPEM(pvtKey),
		},
		Type:      corev1.SecretTypeTLS,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Client:    r.Client,
	}

	return r.reconcileSecret(req, nil, ignoreDrift)
}

// reconcileCASecret ensures the CA Secret is reconciled for the ArgoCD cluster.
// This secret contains the self-signed CA certificate and CA key that is used to generate the TLS
// certificate contained in the cluster TLS secret
func (r *ArgoCDReconciler) reconcileCASecret() error {
	ignoreDrift := true
	cert, pvtKey, err := argoutil.NewCACertAndKey(r.Instance.Name)
	if err != nil {
		return errors.Wrapf(err, "reconcileCLusterCASecret")
	}

	req := workloads.SecretRequest{
		ObjectMeta: argoutil.GetObjMeta(caResourceName, r.Instance.Namespace, r.Instance.Name, r.Instance.Namespace, "", util.EmptyMap(), util.EmptyMap()),
		Data: map[string][]byte{
			corev1.TLSCertKey:              argoutil.EncodeCertificatePEM(cert),
			corev1.ServiceAccountRootCAKey: argoutil.EncodeCertificatePEM(cert),
			corev1.TLSPrivateKeyKey:        argoutil.EncodePrivateKeyPEM(pvtKey),
		},
		Type:      corev1.SecretTypeTLS,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Client:    r.Client,
	}

	return r.reconcileSecret(req, nil, ignoreDrift)
}

func (r *ArgoCDReconciler) reconcileSecret(req workloads.SecretRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := workloads.RequestSecret(req)
	if err != nil {
		r.Logger.Debug("reconcileSecret: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconcileSecret: failed to request secret %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(r.Instance, desired, r.Scheme); err != nil {
		r.Logger.Error(err, "reconcileSecret: failed to set owner reference for secret", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := workloads.GetSecret(desired.Name, desired.Namespace, r.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileSecret: failed to retrieve secret %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = workloads.CreateSecret(desired, r.Client); err != nil {
			return errors.Wrapf(err, "reconcileSecret: failed to create secret %s in namespace %s", desired.Name, desired.Namespace)
		}
		r.Logger.Info("secret created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// secret found, no update required - nothing to do
	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnSecret); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconcileSecret: failed to execute update function for %s in namespace %s", existing.Name, existing.Namespace)
			}
		}
	}

	if !changed {
		return nil
	}

	if err = workloads.UpdateSecret(existing, r.Client); err != nil {
		return errors.Wrapf(err, "reconcileSecret: failed to update secret %s", existing.Name)
	}

	r.Logger.Info("secret updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (r *ArgoCDReconciler) deleteSecret(name, namespace string) error {
	if err := workloads.DeleteSecret(name, namespace, r.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteSecret: failed to delete secret %s", name)
	}
	r.Logger.Info("secret deleted", "name", name, "namespace", namespace)
	return nil
}

func (r *ArgoCDReconciler) secretVarSetter() {
	tlsResourceName = argoutil.GenerateResourceName(r.Instance.Name, common.TLSSuffix)
	adminCredsResourceName = argoutil.GenerateResourceName(r.Instance.Name, common.CLusterSuffix)
	clusterPermissionsResourceName = argoutil.GenerateResourceName(r.Instance.Name, clusterConfigSuffix)
}
