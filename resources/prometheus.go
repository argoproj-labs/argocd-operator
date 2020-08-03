// Copyright 2019 Argo CD Operator Developers
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

package resources

import (
	"fmt"

	"github.com/argoproj-labs/argocd-operator/common"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var prometheusAPIFound = false

// IsPrometheusAPIAvailable returns true if the Prometheus API is present.
func IsPrometheusAPIAvailable() bool {
	return prometheusAPIFound
}

// verifyPrometheusAPI will verify that the Prometheus API is present.
func verifyPrometheusAPI() error {
	found, err := VerifyAPI(monitoringv1.SchemeGroupVersion.Group, monitoringv1.SchemeGroupVersion.Version)
	if err != nil {
		return err
	}
	prometheusAPIFound = found
	return nil
}

// NewPrometheus returns a new Prometheus instance for the given ArgoCD.
func NewPrometheus(meta metav1.ObjectMeta) *monitoringv1.Prometheus {
	return &monitoringv1.Prometheus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: meta.Namespace,
			Labels:    common.LabelsForCluster(meta),
		},
	}
}

// NewServiceMonitor returns a new ServiceMonitor instance.
func NewServiceMonitor(meta metav1.ObjectMeta) *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: meta.Namespace,
			Labels:    common.LabelsForCluster(meta),
		},
	}
}

// NewServiceMonitorWithName returns a new ServiceMonitor instance for the given ObjectMeta and name.
func NewServiceMonitorWithName(meta metav1.ObjectMeta, name string) *monitoringv1.ServiceMonitor {
	svcmon := NewServiceMonitor(meta)
	svcmon.ObjectMeta.Name = name

	lbls := svcmon.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyRelease] = "prometheus-operator"
	svcmon.ObjectMeta.Labels = lbls

	return svcmon
}

// NewServiceMonitorWithSuffix returns a new ServiceMonitor instance for the given ArgoCD using the given suffix.
func NewServiceMonitorWithSuffix(meta metav1.ObjectMeta, suffix string) *monitoringv1.ServiceMonitor {
	return NewServiceMonitorWithName(meta, fmt.Sprintf("%s-%s", meta.Name, suffix))
}
