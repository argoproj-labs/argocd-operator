// Copyright 2019 ArgoCD Operator Developers
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

package argocd

import (
	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var prometheusAPIFound = false

// GetPrometheusHost will return the hostname value for Prometheus.
func GetPrometheusHost(cr *argoprojv1a1.ArgoCD) string {
	host := nameWithSuffix("prometheus", cr)
	if len(cr.Spec.Prometheus.Host) > 0 {
		host = cr.Spec.Prometheus.Host
	}
	return host
}

// getPrometheusSize will return the size value for the Prometheus replica count.
func getPrometheusReplicas(cr *argoprojv1a1.ArgoCD) *int32 {
	replicas := common.ArgoCDDefaultPrometheusReplicas
	if cr.Spec.Prometheus.Size != nil {
		if *cr.Spec.Prometheus.Size >= 0 && *cr.Spec.Prometheus.Size != replicas {
			replicas = *cr.Spec.Prometheus.Size
		}
	}
	return &replicas
}

// IsPrometheusAPIAvailable returns true if the Prometheus API is present.
func IsPrometheusAPIAvailable() bool {
	return prometheusAPIFound
}

// hasPrometheusSpecChanged will return true if the supported properties differs in the actual versus the desired state.
func hasPrometheusSpecChanged(actual *monitoringv1.Prometheus, desired *argoprojv1a1.ArgoCD) bool {
	// Replica count
	if desired.Spec.Prometheus.Size != nil && *desired.Spec.Prometheus.Size >= 0 { // Valid replica count specified in desired state
		if actual.Spec.Replicas != nil { // Actual replicas value is set
			if *actual.Spec.Replicas != *desired.Spec.Prometheus.Size {
				return true
			}
		} else if *desired.Spec.Prometheus.Size != common.ArgoCDDefaultPrometheusReplicas { // Actual replicas value is NOT set, but desired replicas differs from the default
			return true
		}
	} else { // Replica count NOT specified in desired state
		if actual.Spec.Replicas != nil && *actual.Spec.Replicas != common.ArgoCDDefaultPrometheusReplicas {
			return true
		}
	}
	return false
}

// verifyPrometheusAPI will verify that the Prometheus API is present.
func verifyPrometheusAPI() error {
	found, err := argoutil.VerifyAPI(monitoringv1.SchemeGroupVersion.Group, monitoringv1.SchemeGroupVersion.Version)
	if err != nil {
		return err
	}
	prometheusAPIFound = found
	return nil
}

// newPrometheus returns a new Prometheus instance for the given ArgoCD.
func newPrometheus(cr *argoprojv1a1.ArgoCD) *monitoringv1.Prometheus {
	return &monitoringv1.Prometheus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}
