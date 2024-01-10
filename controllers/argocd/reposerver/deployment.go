package reposerver

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
)

// TriggerDeploymentRollout starts server deployment rollout by updating the given key
func (rsr *RepoServerReconciler) TriggerDeploymentRollout(name, namespace, key string) error {
	return argocdcommon.TriggerDeploymentRollout(name, namespace, key, rsr.Client)
}
