package notifications

import "github.com/argoproj-labs/argocd-operator/pkg/resource"

// reconcileStatus will ensure that the notifications controller status is updated for the given ArgoCD instance
func (nr *NotificationsReconciler) ReconcileStatus() error {

	// TO DO

	return nr.updateInstanceStatus()
}

func (nr *NotificationsReconciler) updateInstanceStatus() error {
	return resource.UpdateStatusSubResource(nr.Instance, nr.Client)
}
