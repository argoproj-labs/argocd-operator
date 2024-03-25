package applicationset

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/networking"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/pkg/errors"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (asr *ApplicationSetReconciler) reconcileIngress() error {
	req := networking.IngressRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, asr.Instance.Namespace, asr.Instance.Name, asr.Instance.Namespace, component, util.EmptyMap(), argocdcommon.GetIngressNginxAnnotations()),
		Client:     asr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	// override default annotations if specified
	if len(asr.Instance.Spec.ApplicationSet.WebhookServer.Ingress.Annotations) > 0 {
		req.ObjectMeta.Annotations = asr.Instance.Spec.ApplicationSet.WebhookServer.Ingress.Annotations
	}

	pathType := networkingv1.PathTypeImplementationSpecific
	// Add rules
	req.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: asr.getHost(),
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path: "/api/webhook",
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: resourceName,
									Port: networkingv1.ServiceBackendPort{
										Name: "webhook",
									},
								},
							},
							PathType: &pathType,
						},
					},
				},
			},
		},
	}

	// Allow override of TLS options if specified
	if len(asr.Instance.Spec.ApplicationSet.WebhookServer.Ingress.TLS) > 0 {
		req.Spec.TLS = asr.Instance.Spec.ApplicationSet.WebhookServer.Ingress.TLS
	}

	ignoreDrift := false
	updateFn := func(desired, existing *networkingv1.Ingress, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Spec.IngressClassName, Desired: &desired.Spec.IngressClassName, ExtraAction: nil},
			{Existing: &existing.Spec.Rules, Desired: &desired.Spec.Rules, ExtraAction: nil},
			{Existing: &existing.Spec.TLS, Desired: &desired.Spec.TLS, ExtraAction: nil},
		}
		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}

	return asr.reconIngress(req, argocdcommon.UpdateFnIngress(updateFn), ignoreDrift)
}

func (asr *ApplicationSetReconciler) reconIngress(req networking.IngressRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := networking.RequestIngress(req)
	if err != nil {
		asr.Logger.Debug("reconIngress: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconIngress: failed to request Ingress %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(asr.Instance, desired, asr.Scheme); err != nil {
		asr.Logger.Error(err, "reconIngress: failed to set owner reference for Ingress", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := networking.GetIngress(desired.Name, desired.Namespace, asr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconIngress: failed to retrieve Ingress %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = networking.CreateIngress(desired, asr.Client); err != nil {
			return errors.Wrapf(err, "reconIngress: failed to create Ingress %s in namespace %s", desired.Name, desired.Namespace)
		}
		asr.Logger.Info("Ingress created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// Ingress found, no update required - nothing to do
	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnIngress); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconIngress: failed to execute update function for %s in namespace %s", existing.Name, existing.Namespace)
			}
		}
	}

	if !changed {
		return nil
	}

	if err = networking.UpdateIngress(existing, asr.Client); err != nil {
		return errors.Wrapf(err, "reconIngress: failed to update Ingress %s", existing.Name)
	}

	asr.Logger.Info("Ingress updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteIngress will delete ingress with given name.
func (asr *ApplicationSetReconciler) deleteIngress(name, namespace string) error {
	if err := networking.DeleteIngress(name, namespace, asr.Client); err != nil {
		// resource is already deleted, ignore error
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteIngress: failed to delete ingress %s in namespace %s", name, namespace)
	}
	asr.Logger.Info("ingress deleted", "name", name, "namespace", namespace)
	return nil
}
