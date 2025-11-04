package cacheutils

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotools "k8s.io/client-go/tools/cache"

	"github.com/argoproj-labs/argocd-operator/common"
)

// StripDataFromSecretOrConfigMap returns a TransformFunc that strips data from both
// Secrets and ConfigMaps that are not tracked by the operator. This unified function
// handles both resource types in a single transform. This is useful for reducing memory usage
// when caching Secrets and ConfigMaps that are not managed/used by the operator.
func StripDataFromSecretOrConfigMapTransform() clientgotools.TransformFunc {
	return func(in interface{}) (interface{}, error) {
		if s, ok := in.(*v1.Secret); ok {
			// Keep full secret for operator-managed resources
			if IsTrackedByOperator(s) {
				return in, nil
			}
			if s.Data != nil || s.StringData != nil {
				// Strip data fields from non-operator secrets to reduce memory usage
				s.Data = nil
				s.StringData = nil
			}
			return s, nil
		}

		if cm, ok := in.(*v1.ConfigMap); ok {
			// Keep full configmap for operator-managed resources
			if IsTrackedByOperator(cm) {
				return in, nil
			}
			if cm.Data != nil || cm.BinaryData != nil {
				// Strip data fields from non-operator configmaps to reduce memory usage
				cm.Data = nil
				cm.BinaryData = nil
			}
			return cm, nil
		}

		return in, nil
	}
}

// IsTrackedByOperator checks if the given labels indicate that the resource is tracked by the operator.
// A resource is considered tracked if it has any of the following labels:
// - 'operator.argoproj.io/tracked-by': indicates the resource is managed/used by the operator
// - 'argocd.argoproj.io/secret-type': indicates the resource is an ArgoCD-specific secret type
func IsTrackedByOperator(obj runtime.Object) bool {
	// List of labels that indicate operator tracking
	trackedLabels := []string{
		common.ArgoCDTrackedByOperatorLabel,
		common.ArgoCDSecretTypeLabel,
	}

	// Get labels from the object's metadata
	var labels map[string]string
	if obj != nil {
		// Use meta.Accessor to get object labels (if obj implements metav1.Object)
		if accessor, err := meta.Accessor(obj); err == nil {
			labels = accessor.GetLabels()
		}
	}

	// Check if any tracking label exists
	for _, l := range trackedLabels {
		if _, exists := labels[l]; exists {
			return true
		}
	}
	return false
}
