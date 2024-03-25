package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
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

// reconcileIngresses will ensure that all ArgoCD Server Ingress resources are present.
func (sr *ServerReconciler) reconcileIngresses() error {
	var reconErrs util.MultiError

	// reconcile ingress for server
	if sr.Instance.Spec.Server.Ingress.Enabled {
		if err := sr.reconcileIngress(); err != nil {
			reconErrs.Append(err)
		}
	} else {
		if err := sr.deleteIngress(resourceName, sr.Instance.Namespace); err != nil {
			reconErrs.Append(err)
		}
	}

	// reconcile ingress for server grpc
	if sr.Instance.Spec.Server.GRPC.Ingress.Enabled {
		if err := sr.reconcileGRPCIngress(); err != nil {
			reconErrs.Append(err)
		}
	} else {
		if err := sr.deleteIngress(grpcResourceName, sr.Instance.Namespace); err != nil {
			reconErrs.Append(err)
		}
	}

	return reconErrs.ErrOrNil()
}

// reconcileIngress will ensure that ArgoCD .Spec.Server.Ingress resource is present.
func (sr *ServerReconciler) reconcileIngress() error {
	req := networking.IngressRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component, util.EmptyMap(), argocdcommon.GetIngressNginxAnnotations()),
		Client:     sr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	// override default annotations if specified
	if len(sr.Instance.Spec.Server.Ingress.Annotations) > 0 {
		req.ObjectMeta.Annotations = sr.Instance.Spec.Server.Ingress.Annotations
	}

	req.Spec.IngressClassName = sr.Instance.Spec.Server.Ingress.IngressClassName

	pathType := networkingv1.PathTypeImplementationSpecific
	req.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: sr.getHost(),
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path: getPathOrDefault(sr.Instance.Spec.Server.Ingress.Path),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: resourceName,
									Port: networkingv1.ServiceBackendPort{
										Name: "http",
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

	// add default TLS options
	req.Spec.TLS = []networkingv1.IngressTLS{
		{
			Hosts: []string{
				sr.getHost(),
			},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// override TLS options if specified
	if len(sr.Instance.Spec.Server.Ingress.TLS) > 0 {
		req.Spec.TLS = sr.Instance.Spec.Server.Ingress.TLS
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

	return sr.reconIngress(req, argocdcommon.UpdateFnIngress(updateFn), ignoreDrift)
}

// reconcileGRPCIngress will ensure that ArgoCD .Spec.Server.GRPC.Ingress resource is present.
func (sr *ServerReconciler) reconcileGRPCIngress() error {

	req := networking.IngressRequest{
		ObjectMeta: argoutil.GetObjMeta(grpcResourceName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component, util.EmptyMap(), argocdcommon.GetGRPCIngressNginxAnnotations()),
		Client:     sr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	// override default annotations if specified
	if len(sr.Instance.Spec.Server.GRPC.Ingress.Annotations) > 0 {
		req.ObjectMeta.Annotations = sr.Instance.Spec.Server.GRPC.Ingress.Annotations
	}

	req.Spec.IngressClassName = sr.Instance.Spec.Server.GRPC.Ingress.IngressClassName

	pathType := networkingv1.PathTypeImplementationSpecific
	req.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: sr.getGRPCHost(),
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path: getPathOrDefault(sr.Instance.Spec.Server.GRPC.Ingress.Path),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: resourceName,
									Port: networkingv1.ServiceBackendPort{
										Name: "https",
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

	// add TLS options
	req.Spec.TLS = []networkingv1.IngressTLS{
		{
			Hosts: []string{
				sr.getGRPCHost(),
			},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// override TLS options if specified
	if len(sr.Instance.Spec.Server.GRPC.Ingress.TLS) > 0 {
		req.Spec.TLS = sr.Instance.Spec.Server.GRPC.Ingress.TLS
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

	return sr.reconIngress(req, argocdcommon.UpdateFnIngress(updateFn), ignoreDrift)
}

func (sr *ServerReconciler) reconIngress(req networking.IngressRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := networking.RequestIngress(req)
	if err != nil {
		sr.Logger.Debug("reconIngress: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconIngress: failed to request Ingress %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(sr.Instance, desired, sr.Scheme); err != nil {
		sr.Logger.Error(err, "reconIngress: failed to set owner reference for Ingress", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := networking.GetIngress(desired.Name, desired.Namespace, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconIngress: failed to retrieve Ingress %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = networking.CreateIngress(desired, sr.Client); err != nil {
			return errors.Wrapf(err, "reconIngress: failed to create Ingress %s in namespace %s", desired.Name, desired.Namespace)
		}
		sr.Logger.Info("Ingress created", "name", desired.Name, "namespace", desired.Namespace)
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

	if err = networking.UpdateIngress(existing, sr.Client); err != nil {
		return errors.Wrapf(err, "reconIngress: failed to update Ingress %s", existing.Name)
	}

	sr.Logger.Info("Ingress updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteIngress will delete ingress with given name.
func (sr *ServerReconciler) deleteIngress(name, namespace string) error {
	if err := networking.DeleteIngress(name, namespace, sr.Client); err != nil {
		// resource is already deleted, ignore error
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteIngress: failed to delete ingress %s in namespace %s", name, namespace)
	}
	sr.Logger.Info("ingress deleted", "name", name, "namespace", namespace)
	return nil
}
