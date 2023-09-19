package applicationset

import (
	"fmt"

	"github.com/argoproj-labs/argocd-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
)

// isMergable returns error if any of the extraArgs is already part of the default command Arguments.
func isMergable(extraArgs []string, cmd []string) error {
	for _, arg := range extraArgs {
		if len(arg) > 2 && arg[:2] == "--" && util.ContainsString(cmd, arg, false) {
			return fmt.Errorf("Arg %s is already part of the default command arguments", arg)
		}
	}
	return nil
}

// getApplicationSetResources will return the ResourceRequirements for the Application Sets container.
func (asr *ApplicationSetReconciler) getApplicationSetResources() corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if asr.Instance.Spec.ApplicationSet.Resources != nil {
		resources = *asr.Instance.Spec.ApplicationSet.Resources
	}

	return resources
}

// getSCMRootCAConfigMapName will return the SCMRootCA ConfigMap name for the given ArgoCD ApplicationSet Controller.
func (asr *ApplicationSetReconciler) getSCMRootCAConfigMapName() string {
	if asr.Instance.Spec.ApplicationSet.SCMRootCAConfigMap != "" && len(asr.Instance.Spec.ApplicationSet.SCMRootCAConfigMap) > 0 {
		return asr.Instance.Spec.ApplicationSet.SCMRootCAConfigMap
	}
	return ""
}
