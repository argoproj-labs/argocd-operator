package argocd

import argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"

// getArgoCDNotificationsControllerReplicas will return the size value for the argocd-notifications-controller replica count if it
// has been set in argocd CR. Otherwise, nil is returned if the replicas is not set in the argocd CR or
// replicas value is < 0.
func getArgoCDNotificationsControllerReplicas(cr *argoproj.ArgoCD) *int32 {
	if cr.Spec.Notifications.Replicas != nil && *cr.Spec.Notifications.Replicas >= 0 {
		return cr.Spec.Notifications.Replicas
	}

	return nil
}
