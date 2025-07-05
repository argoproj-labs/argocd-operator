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

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logr "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logr.Log.WithName("controller_agent")

func ReconcilePrincipalServiceAccount(client client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) (*corev1.ServiceAccount, error) {
	sa := buildServiceAccount(compName, cr)

	exists := true
	if err := argoutil.FetchObject(client, cr.Namespace, sa.Name, sa); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get existing principal service account %s in namespace %s: %v", sa.Name, cr.Namespace, err)
		}
		exists = false
	}

	if exists {
		if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
			argoutil.LogResourceDeletion(log, sa, "principal service account is being deleted as principal is disabled")
			if err := client.Delete(context.TODO(), sa); err != nil {
				return nil, fmt.Errorf("failed to delete principal service account %s: %v", sa.Name, err)
			}
			return sa, nil
		}
		return sa, nil
	}

	// If service account doesn't exist and principal is disabled, nothing to do
	if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
		return sa, nil
	}

	if err := controllerutil.SetControllerReference(cr, sa, scheme); err != nil {
		return nil, fmt.Errorf("failed to set ArgoCD CR %s as owner for service account %s: %w", cr.Name, sa.Name, err)
	}

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
			Labels:    buildLabelsForAgentPrincipal(cr.Name),
		},
	}
}

func generateAgentResourceName(crName, compName string) string {
	return fmt.Sprintf("%s-agent-%s", crName, compName)
}

func buildLabelsForAgentPrincipal(crName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/component":  "principal",
		"app.kubernetes.io/name":       "argocd-agent-principal",
		"app.kubernetes.io/part-of":    "argocd-agent",
		"app.kubernetes.io/managed-by": crName,
	}
}
