package openshift

import (
	"fmt"
	"strconv"

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
	mutation.Register(AddOAuthRedirectAnnotationForOpenShift)
}

// TO DO: Add dedicated e2e tests for all these mutations

// AddOAuthRedirectAnnotationForOpenShift adds the OAuth redirect URI annotation to the provided serviceaccount object, using the provided URI as value. This is only used for Dex, and only when OpenShift OAuth is requested
func AddOAuthRedirectAnnotationForOpenShift(cr *argoproj.ArgoCD, resource interface{}, client client.Client, args ...interface{}) error {
	if !IsOpenShiftEnv() {
		return nil
	}
	switch obj := resource.(type) {
	case *corev1.ServiceAccount:

		// ignore if service account does not belong to dex
		if component, ok := obj.GetLabels()[common.AppK8sKeyComponent]; !ok || component != common.DexServerComponent {
			return nil
		}

		if obj.Annotations == nil {
			obj.Annotations = make(map[string]string)
		}

		// Ensure that args carries only one argument, which is a map of type map[string]string
		// containing the keys "wantOpenShiftOAuth" and "redirect-uri". If this is the case, the associated value
		// can be used within the annotation if OpenShiftOAuth is requested
		if len(args) == 1 {
			for _, arg := range args {
				argMap := arg.(map[string]string)

				if val, ok := argMap[common.WantOpenShiftOAuth]; !ok {
					return nil
				} else {
					wantOpenShiftOAuth, err := strconv.ParseBool(val)
					if err != nil {
						return errors.Wrapf(err, "AddOAuthRedirectAnnotationForOpenShift: failed to parse mutation args for resource")
					}

					// return if autoTLS is not requested
					if !wantOpenShiftOAuth {
						return nil
					}
				}

				if val, ok := argMap[common.RedirectURI]; ok {
					obj.Annotations[common.SAOpenshiftKeyOAuthRedirectURI] = val
				}
			}
		}
	}
	return nil
}

// AddAutoTLSAnnotationForOpenShift adds the OpenShift Service CA TLS cert request annotaiton to the provided service object, using the provided secret name as the value
func AddAutoTLSAnnotationForOpenShift(cr *argoproj.ArgoCD, resource interface{}, client client.Client, args ...interface{}) error {
	if !IsOpenShiftEnv() {
		return nil
	}
	switch obj := resource.(type) {
	case *corev1.Service:

		if obj.Annotations == nil {
			obj.Annotations = make(map[string]string)
		}

		// Ensure that args carries only one argument, which is a map of type map[string]string
		// containing the keys "wantAutoTLS" and "tls-secret-name". If this is the case, the associated value
		// can be used within the service annotation if auto TLS is requested
		if len(args) == 1 {
			for _, arg := range args {
				argMap := arg.(map[string]string)

				if val, ok := argMap[common.WantAutoTLSKey]; !ok {
					return nil
				} else {
					wantTLS, err := strconv.ParseBool(val)
					if err != nil {
						return errors.Wrapf(err, "AddAutoTLSAnnotationForOpenShift: failed to parse mutation args for resource")
					}

					// return if autoTLS is not requested
					if !wantTLS {
						return nil
					}
				}

				if val, ok := argMap[common.TLSSecretNameKey]; ok {
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

	addSeccompProfile := func(podSpec *corev1.PodSpec) error {
		if !IsVersionAPIAvailable() {
			return nil
		}
		version, err := GetClusterVersion(client)
		if err != nil {
			return errors.Wrapf(err, "AddSeccompProfileForOpenShift: failed to retrieve OpenShift cluster version")
		}

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

			containers := []corev1.Container{}
			for _, container := range podSpec.Containers {
				if container.SecurityContext.SeccompProfile == nil {
					container.SecurityContext.SeccompProfile = &corev1.SeccompProfile{}
				}
				if len(container.SecurityContext.SeccompProfile.Type) == 0 {
					container.SecurityContext.SeccompProfile.Type = corev1.SeccompProfileTypeRuntimeDefault
				}
				containers = append(containers, container)
			}
			podSpec.Containers = containers

			initContainers := []corev1.Container{}
			for _, initc := range podSpec.InitContainers {
				if initc.SecurityContext.SeccompProfile == nil {
					initc.SecurityContext.SeccompProfile = &corev1.SeccompProfile{}
				}
				if len(initc.SecurityContext.SeccompProfile.Type) == 0 {
					initc.SecurityContext.SeccompProfile.Type = corev1.SeccompProfileTypeRuntimeDefault
				}
				initContainers = append(initContainers, initc)
			}
			podSpec.InitContainers = initContainers

		}
		return nil
	}

	switch obj := resource.(type) {
	case *appsv1.StatefulSet:
		return addSeccompProfile(&obj.Spec.Template.Spec)
	case *appsv1.Deployment:
		return addSeccompProfile(&obj.Spec.Template.Spec)
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
		if component, ok := obj.Labels[common.AppK8sKeyComponent]; !ok || (ok && component != common.RedisComponent) {
			return nil
		}

		if !IsVersionAPIAvailable() {
			return nil
		}
		// Starting with OpenShift 4.11, we need to use the resource name "nonroot-v2" instead of "nonroot"
		resourceName := "nonroot"
		version, err := GetClusterVersion(client)
		if err != nil {
			return errors.Wrapf(err, "AppendNonRootSCCForOpenShift: failed to retrieve OpenShift cluster version")
		}

		if version == "" || semver.Compare(fmt.Sprintf("v%s", version), "v4.10.999") > 0 {
			resourceName = "nonroot-v2"
		}

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

	return nil
}
