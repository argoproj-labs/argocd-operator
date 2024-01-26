package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileHorizontalPodAutoscaler will ensure that ArgoCD .Spec.Server.Autoscale resource is present.
func (sr *ServerReconciler) reconcileHorizontalPodAutoscaler() error {

	var (
		maxReplicas int32 = 3
		minReplicas int32 = 1
		tcup        int32 = 50
	)

	// AutoScale not enabled, cleanup any existing hpa & exit
	if !sr.Instance.Spec.Server.Autoscale.Enabled {
		return sr.deleteHorizontalPodAutoscaler(resourceName, sr.Instance.Namespace)
	}

	hpaReq := workloads.HorizontalPodAutoscalerRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component),
		Spec: autoscaling.HorizontalPodAutoscalerSpec{
			MaxReplicas:                    maxReplicas,
			MinReplicas:                    &minReplicas,
			TargetCPUUtilizationPercentage: &tcup,
			ScaleTargetRef: autoscaling.CrossVersionObjectReference{
				APIVersion: appsv1.GroupName,
				Kind:       common.DeploymentKind,
				Name:       resourceName,
			},
		},
		Client:    sr.Client,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	// HPA spec provided in ArgoCD CR, override default spec
	if sr.Instance.Spec.Server.Autoscale.HPA != nil {
		hpaReq.Spec = *sr.Instance.Spec.Server.Autoscale.HPA
	}

	desiredHPA, err := workloads.RequestHorizontalPodAutoscaler(hpaReq)
	if err != nil {
		return errors.Wrapf(err, "reconcileHorizontalPodAutoscaler: failed to request hpa %s in namespace %s", desiredHPA.Name, desiredHPA.Namespace)
	}

	if err := controllerutil.SetControllerReference(sr.Instance, desiredHPA, sr.Scheme); err != nil {
		sr.Logger.Error(err, "reconcileHorizontalPodAutoscaler: failed to set owner reference for hpa", "name", desiredHPA.Name, "namespace", desiredHPA.Namespace)
	}

	// hpa doesn't exist in the namespace, create it
	existingHPA, err := workloads.GetHorizontalPodAutoscaler(desiredHPA.Name, desiredHPA.Namespace, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileHorizontalPodAutoscaler: failed to retrieve hpa %s in namespace %s", desiredHPA.Name, desiredHPA.Namespace)
		}

		if err = workloads.CreateHorizontalPodAutoscaler(desiredHPA, sr.Client); err != nil {
			return errors.Wrapf(err, "reconcileHorizontalPodAutoscaler: failed to create hpa %s in namespace %s", desiredHPA.Name, desiredHPA.Namespace)
		}

		sr.Logger.V(0).Info("hpa created", "name", desiredHPA.Name, "namespace", desiredHPA.Namespace)
		return nil
	}

	// difference in existing & desired hpa, update it
	changed := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{&existingHPA.Spec, &desiredHPA.Spec, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &changed)
	}

	// nothing changed, exit reconciliation
	if !changed {
		return nil
	}

	if err = workloads.UpdateHorizontalPodAutoscaler(existingHPA, sr.Client); err != nil {
		return errors.Wrapf(err, "reconcileHorizontalPodAutoscaler: failed to update hpa %s in namespace %s", existingHPA.Name, existingHPA.Namespace)
	}

	sr.Logger.V(0).Info("hpa updated", "name", existingHPA.Name, "namespace", existingHPA.Namespace)
	return nil

}

// deleteHorizontalPodAutoscaler will delete hpa with given name.
func (sr *ServerReconciler) deleteHorizontalPodAutoscaler(name, namespace string) error {
	if err := workloads.DeleteHorizontalPodAutoscaler(name, namespace, sr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteHorizontalPodAutoscaler: failed to delete hpa %s in namespace %s", name, namespace)
	}
	sr.Logger.V(0).Info("hpa deleted", "name", name, "namespace", namespace)
	return nil
}
