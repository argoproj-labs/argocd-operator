package clusterserviceversion

import (
	"context"

	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
	. "github.com/onsi/gomega"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Update will update a ClusterServiceVersion CR. Update will keep trying to update object until it succeeds, or times out.
func Update(obj *olmv1alpha1.ClusterServiceVersion, modify func(*olmv1alpha1.ClusterServiceVersion)) {
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
