package clusterrolebinding

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	matcher "github.com/onsi/gomega/types"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

// This is intentionally NOT exported, for now. Create another function in this file/package that calls this function, and export that.
//
//nolint:unused
func fetchClusterRoleBinding(f func(*rbacv1.ClusterRoleBinding) bool) matcher.GomegaMatcher {

	return WithTransform(func(crb *rbacv1.ClusterRoleBinding) bool {

		k8sClient, _, err := utils.GetE2ETestKubeClientWithError()
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		err = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(crb), crb)
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		return f(crb)

	}, BeTrue())

}
