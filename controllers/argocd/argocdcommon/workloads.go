package argocdcommon

import (
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// TriggerDeploymentRollout will update the label with the given key to trigger a new rollout of the Deployment.
func TriggerDeploymentRollout(name, namespace, key string, client cntrlClient.Client) error {
	deployment, err := workloads.GetDeployment(name, namespace, client)
	if err != nil {
		return err
	}

	if deployment.Spec.Template.ObjectMeta.Labels == nil {
		deployment.Spec.Template.ObjectMeta.Labels = make(map[string]string)
	}

	deployment.Spec.Template.ObjectMeta.Labels[key] = util.NowNano()
	return workloads.UpdateDeployment(deployment, client)
}

// TriggerStatefulSetRollout will update the label with the given key to trigger a new rollout of the StatefulSet.
func TriggerStatefulSetRollout(name, namespace, key string, client cntrlClient.Client) error {
	statefulset, err := workloads.GetStatefulSet(name, namespace, client)
	if err != nil {
		return err
	}

	statefulset.Spec.Template.ObjectMeta.Labels[key] = util.NowNano()
	return workloads.UpdateStatefulSet(statefulset, client)
}
