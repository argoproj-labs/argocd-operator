// Copyright 2020 ArgoCD Operator Developers
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
	"reflect"
	"strings"

	oappsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/networking"
	"github.com/argoproj-labs/argocd-operator/pkg/openshift"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

// reconcileStatus will ensure that all of the Status properties are updated for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileStatus() error {
	var statusErr util.MultiError

	if err := r.AppController.ReconcileStatus(); err != nil {
		r.Logger.Error(err, "reconcileStatus")
		statusErr.Append(err)
	}

	if err := r.ServerController.ReconcileStatus(); err != nil {
		r.Logger.Error(err, "reconcileStatus")
		statusErr.Append(err)
	}

	if err := r.RedisController.ReconcileStatus(); err != nil {
		r.Logger.Error(err, "reconcileStatus")
		statusErr.Append(err)
	}

	if err := r.ReposerverController.ReconcileStatus(); err != nil {
		r.Logger.Error(err, "reconcileStatus")
		statusErr.Append(err)
	}

	if err := r.AppsetController.ReconcileStatus(); err != nil {
		r.Logger.Error(err, "reconcileStatus")
		statusErr.Append(err)
	}

	if err := r.NotificationsController.ReconcileStatus(); err != nil {
		r.Logger.Error(err, "reconcileStatus")
		statusErr.Append(err)
	}

	if err := r.SSOController.ReconcileStatus(); err != nil {
		r.Logger.Error(err, "reconcileStatus")
		statusErr.Append(err)
	}

	if err := r.reconcilePhase(); err != nil {
		return err
	}

	if err := r.reconcileHost(); err != nil {
		return err
	}

	return statusErr.ErrOrNil()
}

// reconcileStatusPhase will ensure that the Status Phase is updated for the given ArgoCD.
func (r *ArgoCDReconciler) reconcilePhase() error {
	phase := common.ArgoCDStatusPending

	if ((!r.Instance.Spec.Controller.IsEnabled() && r.Instance.Status.ApplicationController == common.ArgoCDStatusUnknown) || r.Instance.Status.ApplicationController == common.ArgoCDStatusRunning) &&
		((!r.Instance.Spec.Redis.IsEnabled() && r.Instance.Status.Redis == common.ArgoCDStatusUnknown) || r.Instance.Status.Redis == common.ArgoCDStatusRunning) &&
		((!r.Instance.Spec.Repo.IsEnabled() && r.Instance.Status.Repo == common.ArgoCDStatusUnknown) || r.Instance.Status.Repo == common.ArgoCDStatusRunning) &&
		((!r.Instance.Spec.Server.IsEnabled() && r.Instance.Status.Server == common.ArgoCDStatusUnknown) || r.Instance.Status.Server == common.ArgoCDStatusRunning) {
		phase = common.ArgoCDStatusAvailable
	}

	if r.Instance.Status.Phase != phase {
		r.Instance.Status.Phase = phase
	}
	return r.updateInstanceStatus()
}

func (r *ArgoCDReconciler) reconcileHost() error {
	host := ""
	phase := r.Instance.Status.Phase

	// return if neither ingress, nor route is enabled
	if !r.Instance.Spec.Server.Ingress.Enabled && (!r.Instance.Spec.Server.Route.Enabled || !openshift.IsOpenShiftEnv()) {
		r.Instance.Status.Host = host
		r.Instance.Status.Phase = phase
		return r.updateInstanceStatus()
	}

	serverResourceName := argoutil.GenerateResourceName(r.Instance.Name, common.ServerSuffix)

	if r.Instance.Spec.Server.Route.Enabled && openshift.IsOpenShiftEnv() {
		// The Red Hat OpenShift ingress controller implementation is designed to watch ingress objects and create one or more routes
		// to fulfill the conditions specified.
		// But the names of such created route resources are randomly generated so it is better to identify the routes using Labels
		// instead of Name.
		// 1. If a user creates ingress on openshift, Ingress controller generates a route for the ingress with random name.
		// 2. If a user creates route on openshift, Ingress controller processes the route with provided name.
		routeList, err := openshift.ListRoutes(r.Instance.Namespace, r.Client, []client.ListOption{
			&client.ListOptions{
				LabelSelector: labels.SelectorFromSet(map[string]string{
					common.AppK8sKeyName: serverResourceName,
				}),
				Namespace: r.Instance.Namespace,
			},
		})
		if err != nil {
			return errors.Wrap(err, "reconcileHost: faield to list routes")
		}

		if len(routeList.Items) == 0 {
			r.Logger.Debug("reconcileHost: server route requested but not found on cluster")
			phase = common.ArgoCDStatusPending
		} else {
			route := &routeList.Items[0]
			// status.ingress not available
			if route.Status.Ingress == nil {
				host = ""
				phase = common.ArgoCDStatusPending
			} else {
				// conditions exist and type is RouteAdmitted
				routeConditions := route.Status.Ingress[0].Conditions
				if len(routeConditions) > 0 && routeConditions[0].Type == routev1.RouteAdmitted {
					if route.Status.Ingress[0].Conditions[0].Status == corev1.ConditionTrue {
						host = route.Status.Ingress[0].Host
					} else {
						host = ""
						phase = common.ArgoCDStatusPending
					}
				} else {
					// no conditions are available
					if route.Status.Ingress[0].Host != "" {
						host = route.Status.Ingress[0].Host
					} else {
						host = common.ArgoCDStatusUnavailable
						phase = common.ArgoCDStatusPending
					}
				}
			}
		}
	} else if r.Instance.Spec.Server.Ingress.Enabled {
		ingress, err := networking.GetIngress(serverResourceName, r.Instance.Namespace, r.Client)
		if err != nil {
			r.Logger.Debug("reconcileHost: server ingress requested but not found on cluster")
			phase = common.ArgoCDStatusPending
		} else {
			if !reflect.DeepEqual(ingress.Status.LoadBalancer, corev1.LoadBalancerStatus{}) && len(ingress.Status.LoadBalancer.Ingress) > 0 {
				var s []string
				var hosts string
				for _, ingressElement := range ingress.Status.LoadBalancer.Ingress {
					if ingressElement.Hostname != "" {
						s = append(s, ingressElement.Hostname)
						continue
					} else if ingressElement.IP != "" {
						s = append(s, ingressElement.IP)
						continue
					}
				}
				hosts = strings.Join(s, ", ")
				host = hosts
			}
		}
	}

	if r.Instance.Status.Host != host {
		r.Instance.Status.Host = host
	}

	if r.Instance.Status.Phase != phase {
		r.Instance.Status.Phase = phase
	}

	return r.updateInstanceStatus()
}

func (r *ArgoCDReconciler) updateInstanceStatus() error {
	return resource.UpdateStatusSubResource(r.Instance, r.Client)
}

// reconcileStatusKeycloak will ensure that the Keycloak status is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusKeycloak(cr *argoproj.ArgoCD) error {
	status := "Unknown"

	if IsTemplateAPIAvailable() {
		// keycloak is installed using OpenShift templates.
		dc := &oappsv1.DeploymentConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      defaultKeycloakIdentifier,
				Namespace: cr.Namespace,
			},
		}
		if argoutil.IsObjectFound(r.Client, cr.Namespace, dc.Name, dc) {
			status = "Pending"

			if dc.Status.ReadyReplicas == dc.Spec.Replicas {
				status = "Running"
			} else if dc.Status.Conditions != nil {
				for _, condition := range dc.Status.Conditions {
					if condition.Type == oappsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue {
						// Deployment has failed
						status = "Failed"
						break
					}
				}
			}
		}

	} else {
		d := newDeploymentWithName(defaultKeycloakIdentifier, defaultKeycloakIdentifier, cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, d.Name, d) {
			status = "Pending"

			if d.Spec.Replicas != nil {
				if d.Status.ReadyReplicas == *d.Spec.Replicas {
					status = "Running"
				} else if d.Status.Conditions != nil {
					for _, condition := range d.Status.Conditions {
						if condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue {
							// Deployment has failed
							status = "Failed"
							break
						}
					}
				}

			}
		}
	}

	if cr.Status.SSO != status {
		cr.Status.SSO = status
		return r.Client.Status().Update(context.TODO(), cr)
	}

	return nil
}
