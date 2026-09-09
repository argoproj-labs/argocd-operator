// Copyright 2026 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package argoutil

import (
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

// EnsureMetricsServiceAnnotations ensures custom annotations from the metrics spec are applied to the service.
// It adds/updates annotations specified in the metrics spec and removes any previously set annotations
// that are no longer in the spec, while preserving Kubernetes-managed annotations.
// Returns true if annotations were modified, false otherwise.
//
// This function is intentionally designed to remove user-added annotations that are not in the spec,
// treating the spec as the source of truth. Only system annotations (kubernetes.io/, k8s.io/, openshift.io/)
// are preserved to maintain proper system functionality.
func EnsureMetricsServiceAnnotations(svc *corev1.Service, metricsSpec *argoproj.ArgoCDMetricsSpec) bool {
	if svc.Annotations == nil {
		svc.Annotations = make(map[string]string)
	}

	// Build the desired annotations map
	desired := make(map[string]string)

	// Add custom annotations from spec
	if metricsSpec != nil && len(metricsSpec.Annotations) > 0 {
		for k, v := range metricsSpec.Annotations {
			desired[k] = v
		}
	}

	// Preserve Kubernetes and OpenShift managed annotations
	// This prevents removing system annotations like serving-cert-secret-name
	preservedPatterns := []string{
		"kubernetes.io/",
		"k8s.io/",
		"openshift.io/",
		"service.beta.openshift.io/",
		"service.alpha.openshift.io/",
	}

	for key, value := range svc.Annotations {
		shouldPreserve := false
		for _, pattern := range preservedPatterns {
			if strings.HasPrefix(key, pattern) {
				shouldPreserve = true
				break
			}
		}
		if shouldPreserve {
			desired[key] = value
		}
	}

	// Check if update is needed
	if !reflect.DeepEqual(desired, svc.Annotations) {
		svc.Annotations = desired
		return true
	}

	return false
}
