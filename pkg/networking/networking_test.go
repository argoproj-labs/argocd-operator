package networking

import (
	"errors"

	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

var (
	testServiceNameMutated = "mutated-name"
	testRouteNameMutated   = "mutated-name"
	testIngressNameMutated = "mutated-name"
)

func testMutationFuncSuccessful(cr *argoproj.ArgoCD, resource interface{}, client cntrlClient.Client, args ...interface{}) error {
	switch obj := resource.(type) {
	case *corev1.Service:
		obj.Name = testServiceNameMutated
		return nil
	case *routev1.Route:
		obj.Name = testRouteNameMutated
		return nil
	case *networkingv1.Ingress:
		obj.Name = testIngressNameMutated
		return nil
	}
	return errors.New("test-mutation-error")
}
