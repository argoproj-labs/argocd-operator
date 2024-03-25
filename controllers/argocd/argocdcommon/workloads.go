package argocdcommon

import (
	"context"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"sigs.k8s.io/controller-runtime/pkg/client"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
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

	if statefulset.Spec.Template.ObjectMeta.Labels == nil {
		statefulset.Spec.Template.ObjectMeta.Labels = make(map[string]string)
	}

	statefulset.Spec.Template.ObjectMeta.Labels[key] = util.NowNano()
	return workloads.UpdateStatefulSet(statefulset, client)
}

// Returns true if a StatefulSet has pods in ErrImagePull or ImagePullBackoff state.
// These pods cannot be restarted automatially due to known kubernetes issue https://github.com/kubernetes/kubernetes/issues/67250
func ContainsInvalidImage(ls labels.Selector, cr *argoproj.ArgoCD, cl client.Client, logger *util.Logger) bool {

	brokenPod := false

	podList := &corev1.PodList{}

	if err := cl.List(context.TODO(), podList, &cntrlClient.ListOptions{
		LabelSelector: ls,
		Namespace:     cr.Namespace,
	}); err != nil {
		logger.Error(err, "ContainsInvalidImage: failed to list pods")
	}
	if len(podList.Items) > 0 {
		if len(podList.Items[0].Status.ContainerStatuses) > 0 {
			if podList.Items[0].Status.ContainerStatuses[0].State.Waiting != nil && (podList.Items[0].Status.ContainerStatuses[0].State.Waiting.Reason == "ImagePullBackOff" || podList.Items[0].Status.ContainerStatuses[0].State.Waiting.Reason == "ErrImagePull") {
				brokenPod = true
			}
		}
	}
	return brokenPod
}
