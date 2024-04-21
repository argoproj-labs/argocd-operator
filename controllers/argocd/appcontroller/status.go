package appcontroller

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
)

// reconcileStatus will ensure that the app-controller status is updated for the given ArgoCD instance
func (acr *AppControllerReconciler) ReconcileStatus() error {
	status := common.ArgoCDStatusUnknown

	if acr.Instance.Spec.Controller.IsEnabled() {
		ss, err := workloads.GetStatefulSet(resourceName, acr.Instance.Namespace, acr.Client)
		if err != nil {
			return errors.Wrapf(err, "failed to retrieve statefulset %s", resourceName)
		}

		status = common.ArgoCDStatusPending

		if ss.Spec.Replicas != nil {
			if ss.Status.ReadyReplicas == *ss.Spec.Replicas {
				status = common.ArgoCDStatusRunning
			}
		}
	}

	if acr.Instance.Status.ApplicationController != status {
		acr.Instance.Status.ApplicationController = status
	}

	return acr.updateInstanceStatus()
}

func (acr *AppControllerReconciler) updateInstanceStatus() error {
	return resource.UpdateStatusSubResource(acr.Instance, acr.Client)
}
