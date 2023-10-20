package appcontroller

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
)

func (acr *AppControllerReconciler) TriggerAppControllerStatefulSetRollout() error {
	name := util.NameWithSuffix(acr.Instance.Name, common.ApplicationControllerComponent)
	return workloads.TriggerStatefulSetRollout(acr.Client, name, acr.Instance.Namespace, func(name string, namespace string) {
		statefulSet, err := workloads.GetStatefulSet(name, namespace, acr.Client)
		if err != nil {
			acr.Logger.Error(err, "TriggerAppControllerStatefulSetRollout: failed to trigger server deployment", "name", name, "namespace", namespace)
		}
		if statefulSet.Spec.Template.ObjectMeta.Labels == nil {
			statefulSet.Spec.Template.ObjectMeta.Labels = make(map[string]string)
		}
		statefulSet.Spec.Template.ObjectMeta.Labels[common.ArgoCDRepoTLSCertChangedKey] = util.NowNano()
	})
}
