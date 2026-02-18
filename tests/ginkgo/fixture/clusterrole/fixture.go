package clusterrole

import (
	"context"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	matcher "github.com/onsi/gomega/types"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

// Update will keep trying to update object until it succeeds, or times out.
func Update(obj *rbacv1.ClusterRole, modify func(*rbacv1.ClusterRole)) {
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

func HaveRules(expectedRules []rbacv1.PolicyRule) matcher.GomegaMatcher {
	return fetchRole(func(cr *rbacv1.ClusterRole) bool {
		GinkgoWriter.Println("HaveRules - Expected:", expectedRules, "/ Actual:", cr.Rules)
		return reflect.DeepEqual(expectedRules, cr.Rules)
	})
}

// This is intentionally NOT exported, for now. Create another function in this file/package that calls this function, and export that.
//
//nolint:unused
func fetchRole(f func(*rbacv1.ClusterRole) bool) matcher.GomegaMatcher {

	return WithTransform(func(cr *rbacv1.ClusterRole) bool {

		k8sClient, _, err := utils.GetE2ETestKubeClientWithError()
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		err = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(cr), cr)
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		return f(cr)

	}, BeTrue())

}
