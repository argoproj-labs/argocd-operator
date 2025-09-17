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

	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// reconcileStatus will ensure that all of the Status properties are updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatus(cr *argoproj.ArgoCD, argocdStatus *argoproj.ArgoCDStatus) error {

	// Note: NONE of the functions called here should modify the actual K8s object (e.g. the object in etcd on cluster).
	// - These functions should ONLY modify the values in 'argocdStatus' param.
	// - Updating the actual K8s object is handled elsewhere

	if argocdStatus.ApplicationController == "" { // Don't override app controller status if it was already set elsewhere

		if err := r.reconcileStatusApplicationController(cr, argocdStatus); err != nil {
			return err
		}
	}

	if argocdStatus.SSO == "" { // Don't override SSO status if it was already set elsewhere
		if err := r.reconcileStatusSSO(cr, argocdStatus); err != nil {
			return err
		}
	}

	if argocdStatus.Redis == "" {
		if err := r.reconcileStatusRedis(cr, argocdStatus); err != nil {
			return err
		}
	}

	if argocdStatus.Repo == "" {
		if err := r.reconcileStatusRepo(cr, argocdStatus); err != nil {
			return err
		}
	}

	if argocdStatus.Server == "" {
		if err := r.reconcileStatusServer(cr, argocdStatus); err != nil {
			return err
		}
	}

	if argocdStatus.Phase == "" { // We don't want to override a phase that was already set
		if err := r.reconcileStatusHost(cr, argocdStatus); err != nil {
			return err
		}
	}

	if argocdStatus.NotificationsController == "" {
		if err := r.reconcileStatusNotifications(cr, argocdStatus); err != nil {
			return err
		}
	}

	if argocdStatus.ApplicationSetController == "" {
		if err := r.reconcileStatusApplicationSetController(cr, argocdStatus); err != nil {
			return err
		}
	}

	if argocdStatus.Phase == "" {
		// We only want to call reconcileStatusPhase if .status.phase was NOT already set by any of the functions above.
		// - If .status.phase value WAS set by one of the functions above, then we don't want to override that value

		// Since this function reads from values that were set by the other functions above, this function must be called last.
		if err := r.reconcileStatusPhase(cr, argocdStatus); err != nil {
			return err
		}

	}

	return nil
}

// reconcileStatusApplicationController will ensure that the ApplicationController Status is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusApplicationController(cr *argoproj.ArgoCD, argocdStatus *argoproj.ArgoCDStatus) error {
	status := "Unknown"

	ss := newStatefulSetWithSuffix("application-controller", "application-controller", cr)
	ssExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, ss.Name, ss)
	if err != nil {
		argocdStatus.ApplicationController = "Failed"
		return err
	}
	if ssExists {
		status = "Pending"
		if ss.Spec.Replicas != nil {
			if ss.Status.ReadyReplicas == *ss.Spec.Replicas {
				status = "Running"
			}
		}
	}

	argocdStatus.ApplicationController = status

	return nil
}

// reconcileStatusApplicationSetController will ensure that the ApplicationSet controller status is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusApplicationSetController(cr *argoproj.ArgoCD, argocdStatus *argoproj.ArgoCDStatus) error {
	status := "Unknown"

	deploy := newDeploymentWithSuffix("applicationset-controller", "controller", cr)
	deplExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy)
	if err != nil {
		argocdStatus.ApplicationSetController = "Failed"
		return err
	}
	if deplExists {
		status = "Pending"

		if deploy.Spec.Replicas != nil {
			if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
				status = "Running"
			} else if deploy.Status.Conditions != nil {
				for _, condition := range deploy.Status.Conditions {
					if condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue {
						// Deployment has failed
						status = "Failed"
						break
					}
				}
			}
		}
	}

	argocdStatus.ApplicationSetController = status
	return nil
}

// reconcileStatusSSOConfig will ensure that the SSOConfig status is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusSSO(cr *argoproj.ArgoCD, argocdStatus *argoproj.ArgoCDStatus) error {

	if cr.Spec.SSO != nil && cr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeDex {
		// A) If Dex is enabled

		deploy := newDeploymentWithSuffix("dex-server", "dex-server", cr)
		deplExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy)
		if err != nil {
			argocdStatus.SSO = "Failed"
			return err
		}
		status := "Unknown"

		if deplExists {
			status = "Pending"

			if deploy.Spec.Replicas != nil {
				if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
					status = "Running"
				} else if deploy.Status.Conditions != nil {
					for _, condition := range deploy.Status.Conditions {
						if condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue {
							// Deployment has failed
							status = "Failed"
							break
						}
					}
				}
			}
		}
		argocdStatus.SSO = status

		return nil

	} else if cr.Spec.SSO != nil && cr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeKeycloak {

		// B) If Keycloak is enabled
		log.Info("Keycloak SSO provider is no longer supported. RBAC scopes configuration is ignored.")
		// Keycloak functionality has been removed, skipping reconciliation
		argocdStatus.SSO = "Failed"

	} else {
		// C) All other cases
		argocdStatus.SSO = "Unknown"
	}

	return nil
}

// reconcileStatusPhase will ensure that the Status Phase is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusPhase(cr *argoproj.ArgoCD, argocdStatus *argoproj.ArgoCDStatus) error {
	var phase string

	appControllerAvailable := (!cr.Spec.Controller.IsEnabled() && argocdStatus.ApplicationController == "Unknown") || argocdStatus.ApplicationController == "Running"

	redisAvailable := (!cr.Spec.Redis.IsEnabled() && argocdStatus.Redis == "Unknown") || argocdStatus.Redis == "Running" || (cr.Spec.Redis.IsEnabled() && cr.Spec.Redis.Remote != nil && *cr.Spec.Redis.Remote != "")

	repoServerAvailable := (!cr.Spec.Repo.IsEnabled() && argocdStatus.Repo == "Unknown") || argocdStatus.Repo == "Running"

	serverAvailable := (!cr.Spec.Server.IsEnabled() && argocdStatus.Server == "Unknown") || argocdStatus.Server == "Running"

	ssoAvailable := (!cr.Spec.SSO.IsEnabled() && argocdStatus.SSO == "Unknown") || argocdStatus.SSO == "Running"

	if appControllerAvailable &&
		redisAvailable &&
		repoServerAvailable &&
		serverAvailable &&
		ssoAvailable {
		phase = "Available"
	} else {
		phase = "Pending"
	}

	argocdStatus.Phase = phase

	return nil
}

// reconcileStatusRedis will ensure that the Redis status is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusRedis(cr *argoproj.ArgoCD, argocdStatus *argoproj.ArgoCDStatus) error {
	status := "Unknown"

	if !cr.Spec.HA.Enabled {
		deploy := newDeploymentWithSuffix("redis", "redis", cr)
		deplExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy)
		if err != nil {
			argocdStatus.Redis = "Failed"
			return err
		}
		if deplExists {
			status = "Pending"

			if deploy.Spec.Replicas != nil {
				if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
					status = "Running"
				} else if deploy.Status.Conditions != nil {
					for _, condition := range deploy.Status.Conditions {
						if condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue {
							// Deployment has failed
							status = "Failed"
							break
						}
					}
				}
			}
		}
	} else {
		ss := newStatefulSetWithSuffix("redis-ha-server", "redis-ha-server", cr)
		ssExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, ss.Name, ss)
		if err != nil {
			return err
		}
		if ssExists {
			status = "Pending"

			if ss.Status.ReadyReplicas == *ss.Spec.Replicas {
				status = "Running"
			}
		}
		// TODO: Add check for HA proxy deployment here as well?
	}

	argocdStatus.Redis = status

	return nil
}

// reconcileStatusServer will ensure that the Server status is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusServer(cr *argoproj.ArgoCD, argocdStatus *argoproj.ArgoCDStatus) error {
	status := "Unknown"

	deploy := newDeploymentWithSuffix("server", "server", cr)
	deplExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy)
	if err != nil {
		argocdStatus.Server = "Failed"
		return err
	}
	if deplExists {
		status = "Pending"

		if deploy.Spec.Replicas != nil {
			if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
				status = "Running"
			} else if deploy.Status.Conditions != nil {
				for _, condition := range deploy.Status.Conditions {
					if condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue {
						// Deployment has failed
						status = "Failed"
						break
					}
				}
			}
		}
	}

	argocdStatus.Server = status
	return nil
}

// reconcileStatusNotifications will ensure that the Notifications status is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusNotifications(cr *argoproj.ArgoCD, argocdStatus *argoproj.ArgoCDStatus) error {
	status := "Unknown"

	deploy := newDeploymentWithSuffix("notifications-controller", "controller", cr)
	deplExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy)
	if err != nil {
		argocdStatus.NotificationsController = "Failed"
		return err
	}
	if deplExists {
		status = "Pending"

		if deploy.Spec.Replicas != nil {
			if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
				status = "Running"
			} else if deploy.Status.Conditions != nil {
				for _, condition := range deploy.Status.Conditions {
					if condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue {
						// Deployment has failed
						status = "Failed"
						break
					}
				}
			}
		}
	}

	if !cr.Spec.Notifications.Enabled {
		argocdStatus.NotificationsController = ""
	} else {
		argocdStatus.NotificationsController = status
	}

	return nil
}

// reconcileStatusHost will ensure that the host status is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusHost(cr *argoproj.ArgoCD, argocdStatus *argoproj.ArgoCDStatus) error {
	argocdStatus.Host = ""

	if (cr.Spec.Server.Route.Enabled || cr.Spec.Server.Ingress.Enabled) && IsRouteAPIAvailable() {
		route := newRouteWithSuffix("server", cr)

		// The Red Hat OpenShift ingress controller implementation is designed to watch ingress objects and create one or more routes
		// to fulfill the conditions specified.
		// But the names of such created route resources are randomly generated so it is better to identify the routes using Labels
		// instead of Name.
		// 1. If a user creates ingress on openshift, Ingress controller generates a route for the ingress with random name.
		// 2. If a user creates route on openshift, Ingress controller processes the route with provided name.
		routeList := &routev1.RouteList{}
		opts := &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				"app.kubernetes.io/name": route.Name,
			}),
			Namespace: cr.Namespace,
		}

		if err := r.List(context.TODO(), routeList, opts); err != nil {
			argocdStatus.Phase = "Failed"
			return err
		}

		if len(routeList.Items) == 0 {
			log.Info("argocd-server route requested but not found on cluster")
			return nil
		} else {
			route = &routeList.Items[0]
			// status.ingress not available
			if route.Status.Ingress == nil {
				argocdStatus.Host = ""
				argocdStatus.Phase = "Pending"
			} else {
				// conditions exist and type is RouteAdmitted
				if len(route.Status.Ingress[0].Conditions) > 0 && route.Status.Ingress[0].Conditions[0].Type == routev1.RouteAdmitted {
					if route.Status.Ingress[0].Conditions[0].Status == corev1.ConditionTrue {
						argocdStatus.Host = route.Status.Ingress[0].Host
					} else {
						argocdStatus.Host = ""
						argocdStatus.Phase = "Pending"
					}
				} else {
					// no conditions are available
					if route.Status.Ingress[0].Host != "" {
						argocdStatus.Host = route.Status.Ingress[0].Host
					} else {
						argocdStatus.Host = "Unavailable"
						argocdStatus.Phase = "Pending"
					}
				}
			}
		}
	} else if cr.Spec.Server.Ingress.Enabled {
		ingress := newIngressWithSuffix("server", cr)
		ingressExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress)
		if err != nil {
			argocdStatus.Phase = "Failed"
			return err
		}
		if !ingressExists {
			log.Info("argocd-server ingress requested but not found on cluster")
			argocdStatus.Phase = "Pending"
			return nil
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
				argocdStatus.Host = hosts
			}
		}
	}

	return nil
}
