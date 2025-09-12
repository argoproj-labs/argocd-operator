package role

import (
	"context"
	"reflect"

	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/ginkgo/v2" //nolint:all
	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/gomega" //nolint:all
	matcher "github.com/onsi/gomega/types"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Update will keep trying to update object until it succeeds, or times out.
func Update(obj *rbacv1.Role, modify func(*rbacv1.Role)) {
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
	return fetchRole(func(role *rbacv1.Role) bool {
		GinkgoWriter.Println("HaveRules -  Expected:", expectedRules, "/ Actual:", role.Rules)
		return reflect.DeepEqual(expectedRules, role.Rules)
	})
}

// This is intentionally NOT exported, for now. Create another function in this file/package that calls this function, and export that.
func fetchRole(f func(*rbacv1.Role) bool) matcher.GomegaMatcher {

	return WithTransform(func(role *rbacv1.Role) bool {

		k8sClient, _, err := utils.GetE2ETestKubeClientWithError()
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		err = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(role), role)
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		return f(role)

	}, BeTrue())

}
