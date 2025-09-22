package cacheutils

import (
	v1 "k8s.io/api/core/v1"
	clientgotools "k8s.io/client-go/tools/cache"

	"github.com/argoproj-labs/argocd-operator/common"
)

// StripSecretDataTransform returns a TransformFunc that strips the data from Secrets
// that are not tracked by the operator. This is useful for reducing memory usage
// when caching Secrets that are not managed by the operator.
func StripSecretDataTransform() clientgotools.TransformFunc {
	return func(in interface{}) (interface{}, error) {
		if s, ok := in.(*v1.Secret); ok {
			if IsTrackedByOperator(s.Labels) {
				// Keep full secret
				return in, nil
			}
			// Strip data for non-operator secrets
			return &v1.Secret{
				TypeMeta:   s.TypeMeta,
				ObjectMeta: s.ObjectMeta,
				Type:       s.Type,
				Immutable:  s.Immutable,
				Data:       nil,
				StringData: nil,
			}, nil
		}
		return in, nil
	}
}

// StripConfigMapDataTransform returns a TransformFunc that strips the data from ConfigMaps
// that are not tracked by the operator. This is useful for reducing memory usage
// when caching ConfigMaps that are not managed by the operator.
func StripConfigMapDataTransform() clientgotools.TransformFunc {
	return func(in interface{}) (interface{}, error) {
		if cm, ok := in.(*v1.ConfigMap); ok {
			if IsTrackedByOperator(cm.Labels) {
				// Keep full configmap
				return in, nil
			}
			// Strip data for non-operator configmaps
			return &v1.ConfigMap{
				TypeMeta:   cm.TypeMeta,
				ObjectMeta: cm.ObjectMeta,
				Data:       nil,
				BinaryData: nil,
			}, nil
		}
		return in, nil
	}
}

// IsTrackedByOperator checks if the given labels indicate that the resource is tracked by the operator.
func IsTrackedByOperator(labels map[string]string) bool {
	trackedLabels := []string{
		common.ArgoCDTrackedByOperatorLabel,
		common.ArgoCDSecretTypeLabel,
	}

	for _, l := range trackedLabels {
		if _, exists := labels[l]; exists {
			return true
		}
	}
	return false
}
