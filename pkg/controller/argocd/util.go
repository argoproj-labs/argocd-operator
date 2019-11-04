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
	"context"

	argoproj "github.com/jmckind/argocd-operator/pkg/apis/argoproj/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	// ArgoCDAppName is the application name for labels.
	ArgoCDAppName = "argocd"

	// ArgoCDNameKey is the resource name key for labels.
	ArgoCDNameKey = "app.kubernetes.io/name"

	// ArgoCDPartOfKey is the resource part-of key for labels.
	ArgoCDPartOfKey = "app.kubernetes.io/part-of"
)

var isOpenshiftCluster = false

// isObjectFound will perform a basic check that the given object exists via the Kubernetes API.
// If an error occurs as part of the check, the function will return false.
func (r *ReconcileArgoCD) isObjectFound(nsname types.NamespacedName, obj runtime.Object) bool {
	err := r.client.Get(context.TODO(), nsname, obj)
	if err != nil {
		return false
	}
	return true
}

// IsOpenShift returns true if the operator is running in an OpenShift environment.
func IsOpenShift() bool {
	return isOpenshiftCluster
}

// VerifyOpenShift will verify that the OpenShift API is present, indicating an OpenShift cluster.
func VerifyOpenShift() error {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to get k8s config")
		return err
	}

	k8s, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Error(err, "unable to create k8s client")
		return err
	}

	gv := schema.GroupVersion{
		Group:   routev1.GroupName,
		Version: routev1.GroupVersion.Version,
	}

	err = discovery.ServerSupportsVersion(k8s, gv)
	if err == nil {
		log.Info("openshift verified")
		isOpenshiftCluster = true
	}
	return nil
}

// reconcileCertificateAuthority will reconcile all Certificate Authority resources.
func (r *ReconcileArgoCD) reconcileCertificateAuthority(cr *argoproj.ArgoCD) error {
	if err := r.reconcileCASecret(cr); err != nil {
		return err
	}

	if err := r.reconcileCAConfigMap(cr); err != nil {
		return err
	}
	return nil
}

// reconcileOpenShiftResources will reconcile OpenShift specific ArgoCD resources.
func (r *ReconcileArgoCD) reconcileOpenShiftResources(cr *argoproj.ArgoCD) error {
	if err := r.reconcileRoutes(cr); err != nil {
		return err
	}

	if err := r.reconcilePrometheus(cr); err != nil {
		return err
	}

	if err := r.reconcileMetricsServiceMonitor(cr); err != nil {
		return err
	}

	if err := r.reconcileRepoServerServiceMonitor(cr); err != nil {
		return err
	}

	if err := r.reconcileServerMetricsServiceMonitor(cr); err != nil {
		return err
	}
	return nil
}

// reconcileResources will reconcile common ArgoCD resources.
func (r *ReconcileArgoCD) reconcileResources(cr *argoproj.ArgoCD) error {
	if err := r.reconcileCertificateAuthority(cr); err != nil {
		return err
	}

	if err := r.reconcileSecrets(cr); err != nil {
		return err
	}

	if err := r.reconcileConfigMaps(cr); err != nil {
		return err
	}

	if err := r.reconcileServices(cr); err != nil {
		return err
	}

	if err := r.reconcileDeployments(cr); err != nil {
		return err
	}
	return nil
}

// defaultLabels returns the default set of labels for the cluster.
func defaultLabels(cr *argoproj.ArgoCD) map[string]string {
	return map[string]string{
		ArgoCDNameKey:   cr.Name,
		ArgoCDPartOfKey: ArgoCDAppName,
	}
}

// labelsForCluster returns the labels for all cluster resources.
func labelsForCluster(cr *argoproj.ArgoCD) map[string]string {
	labels := defaultLabels(cr)
	for key, val := range cr.ObjectMeta.Labels {
		labels[key] = val
	}
	return labels
}

// setDefaults sets the default vaules for the spec and returns true if the spec was changed.
func setDefaults(cr *argoproj.ArgoCD) bool {
	changed := false
	return changed
}

// withClusterLabels will add the given labels to the labels for the cluster and return the result.
func withClusterLabels(cr *argoproj.ArgoCD, addLabels map[string]string) map[string]string {
	labels := labelsForCluster(cr)
	for key, val := range addLabels {
		labels[key] = val
	}
	return labels
}
