package argoutil

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestNewPVCResourceRequirements(t *testing.T) {
	tests := []struct {
		name     string
		capacity resource.Quantity
		expected resource.Quantity
	}{
		{
			"Capacity 512 Mi",
			resource.MustParse("512Mi"),
			resource.MustParse("512Mi"),
		},
		{
			"Capacity 1 Gi",
			resource.MustParse("1Gi"),
			resource.MustParse("1Gi"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewPVCResourceRequirements(tt.capacity)
			assert.Equal(t, corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": tt.expected,
				},
			}, got)
		})
	}
}

func TestNewPersistentVolumeClaim(t *testing.T) {
	meta := metav1.ObjectMeta{
		Name:      "test-pvc",
		Namespace: "test-namespace",
	}

	expectedLabels := map[string]string{"app.kubernetes.io/managed-by": "test-pvc", "app.kubernetes.io/name": "test-pvc", "app.kubernetes.io/part-of": "argocd"}
	pvc := NewPersistentVolumeClaim(meta)
	assert.Equal(t, "test-pvc", pvc.Name)
	assert.Equal(t, "test-namespace", pvc.Namespace)
	assert.Equal(t, expectedLabels, pvc.Labels)
}

func TestNewPersistentVolumeClaimWithName(t *testing.T) {
	meta := metav1.ObjectMeta{
		Name:      "test-pvc",
		Namespace: "test-namespace",
	}

	tests := []struct {
		name     string
		expected map[string]string
	}{
		{
			"test-pvc-1",
			map[string]string{"app.kubernetes.io/managed-by": "test-pvc", "app.kubernetes.io/name": "test-pvc-1", "app.kubernetes.io/part-of": "argocd"},
		},
		{
			"test-pvc-2",
			map[string]string{"app.kubernetes.io/managed-by": "test-pvc", "app.kubernetes.io/name": "test-pvc-2", "app.kubernetes.io/part-of": "argocd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvc := NewPersistentVolumeClaimWithName(tt.name, meta)

			assert.Equal(t, tt.name, pvc.Name)
			assert.Equal(t, "test-namespace", pvc.Namespace)
			assert.Equal(t, tt.expected, pvc.Labels)
		})
	}
}
