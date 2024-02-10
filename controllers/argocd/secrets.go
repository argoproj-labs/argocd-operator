package argocd

import (
	json "encoding/json"
	"strings"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
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
	caResourceName                 string
	tlsResourceName                string
	adminCredsResourceName         string
	clusterPermissionsResourceName string
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

	// err = r.reconcileArgoCDSecret()
	// reconErrs.Append(err)

	return reconErrs.ErrOrNil()
}

func (r *ArgoCDReconciler) reconcileArgoCDSecret() error {
	ignoreUpdate := false
	clusterPermSecret, err := workloads.GetSecret(adminCredsResourceName, r.Instance.Namespace, r.Client)
	if err != nil {
		return errors.Wrapf(err, "reconcileArgoCDSecret: failed to retrieve secret %s in namespace %s", adminCredsResourceName, r.Instance.Namespace)
	}

	tlsSecret, err := workloads.GetSecret(tlsResourceName, r.Instance.Namespace, r.Client)
	if err != nil {
		return errors.Wrapf(err, "reconcileArgoCDSecret: failed to retrieve tls secret %s in namespace %s", tlsResourceName, r.Instance.Namespace)
	}

	hashedPwd, err := argopass.HashPassword(string(clusterPermSecret.Data[common.ArgoCDKeyAdminPassword]))
	if err != nil {
		return errors.Wrapf(err, "reconcileArgoCDSecret: failed to encrypt admin password")
	}

	// sessionKey, err := generateArgoServerSessionKey()
	// if err != nil {
	// 	return errors.Wrapf(err, "reconcileArgoCDSecret: failed to generate server session key")
	// }

	req := workloads.SecretRequest{
		ObjectMeta: argoutil.GetObjMeta(common.ArgoCDSecretName, r.Instance.Namespace, r.Instance.Name, r.Instance.Namespace, "", util.EmptyMap(), util.EmptyMap()),
		Data: map[string][]byte{
			common.ArgoCDKeyAdminPassword: []byte(hashedPwd),
			// common.ArgoCDKeyAdminPasswordMTime: util.NowBytes(),
			// common.ArgoCDKeyServerSecretKey:    sessionKey,
			common.ArgoCDKeyTLSCert:       tlsSecret.Data[common.ArgoCDKeyTLSCert],
			common.ArgoCDKeyTLSPrivateKey: tlsSecret.Data[common.ArgoCDKeyTLSPrivateKey],
			// TO DO: skip dex secret config
		},
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Client:    r.Client,
	}

	fieldsToCompare := func(existing, desired *corev1.Secret) []argocdcommon.FieldToCompare {
		return []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
			{Existing: &existing.Data, Desired: &desired.Data, ExtraAction: nil},
		}
	}

	return r.reconcileSecret(req, fieldsToCompare, ignoreUpdate)
}

func (r *ArgoCDReconciler) reconcileClusterPermissionsSecret() error {
	ignoreUpdate := false
	nsList := util.StringMapKeys(r.ResourceManagedNamespaces)
	dataBytes, _ := json.Marshal(map[string]interface{}{
		"tlsClientConfig": map[string]interface{}{
			"insecure": false,
		},
	})

	req := workloads.SecretRequest{
		ObjectMeta: argoutil.GetObjMeta(clusterPermissionsResourceName, r.Instance.Namespace, r.Instance.Name, r.Instance.Namespace, "", map[string]string{
			common.ArgoCDSecretTypeLabel: common.SecretTypeCluster,
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

	fieldsToCompare := func(existing, desired *corev1.Secret) []argocdcommon.FieldToCompare {
		return []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
			{Existing: &existing.Data, Desired: &desired.Data, ExtraAction: nil},
		}
	}

	return r.reconcileSecret(req, fieldsToCompare, ignoreUpdate)
}

func (r *ArgoCDReconciler) reconcileAdminCredentialsSecret() error {
	ignoreUpdate := true
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

	fieldsToCompare := func(existing, desired *corev1.Secret) []argocdcommon.FieldToCompare {
		return []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
			{Existing: &existing.Data, Desired: &desired.Data, ExtraAction: nil},
		}
	}

	return r.reconcileSecret(req, fieldsToCompare, ignoreUpdate)
}

// reconcileTLSSecret ensures the TLS Secret is created for the ArgoCD cluster.
// This secret contains the TLS certificate used by Argo CD components. It is generated using the
// CA cert and CA key contained within the cluster CA secret
func (r *ArgoCDReconciler) reconcileTLSSecret() error {
	ignoreUpdate := true
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

	fieldsToCompare := func(existing, desired *corev1.Secret) []argocdcommon.FieldToCompare {
		return []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
			{Existing: &existing.Data, Desired: &desired.Data, ExtraAction: nil},
		}
	}

	return r.reconcileSecret(req, fieldsToCompare, ignoreUpdate)
}

// reconcileCASecret ensures the CA Secret is reconciled for the ArgoCD cluster.
// This secret contains the self-signed CA certificate and CA key that is used to generate the TLS
// certificate contained in the cluster TLS secret
func (r *ArgoCDReconciler) reconcileCASecret() error {
	ignoreUpdate := true
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

	fieldsToCompare := func(existing, desired *corev1.Secret) []argocdcommon.FieldToCompare {
		return []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
			{Existing: &existing.Data, Desired: &desired.Data, ExtraAction: nil},
		}
	}

	return r.reconcileSecret(req, fieldsToCompare, ignoreUpdate)
}

func (r *ArgoCDReconciler) reconcileSecret(req workloads.SecretRequest, compare argocdcommon.FieldCompFnSecret, ignoreUpdate bool) error {
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
	if ignoreUpdate {
		return nil
	}

	changed := false

	argocdcommon.UpdateIfChanged(compare(existing, desired), &changed)

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
	caResourceName = argoutil.GenerateResourceName(r.Instance.Name, common.CASuffix)
	tlsResourceName = argoutil.GenerateResourceName(r.Instance.Name, common.TLSSuffix)
	adminCredsResourceName = argoutil.GenerateResourceName(r.Instance.Name, common.CLusterSuffix)
	clusterPermissionsResourceName = argoutil.GenerateResourceName(r.Instance.Name, clusterConfigSuffix)

}
