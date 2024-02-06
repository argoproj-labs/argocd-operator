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

func (r *ArgoCDReconciler) reconcileConfigMaps() error {
	var reconErrs util.MultiError

	err := r.reconcileRBACCm()
	reconErrs.Append(err)

	err = r.reconcileSSHKnownHostsCm()
	reconErrs.Append(err)

	err = r.reconcileTLSCertsCm()
	reconErrs.Append(err)

	err = r.reconcileGPGKeysCm()
	reconErrs.Append(err)

	return reconErrs.ErrOrNil()
}

// reconcileGPGKeysConfigMap creates a gpg-keys config map
func (r *ArgoCDReconciler) reconcileGPGKeysCm() error {
	req := workloads.ConfigMapRequest{
		ObjectMeta: argoutil.GetObjMeta(common.ArgoCDGPGKeysConfigMapName, r.Instance.Namespace, r.Instance.Name, r.Instance.Namespace, ""),
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Client:     r.Client,
	}

	fieldsToCompare := func(existing, desired *corev1.ConfigMap) []argocdcommon.FieldToCompare {
		return []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
			{Existing: &existing.Data, Desired: &desired.Data, ExtraAction: nil},
		}
	}

	return r.reconcileCM(req, fieldsToCompare)
}

// reconcileTLSCerts will ensure that the ArgoCD TLS Certs ConfigMap is present.
func (r *ArgoCDReconciler) reconcileTLSCertsCm() error {
	req := workloads.ConfigMapRequest{
		ObjectMeta: argoutil.GetObjMeta(common.ArgoCDTLSCertsConfigMapName, r.Instance.Namespace, r.Instance.Name, r.Instance.Namespace, ""),
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Client:     r.Client,
	}

	fieldsToCompare := func(existing, desired *corev1.ConfigMap) []argocdcommon.FieldToCompare {
		return []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
			{Existing: &existing.Data, Desired: &desired.Data, ExtraAction: nil},
		}
	}

	return r.reconcileCM(req, fieldsToCompare)
}

// reconcileSSHKnownHosts will ensure that the ArgoCD SSH Known Hosts ConfigMap is present.
func (r *ArgoCDReconciler) reconcileSSHKnownHostsCm() error {
	req := workloads.ConfigMapRequest{
		ObjectMeta: argoutil.GetObjMeta(common.ArgoCDKnownHostsConfigMapName, r.Instance.Namespace, r.Instance.Name, r.Instance.Namespace, ""),
		Data: map[string]string{
			common.ArgoCDKeySSHKnownHosts: r.getInitialSSHKnownHosts(),
		},
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Client:    r.Client,
	}

	fieldsToCompare := func(existing, desired *corev1.ConfigMap) []argocdcommon.FieldToCompare {
		return []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
			{Existing: &existing.Data, Desired: &desired.Data, ExtraAction: nil},
		}
	}

	return r.reconcileCM(req, fieldsToCompare)
}

// reconcileRBACCm will ensure that the Redis HA ConfigMap is present for the given ArgoCD instance
func (r *ArgoCDReconciler) reconcileRBACCm() error {
	req := workloads.ConfigMapRequest{
		ObjectMeta: argoutil.GetObjMeta(common.ArgoCDRBACConfigMapName, r.Instance.Namespace, r.Instance.Name, r.Instance.Namespace, ""),
		Data: map[string]string{
			common.ArgoCDKeyRBACPolicyCSV:     r.getRBACPolicy(),
			common.ArgoCDKeyRBACPolicyDefault: r.getRBACDefaultPolicy(),
			common.ArgoCDKeyRBACScopes:        r.getRBACScopes(),
			common.ArgoCDKeyPolicyMatcherMode: r.getRBACPolicyMatcherMode(),
		},
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Client:    r.Client,
	}

	fieldsToCompare := func(existing, desired *corev1.ConfigMap) []argocdcommon.FieldToCompare {
		return []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
			{Existing: &existing.Data, Desired: &desired.Data, ExtraAction: nil},
		}
	}

	return r.reconcileCM(req, fieldsToCompare)
}

func (r *ArgoCDReconciler) reconcileCM(req workloads.ConfigMapRequest, fcf argocdcommon.FieldCompFnCm) error {
	desired, err := workloads.RequestConfigMap(req)
	if err != nil {
		r.Logger.Debug("reconcileCM: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconcileCM: failed to request configMap %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(r.Instance, desired, r.Scheme); err != nil {
		r.Logger.Error(err, "reconcileCM: failed to set owner reference for configMap", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := workloads.GetConfigMap(desired.Name, desired.Namespace, r.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileCM: failed to retrieve configMap %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = workloads.CreateConfigMap(desired, r.Client); err != nil {
			return errors.Wrapf(err, "reconcileCM: failed to create configMap %s in namespace %s", desired.Name, desired.Namespace)
		}
		r.Logger.Info("config map created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}
	changed := false

	argocdcommon.UpdateIfChanged(fcf(existing, desired), &changed)

	if !changed {
		return nil
	}

	if err = workloads.UpdateConfigMap(existing, r.Client); err != nil {
		return errors.Wrapf(err, "reconcileCM: failed to update configmap %s", existing.Name)
	}

	r.Logger.Info("configmap updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}
