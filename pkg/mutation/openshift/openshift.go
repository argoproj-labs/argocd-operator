package openshift

import (
	"fmt"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"golang.org/x/mod/semver"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	mutation.Register(AddSeccompProfileForOpenShift)
}

func AddSeccompProfileForOpenShift(cr *v1alpha1.ArgoCD, resource interface{}, client client.Client) error {
	if !cluster.IsVersionAPIAvailable() {
		return nil
	}
	switch obj := resource.(type) {
	case *corev1.PodSpec:
		version, err := cluster.GetClusterVersion(client)
		if err != nil {
			return err
		}
		if version == "" || semver.Compare(fmt.Sprintf("v%s", version), "v4.10.999") > 0 {
			if obj.SecurityContext == nil {
				obj.SecurityContext = &corev1.PodSecurityContext{}
			}
			if obj.SecurityContext.SeccompProfile == nil {
				obj.SecurityContext.SeccompProfile = &corev1.SeccompProfile{}
			}
			if len(obj.SecurityContext.SeccompProfile.Type) == 0 {
				obj.SecurityContext.SeccompProfile.Type = corev1.SeccompProfileTypeRuntimeDefault
			}
		}
	}
	return nil
}
