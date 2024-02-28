package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
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

	req := workloads.HorizontalPodAutoscalerRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
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
		req.Spec = *sr.Instance.Spec.Server.Autoscale.HPA
	}

	desired, err := workloads.RequestHorizontalPodAutoscaler(req)
	if err != nil {
		return errors.Wrapf(err, "reconcileHorizontalPodAutoscaler: failed to request hpa %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err := controllerutil.SetControllerReference(sr.Instance, desired, sr.Scheme); err != nil {
		sr.Logger.Error(err, "reconcileHorizontalPodAutoscaler: failed to set owner reference for hpa", "name", desired.Name, "namespace", desired.Namespace)
	}

	// hpa doesn't exist in the namespace, create it
	existing, err := workloads.GetHorizontalPodAutoscaler(desired.Name, desired.Namespace, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileHorizontalPodAutoscaler: failed to retrieve hpa %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = workloads.CreateHorizontalPodAutoscaler(desired, sr.Client); err != nil {
			return errors.Wrapf(err, "reconcileHorizontalPodAutoscaler: failed to create hpa %s in namespace %s", desired.Name, desired.Namespace)
		}

		sr.Logger.Info("hpa created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// difference in existing & desired hpa, update it
	changed := false
	fieldsToCompare := []argocdcommon.FieldToCompare{
		{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		{Existing: &existing.Spec, Desired: &desired.Spec, ExtraAction: nil},
	}
	argocdcommon.UpdateIfChanged(fieldsToCompare, &changed)

	// nothing changed, exit reconciliation
	if !changed {
		return nil
	}

	if err = workloads.UpdateHorizontalPodAutoscaler(existing, sr.Client); err != nil {
		return errors.Wrapf(err, "reconcileHorizontalPodAutoscaler: failed to update hpa %s in namespace %s", existing.Name, existing.Namespace)
	}

	sr.Logger.Info("hpa updated", "name", existing.Name, "namespace", existing.Namespace)
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
	sr.Logger.Info("hpa deleted", "name", name, "namespace", namespace)
	return nil
}
