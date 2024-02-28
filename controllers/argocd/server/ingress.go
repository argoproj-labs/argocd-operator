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
	if err := sr.reconcileServerIngress(); err != nil {
		reconErrs.Append(err)
	}

	// reconcile ingress for server grpc
	if err := sr.reconcileServerGRPCIngress(); err != nil {
		reconErrs.Append(err)
	}

	return reconErrs.ErrOrNil()
}

// reconcileIngresses will ensure that ArgoCD .Spec.Server.Ingress resource is present.
func (sr *ServerReconciler) reconcileServerIngress() error {

	// ingress disabled, cleanup and exit
	if !sr.Instance.Spec.Server.Ingress.Enabled {
		return sr.deleteIngress(resourceName, sr.Instance.Namespace)
	}

	ingressReq := networking.IngressRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Client:     sr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	// add ingress labels
	ingressReq.ObjectMeta.Labels[common.NginxIngressK8sKeyForceSSLRedirect] = "true"
	ingressReq.ObjectMeta.Labels[common.NginxIngressK8sKeyBackendProtocol] = "true"

	// override default annotations if specified
	if len(sr.Instance.Spec.Server.Ingress.Annotations) > 0 {
		ingressReq.ObjectMeta.Annotations = sr.Instance.Spec.Server.Ingress.Annotations
	}

	ingressReq.Spec.IngressClassName = sr.Instance.Spec.Server.Ingress.IngressClassName

	pathType := networkingv1.PathTypeImplementationSpecific
	ingressReq.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: getHost(sr.Instance),
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
	ingressReq.Spec.TLS = []networkingv1.IngressTLS{
		{
			Hosts: []string{
				getHost(sr.Instance),
			},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// override TLS options if specified
	if len(sr.Instance.Spec.Server.Ingress.TLS) > 0 {
		ingressReq.Spec.TLS = sr.Instance.Spec.Server.Ingress.TLS
	}

	return sr.reconcileIngress(ingressReq)
}

// reconcileIngresses will ensure that ArgoCD .Spec.Server.GRPC.Ingress resource is present.
func (sr *ServerReconciler) reconcileServerGRPCIngress() error {

	ingressName := resourceName + "-grpc"

	// ingress disabled, cleanup and exit
	if !sr.Instance.Spec.Server.GRPC.Ingress.Enabled {
		return sr.deleteIngress(ingressName, sr.Instance.Namespace)
	}

	ingressReq := networking.IngressRequest{
		ObjectMeta: argoutil.GetObjMeta(ingressName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Client:     sr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	// add ingress annotations
	ingressReq.ObjectMeta.Labels[common.NginxIngressK8sKeyBackendProtocol] = "GRPC"

	// override default annotations if specified
	if len(sr.Instance.Spec.Server.GRPC.Ingress.Annotations) > 0 {
		ingressReq.ObjectMeta.Annotations = sr.Instance.Spec.Server.GRPC.Ingress.Annotations
	}

	ingressReq.Spec.IngressClassName = sr.Instance.Spec.Server.GRPC.Ingress.IngressClassName

	pathType := networkingv1.PathTypeImplementationSpecific
	ingressReq.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: getGRPCHost(sr.Instance),
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
	ingressReq.Spec.TLS = []networkingv1.IngressTLS{
		{
			Hosts: []string{
				getGRPCHost(sr.Instance),
			},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// override TLS options if specified
	if len(sr.Instance.Spec.Server.GRPC.Ingress.TLS) > 0 {
		ingressReq.Spec.TLS = sr.Instance.Spec.Server.GRPC.Ingress.TLS
	}

	return sr.reconcileIngress(ingressReq)
}

// reconcileIngress will ensure that provided ingressRequest resource is created or updated.
func (sr *ServerReconciler) reconcileIngress(req networking.IngressRequest) error {

	desired, err := networking.RequestIngress(req)
	if err != nil {
		return errors.Wrapf(err, "reconcileIngress: failed to request ingress %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err := controllerutil.SetControllerReference(sr.Instance, desired, sr.Scheme); err != nil {
		sr.Logger.Error(err, "reconcileIngress: failed to set owner reference for ingress", "name", desired.Name, "namespace", desired.Namespace)
	}

	// ingress doesn't exist in the namespace, create it
	existing, err := networking.GetIngress(desired.Name, desired.Namespace, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileIngress: failed to retrieve ingress %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = networking.CreateIngress(desired, sr.Client); err != nil {
			return errors.Wrapf(err, "reconcileIngress: failed to create ingress %s in namespace %s", desired.Name, desired.Namespace)
		}

		sr.Logger.Info("ingress created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// difference in existing & desired ingress, update it
	changed := false
	fieldsToCompare := []argocdcommon.FieldToCompare{
		{Existing: &existing.ObjectMeta.Annotations, Desired: &desired.ObjectMeta.Annotations, ExtraAction: nil},
		{Existing: &existing.Spec.IngressClassName, Desired: &desired.Spec.IngressClassName, ExtraAction: nil},
		{Existing: &existing.Spec.Rules, Desired: &desired.Spec.Rules, ExtraAction: nil},
		{Existing: &existing.Spec.TLS, Desired: &desired.Spec.TLS, ExtraAction: nil},
	}
	argocdcommon.UpdateIfChanged(fieldsToCompare, &changed)

	// nothing changed, exit reconciliation
	if !changed {
		return nil
	}

	if err = networking.UpdateIngress(existing, sr.Client); err != nil {
		return errors.Wrapf(err, "reconcileIngress: failed to update ingress %s in namespace %s", existing.Name, existing.Namespace)
	}

	sr.Logger.Info("reconcileIngress: ingress updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteIngresses will delete all ArgoCD Server Ingress resources
func (sr *ServerReconciler) deleteIngresses(name, namespace string) error {
	var reconErrs util.MultiError

	// delete server ingress
	if err := sr.deleteIngress(name, namespace); err != nil {
		reconErrs.Append(err)
	}

	// delete server grpc ingress
	if err := sr.deleteIngress(name+"-grpc", namespace); err != nil {
		reconErrs.Append(err)
	}

	return reconErrs.ErrOrNil()
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
