package clientwrapper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj-labs/argocd-operator/common"
)

func makeTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	return scheme
}

func TestClientWrapper_Get_FoundInCache(t *testing.T) {
	// Setup test ConfigMap
	testCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "test-ns",
		},
		Data: map[string]string{"key": "value"},
	}

	// Create cached client with the ConfigMap already present
	scheme := makeTestScheme()
	cachedClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testCM).
		Build()

	// Create live client (empty - shouldn't be called)
	liveClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	wrapper := NewClientWrapper(cachedClient, liveClient)

	// Test
	ctx := context.Background()
	key := types.NamespacedName{Name: "test-cm", Namespace: "test-ns"}
	result := &corev1.ConfigMap{}

	err := wrapper.Get(ctx, key, result)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "test-cm", result.Name)
	assert.Equal(t, "test-ns", result.Namespace)
	assert.Equal(t, "value", result.Data["key"])
}

func TestClientWrapper_Get_NotInCache_FoundLive_SuccessfulLabel(t *testing.T) {
	// Setup test ConfigMap that exists live but not in cache
	testCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "test-ns",
		},
		Data: map[string]string{"key": "value"},
	}

	// Create empty cached client
	scheme := makeTestScheme()
	cachedClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	// Create live client with the ConfigMap
	liveClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testCM).
		Build()

	wrapper := NewClientWrapper(cachedClient, liveClient)

	// Test
	ctx := context.Background()
	key := types.NamespacedName{Name: "test-cm", Namespace: "test-ns"}
	result := &corev1.ConfigMap{}

	err := wrapper.Get(ctx, key, result)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "test-cm", result.Name)
	assert.Equal(t, "test-ns", result.Namespace)
	assert.Equal(t, "value", result.Data["key"])

	// Verify the label was added
	expectedLabel := common.ArgoCDAppName
	assert.Equal(t, expectedLabel, result.Labels[common.ArgoCDTrackedByOperatorLabel])

	// Verify the resource is now labeled in the live client
	updatedCM := &corev1.ConfigMap{}
	err = liveClient.Get(ctx, key, updatedCM)
	assert.NoError(t, err)
	assert.Equal(t, expectedLabel, updatedCM.Labels[common.ArgoCDTrackedByOperatorLabel])
}

func TestClientWrapper_Get_NotFoundAnywhere(t *testing.T) {
	// Create empty clients (no objects)
	scheme := makeTestScheme()
	cachedClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	liveClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	wrapper := NewClientWrapper(cachedClient, liveClient)

	// Test
	ctx := context.Background()
	key := types.NamespacedName{Name: "nonexistent-cm", Namespace: "test-ns"}
	result := &corev1.ConfigMap{}

	err := wrapper.Get(ctx, key, result)

	// Assert
	assert.Error(t, err)
	assert.True(t, kerrors.IsNotFound(err))
}
