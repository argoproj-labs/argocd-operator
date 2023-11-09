package server

import (
	"reflect"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	appsv1 "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileHorizontalPodAutoscaler will ensure that ArgoCD .Spec.Server.Autoscale resource is present.
func (sr *ServerReconciler) reconcileHorizontalPodAutoscaler() error {
	sr.Logger.Info("reconciling horizontal pod autoscaler")

	var (
		maxReplicas int32 = 3
		minReplicas int32 = 1
		tcup        int32 = 50
	)

	hpaName := getHPAName(sr.Instance.Name)
	hpaNS := sr.Instance.Namespace
	hpaLabels := common.DefaultLabels(hpaName, hpaNS, ServerControllerComponent)

	deploymentName := getDeploymentName(sr.Instance.Name)

	// AutoScale not enabled, cleanup any existing hpa & exit 
	if !sr.Instance.Spec.Server.Autoscale.Enabled {
		return sr.deleteHorizontalPodAutoscaler(hpaName, hpaNS)
	}

	hpaReq := workloads.HorizontalPodAutoscalerRequest{
		ObjectMeta:  metav1.ObjectMeta{
			Name:        hpaName,
			Namespace: 	 hpaNS,
			Labels:      hpaLabels,
			Annotations: sr.Instance.Annotations,
		},
		Spec: autoscaling.HorizontalPodAutoscalerSpec{
			MaxReplicas:                    maxReplicas,
			MinReplicas:                    &minReplicas,
			TargetCPUUtilizationPercentage: &tcup,
			ScaleTargetRef: autoscaling.CrossVersionObjectReference{
				APIVersion: appsv1.GroupName,
				Kind:       common.DeploymentKind,
				Name:       deploymentName,
			},
		},
		Client: sr.Client,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	// HPA spec provided in ArgoCD CR, override default spec
	if sr.Instance.Spec.Server.Autoscale.HPA != nil {
		hpaReq.Spec = *sr.Instance.Spec.Server.Autoscale.HPA
	} 

	desiredHPA, err := workloads.RequestHorizontalPodAutoscaler(hpaReq)
	if err != nil {
		sr.Logger.Error(err, "reconcileHorizontalPodAutoscaler: failed to request hpa", "name", desiredHPA.Name, "namespace", desiredHPA.Namespace)
		sr.Logger.V(1).Info("reconcileHorizontalPodAutoscaler: one or more mutations could not be applied")
		return err
	}

	// hpa doesn't exist in the namespace, create it
	existingHPA, err := workloads.GetHorizontalPodAutoscaler(desiredHPA.Name, desiredHPA.Namespace, sr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			sr.Logger.Error(err, "reconcileHorizontalPodAutoscaler: failed to retrieve hpa", "name", desiredHPA.Name, "namespace", desiredHPA.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(sr.Instance, desiredHPA, sr.Scheme); err != nil {
			sr.Logger.Error(err, "reconcileHorizontalPodAutoscaler: failed to set owner reference for hpa", "name", desiredHPA.Name, "namespace", desiredHPA.Namespace)
		}

		if err = workloads.CreateHorizontalPodAutoscaler(desiredHPA, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileHorizontalPodAutoscaler: failed to create hpa", "name", desiredHPA.Name, "namespace", desiredHPA.Namespace)
			return err
		}
		sr.Logger.V(0).Info("reconcileHorizontalPodAutoscaler: hpa created", "name", desiredHPA.Name, "namespace", desiredHPA.Namespace)
		return nil
	}

	// difference in existing & desired hpa, update it
	changed := false
	if !reflect.DeepEqual(existingHPA.Spec, desiredHPA.Spec) {
		existingHPA.Spec = desiredHPA.Spec
		changed = true
	}

	if changed {
		if err = workloads.UpdateHorizontalPodAutoscaler(existingHPA, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileHorizontalPodAutoscaler: failed to update hpa", "name", existingHPA.Name, "namespace", existingHPA.Namespace)
			return err
		}

		sr.Logger.V(0).Info("reconcileHorizontalPodAutoscaler: hpa updated", "name", existingHPA.Name, "namespace", existingHPA.Namespace)
	}
	
	// hpa found, no changes detected
	return nil

}

// deleteHorizontalPodAutoscaler will delete hpa with given name.
func (sr *ServerReconciler) deleteHorizontalPodAutoscaler(name, namespace string) error {
	if err := workloads.DeleteHorizontalPodAutoscaler(name, namespace, sr.Client); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		sr.Logger.Error(err, "deleteHorizontalPodAutoscaler: failed to delete hpa", "name", name, "namespace", namespace)
		return err
	}
	sr.Logger.V(0).Info("deleteHorizontalPodAutoscaler: hpa deleted", "name", name, "namespace", namespace)
	return nil
}