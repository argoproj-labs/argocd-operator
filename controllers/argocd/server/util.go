package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
)

func (sr *ServerReconciler) TriggerServerDeploymentRollout() error {
	name := argoutil.NameWithSuffix(sr.Instance.Name, common.ServerControllerComponent)
	return workloads.TriggerDeploymentRollout(sr.Client, name, sr.Instance.Namespace, func(name string, namespace string) {
		deployment, err := workloads.GetDeployment(name, namespace, sr.Client)
		if err != nil {
			sr.Logger.Error(err, "triggerServerDeploymentRollout: failed to trigger server deployment", "name", name, "namespace", namespace)
		}
		deployment.Spec.Template.ObjectMeta.Labels[common.ArgoCDRepoTLSCertChangedKey] = util.NowNano()
	})
}
