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
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	maxReplicas int32 = 3
	minReplicas int32 = 1
	tcup        int32 = 50
)

// reconcileHorizontalPodAutoscaler will ensure that ArgoCD .Spec.Server.Autoscale resource is present.
func (sr *ServerReconciler) reconcileHorizontalPodAutoscaler() error {

	req := workloads.HorizontalPodAutoscalerRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Spec: autoscalingv1.HorizontalPodAutoscalerSpec{
			MaxReplicas:                    maxReplicas,
			MinReplicas:                    &minReplicas,
			TargetCPUUtilizationPercentage: &tcup,
			ScaleTargetRef: autoscalingv1.CrossVersionObjectReference{
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

	ignoreDrift := false
	updateFn := func(existing, desired *autoscalingv1.HorizontalPodAutoscaler, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Spec, Desired: &desired.Spec, ExtraAction: nil},
		}
		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}
	return sr.reconHorizontalPodAutoscaler(req, argocdcommon.UpdateFnHPA(updateFn), ignoreDrift)
}

func (sr *ServerReconciler) reconHorizontalPodAutoscaler(req workloads.HorizontalPodAutoscalerRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := workloads.RequestHorizontalPodAutoscaler(req)
	if err != nil {
		sr.Logger.Debug("reconHorizontalPodAutoscaler: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconHorizontalPodAutoscaler: failed to request HorizontalPodAutoscaler %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(sr.Instance, desired, sr.Scheme); err != nil {
		sr.Logger.Error(err, "reconHorizontalPodAutoscaler: failed to set owner reference for HorizontalPodAutoscaler", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := workloads.GetHorizontalPodAutoscaler(desired.Name, desired.Namespace, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconHorizontalPodAutoscaler: failed to retrieve HorizontalPodAutoscaler %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = workloads.CreateHorizontalPodAutoscaler(desired, sr.Client); err != nil {
			return errors.Wrapf(err, "reconHorizontalPodAutoscaler: failed to create HorizontalPodAutoscaler %s in namespace %s", desired.Name, desired.Namespace)
		}
		sr.Logger.Info("HorizontalPodAutoscaler created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// HorizontalPodAutoscaler found, no update required - nothing to do
	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnHPA); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconHorizontalPodAutoscaler: failed to execute update function for %s in namespace %s", existing.Name, existing.Namespace)
			}
		}
	}

	if !changed {
		return nil
	}

	if err = workloads.UpdateHorizontalPodAutoscaler(existing, sr.Client); err != nil {
		return errors.Wrapf(err, "reconHorizontalPodAutoscaler: failed to update HorizontalPodAutoscaler %s", existing.Name)
	}

	sr.Logger.Info("HorizontalPodAutoscaler updated", "name", existing.Name, "namespace", existing.Namespace)
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
