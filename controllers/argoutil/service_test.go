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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

func TestEnsureMetricsServiceAnnotations(t *testing.T) {
	t.Run("Apply annotations to service without existing annotations", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "test-namespace",
			},
		}

		metricsSpec := &argoproj.ArgoCDMetricsSpec{
			Annotations: map[string]string{
				"ad.datadoghq.com/service.check_names":  `["openmetrics"]`,
				"ad.datadoghq.com/service.init_configs": `[{}]`,
				"custom.io/annotation":                  "value",
			},
		}

		modified := EnsureMetricsServiceAnnotations(svc, metricsSpec)
		if !modified {
			t.Error("Expected annotations to be modified")
		}
		if svc.Annotations["ad.datadoghq.com/service.check_names"] != `["openmetrics"]` {
			t.Errorf("Expected check_names annotation, got %v", svc.Annotations)
		}
		if svc.Annotations["ad.datadoghq.com/service.init_configs"] != `[{}]` {
			t.Error("Expected init_configs annotation")
		}
		if svc.Annotations["custom.io/annotation"] != "value" {
			t.Error("Expected custom annotation")
		}
	})

	t.Run("Apply annotations to service with existing non-k8s annotations removes old annotations", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "test-namespace",
				Annotations: map[string]string{
					"existing.io/annotation": "existing-value",
				},
			},
		}

		metricsSpec := &argoproj.ArgoCDMetricsSpec{
			Annotations: map[string]string{
				"custom.io/annotation": "value",
			},
		}

		modified := EnsureMetricsServiceAnnotations(svc, metricsSpec)
		if !modified {
			t.Error("Expected annotations to be modified")
		}
		if svc.Annotations["custom.io/annotation"] != "value" {
			t.Error("Expected custom annotation")
		}
		// Old annotation should be removed since it's not in spec and not a k8s annotation
		if _, hasExisting := svc.Annotations["existing.io/annotation"]; hasExisting {
			t.Error("Old annotation should have been removed")
		}
	})

	t.Run("No modification when annotations already match", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "test-namespace",
				Annotations: map[string]string{
					"custom.io/annotation": "value",
				},
			},
		}

		metricsSpec := &argoproj.ArgoCDMetricsSpec{
			Annotations: map[string]string{
				"custom.io/annotation": "value",
			},
		}

		modified := EnsureMetricsServiceAnnotations(svc, metricsSpec)
		if modified {
			t.Error("Expected no modifications")
		}
	})

	t.Run("Remove all annotations when metricsSpec is nil", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "test-namespace",
				Annotations: map[string]string{
					"custom.io/annotation": "value",
				},
			},
		}

		modified := EnsureMetricsServiceAnnotations(svc, nil)
		if !modified {
			t.Error("Expected annotations to be modified")
		}
		// All non-k8s annotations should be removed
		if _, hasCustom := svc.Annotations["custom.io/annotation"]; hasCustom {
			t.Error("Custom annotation should have been removed")
		}
	})

	t.Run("Remove all annotations when metricsSpec has no annotations", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "test-namespace",
				Annotations: map[string]string{
					"custom.io/annotation": "value",
				},
			},
		}

		metricsSpec := &argoproj.ArgoCDMetricsSpec{
			Interval: "30s",
		}

		modified := EnsureMetricsServiceAnnotations(svc, metricsSpec)
		if !modified {
			t.Error("Expected annotations to be modified")
		}
		// All non-k8s annotations should be removed
		if _, hasCustom := svc.Annotations["custom.io/annotation"]; hasCustom {
			t.Error("Custom annotation should have been removed")
		}
	})

	t.Run("Update existing annotation value", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "test-namespace",
				Annotations: map[string]string{
					"custom.io/annotation": "old-value",
				},
			},
		}

		metricsSpec := &argoproj.ArgoCDMetricsSpec{
			Annotations: map[string]string{
				"custom.io/annotation": "new-value",
			},
		}

		modified := EnsureMetricsServiceAnnotations(svc, metricsSpec)
		if !modified {
			t.Error("Expected annotations to be modified")
		}
		if svc.Annotations["custom.io/annotation"] != "new-value" {
			t.Error("Expected annotation to be updated")
		}
	})

	t.Run("Preserve Kubernetes-managed annotations", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "test-namespace",
				Annotations: map[string]string{
					"service.beta.openshift.io/serving-cert-secret-name": "my-secret",
					"kubernetes.io/service-name":                         "my-service",
					"custom.io/old-annotation":                           "old-value",
				},
			},
		}

		metricsSpec := &argoproj.ArgoCDMetricsSpec{
			Annotations: map[string]string{
				"custom.io/annotation": "value",
			},
		}

		modified := EnsureMetricsServiceAnnotations(svc, metricsSpec)
		if !modified {
			t.Error("Expected annotations to be modified")
		}
		if svc.Annotations["custom.io/annotation"] != "value" {
			t.Error("Expected custom annotation")
		}
		// K8s annotations should be preserved
		if svc.Annotations["service.beta.openshift.io/serving-cert-secret-name"] != "my-secret" {
			t.Error("Expected OpenShift annotation to be preserved")
		}
		if svc.Annotations["kubernetes.io/service-name"] != "my-service" {
			t.Error("Expected Kubernetes annotation to be preserved")
		}
		// Old custom annotation should be removed
		if _, hasOld := svc.Annotations["custom.io/old-annotation"]; hasOld {
			t.Error("Old custom annotation should have been removed")
		}
	})

	t.Run("Remove annotation when deleted from spec", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "test-namespace",
				Annotations: map[string]string{
					"custom.io/annotation": "value",
					"to-be-removed":        "old-value",
				},
			},
		}

		metricsSpec := &argoproj.ArgoCDMetricsSpec{
			Annotations: map[string]string{
				"custom.io/annotation": "value",
			},
		}

		modified := EnsureMetricsServiceAnnotations(svc, metricsSpec)
		if !modified {
			t.Error("Expected annotations to be modified")
		}
		if svc.Annotations["custom.io/annotation"] != "value" {
			t.Error("Expected custom annotation")
		}
		if _, hasRemoved := svc.Annotations["to-be-removed"]; hasRemoved {
			t.Error("Removed annotation should not be present")
		}
	})
}
