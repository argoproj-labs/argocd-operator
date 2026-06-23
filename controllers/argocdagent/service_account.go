// Copyright 2025 ArgoCD Operator Developers
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

package argocdagent

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logr "sigs.k8s.io/controller-runtime/pkg/log"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

var log = logr.Log.WithName("controller_agent")

// ReconcilePrincipalServiceAccount reconciles the service account for the ArgoCD agent principal component.
// It handles creation, deletion, and updates of the service account based on the principal configuration.
func ReconcilePrincipalServiceAccount(client client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) (*corev1.ServiceAccount, error) {
	sa := buildServiceAccount(compName, cr)

	// Check if the service account already exists
	exists := true
	if err := argoutil.FetchObject(client, cr.Namespace, sa.Name, sa); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get existing principal service account %s in namespace %s: %v", sa.Name, cr.Namespace, err)
		}
		exists = false
	}

	if exists {
		// If service account exists but principal is disabled, delete it
		if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
			argoutil.LogResourceDeletion(log, sa, "principal service account is being deleted as principal is disabled")
			if err := client.Delete(context.TODO(), sa); err != nil {
				return nil, fmt.Errorf("failed to delete principal service account %s: %v", sa.Name, err)
			}
			return sa, nil
		}
		// Service account exists and principal is enabled, nothing to do
		return sa, nil
	}

	// If service account doesn't exist and principal is disabled, nothing to do
	if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
		return sa, nil
	}

	if err := controllerutil.SetControllerReference(cr, sa, scheme); err != nil {
		return nil, fmt.Errorf("failed to set ArgoCD CR %s as owner for service account %s: %w", cr.Name, sa.Name, err)
	}

	// Create the service account since it doesn't exist and principal is enabled
	argoutil.LogResourceCreation(log, sa)
	if err := client.Create(context.TODO(), sa); err != nil {
		return nil, fmt.Errorf("failed to create principal service account %s: %v", sa.Name, err)
	}
	return sa, nil
}

func buildServiceAccount(compName string, cr *argoproj.ArgoCD) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, compName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, compName),
		},
	}
}

// generateAgentResourceName creates a standardized resource name for ArgoCD agent components
// by combining the ArgoCD CR name with the component name.
func generateAgentResourceName(crName, compName string) string {
	return fmt.Sprintf("%s-agent-%s", crName, compName)
}

func buildLabelsForAgentPrincipal(crName, compName string) map[string]string {
	return map[string]string{
		common.ArgoCDKeyComponent: compName,
		common.ArgoCDKeyName:      generateAgentResourceName(crName, compName),
		common.ArgoCDKeyPartOf:    "argocd-agent",
		common.ArgoCDKeyManagedBy: crName,
	}
}
