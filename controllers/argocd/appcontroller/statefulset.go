package appcontroller

import "github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"

func (acr *AppControllerReconciler) TriggerStatefulSetRollout(name, namespace, key string) error {
	return argocdcommon.TriggerStatefulSetRollout(name, namespace, key, acr.Client)
}
