package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
)

// reconcileArgoCDCM updates the Argo CD cm with the server's URI
func (sr *ServerReconciler) reconcileArgoCDCM() error {
	cm, err := workloads.GetConfigMap(common.ArgoCDConfigMapName, sr.Instance.Namespace, sr.Client)
	if err != nil {
		return errors.Wrap(err, "reconcileArgoCDCM: failed to retrieve config map")
	}

	cm.Data[common.ArgoCDKeyServerURL] = sr.getURI()

	if err := workloads.UpdateConfigMap(cm, sr.Client); err != nil {
		return errors.Wrap(err, "reconcileArgoCDCM: failed to update configmap")
	}

	sr.Logger.Info("config map updated", "name", common.ArgoCDConfigMapName, "namespace", sr.Instance.Namespace)
	return nil
}
