package argocd

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	caResourceName  string
	tlsResourceName string
)

func (r *ArgoCDReconciler) reconcileSecrets() error {
	var reconErrs util.MultiError
	r.secretVarSetter()

	err := r.reconcileCLusterCASecret()
	reconErrs.Append(err)

	return reconErrs.ErrOrNil()
}

// reconcileClusterTLSSecret ensures the TLS Secret is created for the ArgoCD cluster.
// This secret contains the TLS certificate used by Argo CD components. It is generated using the
// CA cert and CA key contained within the cluster CA secret
func (r *ArgoCDReconciler) reconcileClusterTLSSecret() error {
	caSecret, err := workloads.GetSecret(caResourceName, r.Instance.Namespace, r.Client)
	if err != nil {
		return errors.Wrapf(err, "reconcileClusterTLSSecret: failed to retrieve ca secret %s in namespace %s", caResourceName, r.Instance.Namespace)
	}

}

// reconcileClusterCASecret ensures the CA Secret is reconciled for the ArgoCD cluster.
// This secret contains the self-signed CA certificate and CA key that is used to generate the TLS
// certificate contained in the cluster TLS secret
func (r *ArgoCDReconciler) reconcileCLusterCASecret() error {
	pvtKey, err := argoutil.NewPrivateKey()
	if err != nil {
		return errors.Wrapf(err, "reconcileCLusterCASecret: failed to generate private key")
	}

	cert, err := argoutil.NewSelfSignedCACertificate(r.Instance.Name, pvtKey)
	if err != nil {
		return errors.Wrapf(err, "reconcileCLusterCASecret: failed to generate self-signed certificate")
	}

	req := workloads.SecretRequest{
		ObjectMeta: argoutil.GetObjMeta(caResourceName, r.Instance.Namespace, r.Instance.Name, r.Instance.Namespace, ""),
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

	return r.reconcileSecret(req, fieldsToCompare)
}

func (r *ArgoCDReconciler) reconcileSecret(req workloads.SecretRequest, compare argocdcommon.FieldCompFnSecret) error {
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
	caResourceName = argoutil.GenerateResourceName(r.Instance.Name, common.ArgoCDCASuffix)
	tlsResourceName = argoutil.GenerateResourceName(r.Instance.Name, common.ArgoCDTLSSuffix)
}
