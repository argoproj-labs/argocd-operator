package openshift

import (
	"fmt"

	"golang.org/x/mod/semver"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
)

func init() {
	mutation.Register(AddSeccompProfileForOpenShift)
	mutation.Register(AppendNonRootSCCForOpenShift)
}

func AddSeccompProfileForOpenShift(cr *argoproj.ArgoCD, resource interface{}, client client.Client) error {
	if !IsOpenShiftEnv() {
		return nil
	}
	switch obj := resource.(type) {
	case *corev1.PodSpec:
		if !IsVersionAPIAvailable() {
			return nil
		}
		version, err := GetClusterVersion(client)
		if err != nil {
			return errors.Wrapf(err, "AddSeccompProfileForOpenShift: failed to retrieve OpenShift cluster version")
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

func AppendNonRootSCCForOpenShift(cr *argoproj.ArgoCD, resource interface{}, client client.Client) error {
	if !IsOpenShiftEnv() {
		return nil
	}
	switch obj := resource.(type) {
	case *rbacv1.Role:
		// This mutation only applies to redis and redis-ha roles
		if component, ok := obj.Annotations[common.AppK8sKeyComponent]; !ok || (ok && component != common.RedisComponent) {
			return nil
		}

		if !IsVersionAPIAvailable() {
			return nil
		}
		version, err := GetClusterVersion(client)
		if err != nil {
			return errors.Wrapf(err, "AppendNonRootSCCForOpenShift: failed to retrieve OpenShift cluster version")
		}
		// Starting with OpenShift 4.11, we need to use the resource name "nonroot-v2" instead of "nonroot"
		resourceName := "nonroot"
		if version == "" || semver.Compare(fmt.Sprintf("v%s", version), "v4.10.999") > 0 {
			orules := rbacv1.PolicyRule{
				APIGroups: []string{
					"security.openshift.io",
				},
				ResourceNames: []string{
					resourceName,
				},
				Resources: []string{
					"securitycontextconstraints",
				},
				Verbs: []string{
					"use",
				},
			}
			obj.Rules = append(obj.Rules, orules)
		}
	}

	return nil
}
