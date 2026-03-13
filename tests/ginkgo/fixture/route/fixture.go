package route

import (
	"context"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	matcher "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"

	routev1 "github.com/openshift/api/route/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

func HaveConditionTypeStatus(expectedConditionType routev1.RouteIngressConditionType, expectedConditionStatus corev1.ConditionStatus) matcher.GomegaMatcher {
	return fetchRoute(func(r *routev1.Route) bool {

		GinkgoWriter.Println("Conditions:")
		for _, ingress := range r.Status.Ingress {

			for _, condition := range ingress.Conditions {
				GinkgoWriter.Println("-", condition.Type, condition.Status)
				if condition.Type == expectedConditionType && condition.Status == expectedConditionStatus {
					return true
				}
			}
		}

		return false
	})
}

func HavePort(targetPort intstr.IntOrString) matcher.GomegaMatcher {
	return fetchRoute(func(r *routev1.Route) bool {
		if r.Spec.Port == nil {
			return false
		}
		GinkgoWriter.Println("HavePort - expected: ", targetPort, "actual:", r.Spec.Port.TargetPort)
		return reflect.DeepEqual(r.Spec.Port.TargetPort, targetPort)
	})
}

func HaveTLS(termination routev1.TLSTerminationType, insecureEdgeTerminationPolicy routev1.InsecureEdgeTerminationPolicyType) matcher.GomegaMatcher {
	return fetchRoute(func(r *routev1.Route) bool {
		if r.Spec.TLS == nil {
			return false
		}
		GinkgoWriter.Println("HaveTLS - expected:", termination, insecureEdgeTerminationPolicy, "actual:", r.Spec.TLS.Termination, r.Spec.TLS.InsecureEdgeTerminationPolicy)
		return r.Spec.TLS.Termination == termination && r.Spec.TLS.InsecureEdgeTerminationPolicy == insecureEdgeTerminationPolicy
	})
}

// Whether the Route ingress status has been admitted:
// status:
//
//	ingress:
//	- conditions:
//	  - status: "True"
//	    type: Admitted
func HaveAdmittedIngress() matcher.GomegaMatcher {
	return fetchRoute(func(r *routev1.Route) bool {

		match := false
		for _, routeIngress := range r.Status.Ingress {

			for _, c := range routeIngress.Conditions {

				if c.Status == corev1.ConditionTrue && c.Type == routev1.RouteAdmitted {
					match = true
				}
			}
		}

		GinkgoWriter.Println("HaveAdmittedIngress - value:", match)

		return match
	})
}

func HaveTo(routeTR routev1.RouteTargetReference) matcher.GomegaMatcher {
	return fetchRoute(func(r *routev1.Route) bool {
		GinkgoWriter.Println("HaveTo - expected:", routeTR, "actual:", r.Spec.To)
		return reflect.DeepEqual(routeTR, r.Spec.To)
	})
}

// Update will keep trying to update object until it succeeds, or times out.
func Update(obj *routev1.Route, modify func(*routev1.Route)) {
	k8sClient, _ := utils.GetE2ETestKubeClient()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Retrieve the latest version of the object
		err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			return err
		}

		modify(obj)

		// Attempt to update the object
		return k8sClient.Update(context.Background(), obj)
	})
	Expect(err).ToNot(HaveOccurred())
}

// This is intentionally NOT exported, for now. Create another function in this file/package that calls this function, and export that.
func fetchRoute(f func(*routev1.Route) bool) matcher.GomegaMatcher {

	return WithTransform(func(route *routev1.Route) bool {

		k8sClient, _, err := utils.GetE2ETestKubeClientWithError()
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		err = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(route), route)
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		return f(route)

	}, BeTrue())

}
