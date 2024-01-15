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
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

func init() {
	mutation.Register(AddSeccompProfileForOpenShift)
	mutation.Register(AddNonRootSCCForOpenShift)
	mutation.Register(AddAutoTLSAnnotationForOpenShift)
}

// TO DO: Add dedicated e2e tests for all these mutations

// AddAutoTLSAnnotationForOpenShift adds the OpenShift Service CA TLS cert request annotaiton to the provided service object, using the provided secret name as the value
func AddAutoTLSAnnotationForOpenShift(cr *argoproj.ArgoCD, resource interface{}, client client.Client, args ...interface{}) error {
	if !IsOpenShiftEnv() {
		return nil
	}
	switch obj := resource.(type) {
	case *corev1.Service:
		// return if autoTLS is not requested
		if !cr.Spec.Redis.WantsAutoTLS() {
			return nil
		}

		if obj.Annotations == nil {
			obj.Annotations = make(map[string]string)
		}

		// there should only be one extra parameter of type string, which would be the name of the TLS secret to be used in the annotation.
		// Check to make sure length and type of extra argument match before using this as the secret name
		if len(args) == 1 {
			for _, arg := range args {
				switch val := arg.(type) {
				case string:
					obj.Annotations[common.ServiceBetaOpenshiftKeyCertSecret] = val
				}
			}
		}
	}
	return nil
}

func AddSeccompProfileForOpenShift(cr *argoproj.ArgoCD, resource interface{}, client client.Client, args ...interface{}) error {
	if !IsOpenShiftEnv() {
		return nil
	}
	switch obj := resource.(type) {
	case *appsv1.StatefulSet:
	case *appsv1.Deployment:
		if !IsVersionAPIAvailable() {
			return nil
		}
		version, err := GetClusterVersion(client)
		if err != nil {
			return errors.Wrapf(err, "AddSeccompProfileForOpenShift: failed to retrieve OpenShift cluster version")
		}

		podSpec := obj.Spec.Template.Spec

		if version == "" || semver.Compare(fmt.Sprintf("v%s", version), "v4.10.999") > 0 {
			if podSpec.SecurityContext == nil {
				podSpec.SecurityContext = &corev1.PodSecurityContext{}
			}
			if podSpec.SecurityContext.SeccompProfile == nil {
				podSpec.SecurityContext.SeccompProfile = &corev1.SeccompProfile{}
			}
			if len(podSpec.SecurityContext.SeccompProfile.Type) == 0 {
				podSpec.SecurityContext.SeccompProfile.Type = corev1.SeccompProfileTypeRuntimeDefault
			}
		}
	}
	return nil
}

func AddNonRootSCCForOpenShift(cr *argoproj.ArgoCD, resource interface{}, client client.Client, args ...interface{}) error {
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
