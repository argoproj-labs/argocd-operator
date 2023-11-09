package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/networking"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	amerr "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileIngresses will ensure that all ArgoCD Server Ingress resources are present.
func (sr *ServerReconciler) reconcileIngresses() error {
	sr.Logger.Info("reconciling ingresses")

	var reconciliationErrors []error

	// reconcile ingress for server 
	if err := sr.reconcileServerIngress(); err != nil {
		reconciliationErrors = append(reconciliationErrors, err)
	}

	// reconcile ingress for server grpc
	if err := sr.reconcileServerGRPCIngress(); err != nil {
		reconciliationErrors = append(reconciliationErrors, err)
	}

	return amerr.NewAggregate(reconciliationErrors)
}

// reconcileIngresses will ensure that ArgoCD .Spec.Server.Ingress resource is present.
func (sr *ServerReconciler) reconcileServerIngress() error {

	ingressName := getIngressName(sr.Instance.Name)
	ingressNS := sr.Instance.Namespace

	// default annotations
	ingressLabels := common.DefaultLabels(ingressName, sr.Instance.Name, ServerControllerComponent)
	ingressLabels[ArgoCDKeyIngressSSLRedirect] = "true"
	ingressLabels[ArgoCDKeyIngressBackendProtocol] = "HTTP"

	// ingress disabled, cleanup and exit 
	if !sr.Instance.Spec.Server.Ingress.Enabled {
		return sr.deleteIngress(ingressName, ingressNS)
	}

	ingressRequest := networking.IngressRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: 		 ingressName,
			Labels:      ingressLabels,
			Annotations: sr.Instance.Annotations,
			Namespace: 	ingressNS,
		},
		Client:    sr.Client,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	// override default annotations if specified
	if len(sr.Instance.Spec.Server.Ingress.Annotations) > 0 {
		ingressRequest.ObjectMeta.Annotations = sr.Instance.Spec.Server.Ingress.Annotations
	}

	ingressRequest.Spec.IngressClassName = sr.Instance.Spec.Server.Ingress.IngressClassName

	pathType := networkingv1.PathTypeImplementationSpecific

	ingressRequest.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: getHost(sr.Instance),
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path: getPathOrDefault(sr.Instance.Spec.Server.Ingress.Path),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: ingressName,
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
	ingressRequest.Spec.TLS = []networkingv1.IngressTLS{
		{
			Hosts: []string{
				getHost(sr.Instance),
			},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// override TLS options if specified
	if len(sr.Instance.Spec.Server.Ingress.TLS) > 0 {
		ingressRequest.Spec.TLS = sr.Instance.Spec.Server.Ingress.TLS
	}

	return sr.reconcileIngress(ingressRequest)
} 

// reconcileIngresses will ensure that ArgoCD .Spec.Server.GRPC.Ingress resource is present.
func (sr *ServerReconciler) reconcileServerGRPCIngress() error {

	ingressName := getGRPCIngressName(sr.Instance.Name)
	ingressNS := sr.Instance.Namespace

	// default annotations
	ingressLabels := common.DefaultLabels(ingressName, sr.Instance.Name, ServerControllerComponent)
	ingressLabels[ArgoCDKeyIngressBackendProtocol] = "GRPC"

	// ingress disabled, cleanup and exit 
	if !sr.Instance.Spec.Server.GRPC.Ingress.Enabled  {
		return sr.deleteIngress(ingressName, ingressNS)
	}

	ingressRequest := networking.IngressRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: 		 ingressName,
			Labels:      ingressLabels,
			Annotations: sr.Instance.Annotations,
			Namespace: 	ingressNS,
		},
		Client:    sr.Client,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	// override default annotations if specified
	if len(sr.Instance.Spec.Server.GRPC.Ingress.Annotations) > 0 {
		ingressRequest.ObjectMeta.Annotations = sr.Instance.Spec.Server.GRPC.Ingress.Annotations
	}

	ingressRequest.Spec.IngressClassName = sr.Instance.Spec.Server.GRPC.Ingress.IngressClassName

	pathType := networkingv1.PathTypeImplementationSpecific

	ingressRequest.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: getGRPCHost(sr.Instance),
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path: getPathOrDefault(sr.Instance.Spec.Server.GRPC.Ingress.Path),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: ingressName,
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
	ingressRequest.Spec.TLS = []networkingv1.IngressTLS{
		{
			Hosts: []string{
				getGRPCHost(sr.Instance),
			},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// override TLS options if specified
	if len(sr.Instance.Spec.Server.GRPC.Ingress.TLS) > 0 {
		ingressRequest.Spec.TLS = sr.Instance.Spec.Server.GRPC.Ingress.TLS
	}

	return sr.reconcileIngress(ingressRequest)
} 

// reconcileIngress will ensure that provided ingressRequest resource is created or updated.
func (sr *ServerReconciler) reconcileIngress(ingressRequest networking.IngressRequest) error {

	desiredIngress, err := networking.RequestIngress(ingressRequest)
	if err != nil {
		sr.Logger.Error(err, "reconcileIngress: failed to request ingress", "name", desiredIngress.Name, "namespace", desiredIngress.Namespace)
		sr.Logger.V(1).Info("reconcileIngress: one or more mutations could not be applied")
		return err
	}

	// ingress doesn't exist in the namespace, create it
	existingIngress, err := networking.GetIngress(desiredIngress.Name, desiredIngress.Namespace, sr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			sr.Logger.Error(err, "reconcileIngress: failed to retrieve ingress", "name", desiredIngress.Name, "namespace", desiredIngress.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(sr.Instance, desiredIngress, sr.Scheme); err != nil {
			sr.Logger.Error(err, "reconcileIngress: failed to set owner reference for ingress", "name", desiredIngress.Name, "namespace", desiredIngress.Namespace)
		}

		if err = networking.CreateIngress(desiredIngress, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileIngress: failed to create ingress", "name", desiredIngress.Name, "namespace", desiredIngress.Namespace)
			return err
		}
		
		sr.Logger.V(0).Info("reconcileIngress: serviceAccount ingress", "name", desiredIngress.Name, "namespace", desiredIngress.Namespace)
		return nil
	}

		// difference in existing & desired ingress, update it
		changed := false

		fieldsToCompare := []struct {
			existing, desired interface{}
			extraAction       func()
		}{
			{&existingIngress.ObjectMeta.Annotations, &desiredIngress.ObjectMeta.Annotations, nil},
			{&existingIngress.Spec.IngressClassName, &desiredIngress.Spec.IngressClassName, nil},
			{&existingIngress.Spec.Rules, &desiredIngress.Spec.Rules, nil},
			{&existingIngress.Spec.TLS, &desiredIngress.Spec.TLS, nil},
		}
	
		for _, field := range fieldsToCompare {
			argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &changed)
		}
	
		if changed {
			if err = networking.UpdateIngress(existingIngress, sr.Client); err != nil {
				sr.Logger.Error(err, "reconcileIngress: failed to update ingress", "name", existingIngress.Name, "namespace", existingIngress.Namespace)
				return err
			}
			sr.Logger.V(0).Info("reconcileIngress: ingress updated", "name", existingIngress.Name, "namespace", existingIngress.Namespace)
		}
	
	// ingress found, no changes detected
	return nil
}

// deleteIngresses will delete all ArgoCD Server Ingress resources
func (sr *ServerReconciler) deleteIngresses(argoCDName, namespace string) error {
	var reconciliationErrors []error

	// delete server ingress
	if err := sr.deleteIngress(getIngressName(argoCDName), namespace); err != nil {
		reconciliationErrors = append(reconciliationErrors, err)
	}

	// delete server grpc ingress
	if err := sr.deleteIngress(getGRPCIngressName(argoCDName), namespace); err != nil {
		reconciliationErrors = append(reconciliationErrors, err)
	}

	return amerr.NewAggregate(reconciliationErrors)
}

// deleteIngress will delete ingress with given name.
func (sr *ServerReconciler) deleteIngress(name, namespace string) error {
	if err := networking.DeleteIngress(name, namespace, sr.Client); err != nil {
		// resource is already deleted, ignore error
		if errors.IsNotFound(err) {
			return nil
		}

		sr.Logger.Error(err, "deleteIngress: failed to delete ingress", "name", name, "namespace", namespace)
		return err
	}
	sr.Logger.V(0).Info("deleteIngress: ingress deleted", "name", name, "namespace", namespace)
	return nil
}


