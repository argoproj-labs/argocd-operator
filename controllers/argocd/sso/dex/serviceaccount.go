package dex

import (
	"strconv"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (dr *DexReconciler) reconcileServiceAccount() error {
	req := permissions.ServiceAccountRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, dr.Instance.Namespace, dr.Instance.Name, dr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		MutationArgs: util.ConvertStringMapToInterfaces(map[string]string{
			common.RedirectURI:        dr.GetOAuthRedirectURI(),
			common.WantOpenShiftOAuth: strconv.FormatBool(dr.Instance.Spec.SSO.Dex.OpenShiftOAuth),
		}),
	}

	ignoreDrift := false
	updateFn := func(existing, desired *corev1.ServiceAccount, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
		}

		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}

	return dr.reconServiceAccount(req, argocdcommon.UpdateFnSa(updateFn), ignoreDrift)
}

func (dr *DexReconciler) reconServiceAccount(req permissions.ServiceAccountRequest, updateFn interface{}, ignoreDrift bool) error {
	desired := permissions.RequestServiceAccount(req)

	if err := controllerutil.SetControllerReference(dr.Instance, desired, dr.Scheme); err != nil {
		dr.Logger.Error(err, "reconServiceAccount: failed to set owner reference for ServiceAccount", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := permissions.GetServiceAccount(desired.Name, desired.Namespace, dr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconServiceAccount: failed to retrieve ServiceAccount %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = permissions.CreateServiceAccount(desired, dr.Client); err != nil {
			return errors.Wrapf(err, "reconServiceAccount: failed to create ServiceAccount %s in namespace %s", desired.Name, desired.Namespace)
		}
		dr.Logger.Info("service account created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnSa); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconServiceaccount: failed to execute update function for %s in namespace %s", existing.Name, existing.Namespace)
			}
		}
	}

	if !changed {
		return nil
	}

	if err = permissions.UpdateServiceAccount(existing, dr.Client); err != nil {
		return errors.Wrapf(err, "reconServiceaccount: failed to update serviceaccount %s", existing.Name)
	}

	dr.Logger.Info("serviceaccount updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteServiceAccount will delete service account with given name.
func (dr *DexReconciler) deleteServiceAccount(name, namespace string) error {
	if err := permissions.DeleteServiceAccount(name, namespace, dr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteServiceAccount: failed to delete serviceaccount %s in namespace %s", name, namespace)
	}
	dr.Logger.Info("service account deleted", "name", name, "namespace", namespace)
	return nil
}
