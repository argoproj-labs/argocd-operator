// Copyright 2020 Argo CD Operator Developers
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
	"fmt"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/resources"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var log = ctrl.Log.WithName("argocd")

// ArgoClusterReconciler represents the reconcile process of an Argo CD cluster.
type ArgoClusterReconciler struct {
	Client client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// NewArgoClusterReconciler will return a new ArgoClusterReconciler instance populated with the given arguments.
func NewArgoClusterReconciler(client client.Client, log logr.Logger, scheme *runtime.Scheme) *ArgoClusterReconciler {
	return &ArgoClusterReconciler{
		Client: client,
		Log:    log,
		Scheme: scheme,
	}
}

// getArgoContainerImage will return the container image for ArgoCD.
func getArgoContainerImage(cr *v1alpha1.ArgoCD) string {
	img := cr.Spec.Image
	if len(img) <= 0 {
		img = common.ArgoCDDefaultArgoImage
	}

	tag := cr.Spec.Version
	if len(tag) <= 0 {
		tag = common.ArgoCDDefaultArgoVersion
	}

	return common.CombineImageTag(img, tag)
}

// reconcileAutoscalers will ensure that all HorizontalPodAutoscalers are present for the given ArgoCD.
func (r *ArgoClusterReconciler) reconcileAutoscalers(cr *v1alpha1.ArgoCD) error {
	if err := r.reconcileServerHPA(cr); err != nil {
		return err
	}
	return nil
}

// reconcileClusterSecrets will reconcile all Secret resources for the ArgoCD cluster.
func (r *ArgoClusterReconciler) reconcileClusterSecrets(cr *v1alpha1.ArgoCD) error {
	if err := r.reconcileClusterMainSecret(cr); err != nil {
		return err
	}

	if err := r.reconcileClusterCASecret(cr); err != nil {
		return err
	}

	if err := r.reconcileClusterTLSSecret(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaSecret(cr); err != nil {
		return err
	}

	return nil
}

// reconcileConfigMaps will ensure that all ArgoCD ConfigMaps are present.
func (r *ArgoClusterReconciler) reconcileConfigMaps(cr *v1alpha1.ArgoCD) error {
	if err := r.reconcileArgoConfigMap(cr); err != nil {
		return err
	}

	if err := r.reconcileRedisConfiguration(cr); err != nil {
		return err
	}

	if err := r.reconcileRBAC(cr); err != nil {
		return err
	}

	if err := r.reconcileSSHKnownHosts(cr); err != nil {
		return err
	}

	if err := r.reconcileTLSCerts(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaConfiguration(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaDashboards(cr); err != nil {
		return err
	}

	return nil
}

// reconcileDeployments will ensure that all Deployment resources are present for the given ArgoCD.
func (r *ArgoClusterReconciler) reconcileDeployments(cr *v1alpha1.ArgoCD) error {
	err := r.reconcileApplicationControllerDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileDexDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRedisDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRedisHAProxyDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRepoDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileServerDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileGrafanaDeployment(cr)
	if err != nil {
		return err
	}

	return nil
}

// reconcileIngresses will ensure that all ArgoCD Ingress resources are present.
func (r *ArgoClusterReconciler) reconcileIngresses(cr *v1alpha1.ArgoCD) error {
	if err := r.reconcileArgoServerIngress(cr); err != nil {
		return err
	}

	if err := r.reconcileArgoServerGRPCIngress(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaIngress(cr); err != nil {
		return err
	}

	if err := r.reconcilePrometheusIngress(cr); err != nil {
		return err
	}
	return nil
}

// reconcileResources will reconcile common ArgoCD resources.
func (r *ArgoClusterReconciler) ReconcileResources(cr *v1alpha1.ArgoCD) error {
	log.Info("reconciling status")
	if err := r.reconcileStatus(cr); err != nil {
		return err
	}

	log.Info("reconciling service accounts")
	if err := r.reconcileServiceAccounts(cr); err != nil {
		return err
	}

	log.Info("reconciling certificate authority")
	if err := r.reconcileCertificateAuthority(cr); err != nil {
		return err
	}

	log.Info("reconciling secrets")
	if err := r.reconcileSecrets(cr); err != nil {
		return err
	}

	log.Info("reconciling config maps")
	if err := r.reconcileConfigMaps(cr); err != nil {
		return err
	}

	log.Info("reconciling services")
	if err := r.reconcileServices(cr); err != nil {
		return err
	}

	log.Info("reconciling deployments")
	if err := r.reconcileDeployments(cr); err != nil {
		return err
	}

	log.Info("reconciling statefulsets")
	if err := r.reconcileStatefulSets(cr); err != nil {
		return err
	}

	log.Info("reconciling autoscalers")
	if err := r.reconcileAutoscalers(cr); err != nil {
		return err
	}

	log.Info("reconciling ingresses")
	if err := r.reconcileIngresses(cr); err != nil {
		return err
	}

	if resources.IsRouteAPIAvailable() {
		log.Info("reconciling routes")
		if err := r.reconcileRoutes(cr); err != nil {
			return err
		}
	}

	if resources.IsPrometheusAPIAvailable() {
		log.Info("reconciling prometheus")
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
	}

	return nil
}

// reconcileRoutes will ensure that all ArgoCD Routes are present.
func (r *ArgoClusterReconciler) reconcileRoutes(cr *v1alpha1.ArgoCD) error {
	if err := r.reconcileGrafanaRoute(cr); err != nil {
		return err
	}

	if err := r.reconcilePrometheusRoute(cr); err != nil {
		return err
	}

	if err := r.reconcileServerRoute(cr); err != nil {
		return err
	}
	return nil
}

// reconcileSecrets will reconcile all ArgoCD Secret resources.
func (r *ArgoClusterReconciler) reconcileSecrets(cr *v1alpha1.ArgoCD) error {
	if err := r.reconcileClusterSecrets(cr); err != nil {
		return err
	}

	if err := r.reconcileArgoSecret(cr); err != nil {
		return err
	}

	return nil
}

// reconcileServices will ensure that all Services are present for the given ArgoCD.
func (r *ArgoClusterReconciler) reconcileServices(cr *v1alpha1.ArgoCD) error {
	err := r.reconcileDexService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileGrafanaService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileMetricsService(cr)
	if err != nil {
		return err
	}

	if cr.Spec.HA.Enabled {
		err = r.reconcileRedisHAServices(cr)
		if err != nil {
			return err
		}
	} else {
		err = r.reconcileRedisService(cr)
		if err != nil {
			return err
		}
	}

	err = r.reconcileRepoService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileServerMetricsService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileServerService(cr)
	if err != nil {
		return err
	}
	return nil
}

// reconcileServiceAccounts will ensure that all ArgoCD Service Accounts are configured.
func (r *ArgoClusterReconciler) reconcileServiceAccounts(cr *v1alpha1.ArgoCD) error {
	if err := r.reconcileDexServiceAccount(cr); err != nil {
		return err
	}
	return nil
}

// reconcileStatefulSets will ensure that all StatefulSets are present for the given ArgoCD.
func (r *ArgoClusterReconciler) reconcileStatefulSets(cr *v1alpha1.ArgoCD) error {
	if err := r.reconcileRedisStatefulSet(cr); err != nil {
		return nil
	}
	return nil
}

// reconcileStatus will ensure that all of the Status properties are updated for the given ArgoCD.
func (r *ArgoClusterReconciler) reconcileStatus(cr *v1alpha1.ArgoCD) error {
	if err := r.reconcileStatusApplicationController(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusDex(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusPhase(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusRedis(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusRepo(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusServer(cr); err != nil {
		return err
	}
	return nil
}

// reconcileStatusPhase will ensure that the Status Phase is updated for the given ArgoCD.
func (r *ArgoClusterReconciler) reconcileStatusPhase(cr *v1alpha1.ArgoCD) error {
	phase := "Unknown"

	if cr.Status.ApplicationController == "Running" && cr.Status.Redis == "Running" && cr.Status.Repo == "Running" && cr.Status.Server == "Running" {
		phase = "Available"
	} else {
		phase = "Pending"
	}

	if cr.Status.Phase != phase {
		cr.Status.Phase = phase
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}

// triggerRollout will update the label with the given key to trigger a new rollout of the Deployment.
func (r *ArgoClusterReconciler) triggerRollout(deployment *appsv1.Deployment, key string) error {
	if !resources.IsObjectFound(r.Client, deployment.Namespace, deployment.Name, deployment) {
		log.Info(fmt.Sprintf("unable to locate deployment with name: %s", deployment.Name))
		return nil
	}

	deployment.Spec.Template.ObjectMeta.Labels[key] = common.NowDefault()
	return r.Client.Update(context.TODO(), deployment)
}
