package notifications

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

func (nr *NotificationsReconciler) reconcileSecret() error {

	req := workloads.SecretRequest{
		ObjectMeta: argoutil.GetObjMeta(common.NotificationsSecretName, nr.Instance.Namespace, nr.Instance.Name, nr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Client:     nr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	ignoreDrift := false
	updateFn := func(existing, desired *corev1.Secret, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Data, Desired: &desired.Data, ExtraAction: nil},
			{Existing: &existing.StringData, Desired: &desired.StringData, ExtraAction: nil},
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		}
		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}

	return nr.reconSecret(req, argocdcommon.UpdateFnSecret(updateFn), ignoreDrift)
}

func (nr *NotificationsReconciler) reconSecret(req workloads.SecretRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := workloads.RequestSecret(req)
	if err != nil {
		nr.Logger.Debug("reconSecret: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconSecret: failed to request Secret %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(nr.Instance, desired, nr.Scheme); err != nil {
		nr.Logger.Error(err, "reconSecret: failed to set owner reference for Secret", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := workloads.GetSecret(desired.Name, desired.Namespace, nr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconSecret: failed to retrieve Secret %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = workloads.CreateSecret(desired, nr.Client); err != nil {
			return errors.Wrapf(err, "reconSecret: failed to create Secret %s in namespace %s", desired.Name, desired.Namespace)
		}
		nr.Logger.Info("Secret created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// Secret found, no update required - nothing to do
	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnSecret); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconSecret: failed to execute update function for %s in namespace %s", existing.Name, existing.Namespace)
			}
		}
	}

	if !changed {
		return nil
	}

	if err = workloads.UpdateSecret(existing, nr.Client); err != nil {
		return errors.Wrapf(err, "reconSecret: failed to update Secret %s", existing.Name)
	}

	nr.Logger.Info("Secret updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (nr *NotificationsReconciler) deleteSecret(namespace string) error {
	if err := workloads.DeleteSecret(common.NotificationsSecretName, namespace, nr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		nr.Logger.Error(err, "DeleteSecret: failed to delete secret", "name", common.NotificationsSecretName, "namespace", namespace)
		return err
	}
	nr.Logger.Info("secret deleted", "name", common.NotificationsSecretName, "namespace", namespace)
	return nil
}
