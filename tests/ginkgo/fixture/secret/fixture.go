package secret

import (
	"bytes"
	"context"

	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/ginkgo/v2" //nolint:all
	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/gomega" //nolint:all
	matcher "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Update will keep trying to update object until it succeeds, or times out.
func Update(obj *corev1.Secret, modify func(*corev1.Secret)) {
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

// Update will keep trying to update object until it succeeds, or times out.
func UpdateWithError(obj *corev1.Secret, modify func(*corev1.Secret)) error {
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

	return err
}

// HaveNonEmptyKeyValue returns true if Secret has the given key, and the value of the key is non-empty
func HaveNonEmptyKeyValue(key string) matcher.GomegaMatcher {
	return fetchSecret(func(sec *corev1.Secret) bool {
		a, exists := sec.Data[key]
		if !exists {
			GinkgoWriter.Println("HaveNonEmptyKeyValue - Key:", key, "does not exist")
			return false
		}

		GinkgoWriter.Println("HaveNonEmptyKeyValue - Key:", key, " Have:", string(a))

		return len(a) > 0
	})

}

// HaveStringDataKeyValue returns true if Secret has 'key' field under .data map, and the value of that field is equal to 'value'
func HaveStringDataKeyValue(key string, value string) matcher.GomegaMatcher {
	return fetchSecret(func(sec *corev1.Secret) bool {
		a, exists := sec.Data[key]
		if !exists {
			GinkgoWriter.Println("HaveStringDataKeyValue - Key:", key, "does not exist")
			return false
		}

		GinkgoWriter.Println("HaveStringDataKeyValue - Key:", key, "Expected:", value, "/ Have:", string(a))

		return string(a) == value
	})

}

// HaveDataKeyValue returns true if Secret has 'key' field under .data map, and the value of that field is equal to 'value'
func HaveDataKeyValue(key string, value []byte) matcher.GomegaMatcher {
	return fetchSecret(func(sec *corev1.Secret) bool {
		a, exists := sec.Data[key]
		if !exists {
			return false
		}
		return bytes.Equal(a, value)
	})

}

// NotHaveDataKey returns true if Secret's .data 'key' does not exist, false otherwise
func NotHaveDataKey(key string) matcher.GomegaMatcher {
	return fetchSecret(func(secret *corev1.Secret) bool {
		_, exists := secret.Data[key]
		GinkgoWriter.Println("NotHaveDataKey - key:", key, "Exists:", exists)
		return !exists
	})

}

// This is intentionally NOT exported, for now. Create another function in this file/package that calls this function, and export that.
func fetchSecret(f func(*corev1.Secret) bool) matcher.GomegaMatcher {

	return WithTransform(func(secret *corev1.Secret) bool {

		k8sClient, _, err := utils.GetE2ETestKubeClientWithError()
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		err = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(secret), secret)
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		return f(secret)

	}, BeTrue())

}
