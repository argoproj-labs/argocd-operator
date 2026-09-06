// Copyright 2026 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gitopspromoter

import (
	"context"
	"fmt"
	"reflect"

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

var log = logr.Log.WithName("controller_promoter")

// generatePromoterResourceName generates a resource name for a resource that adheres to length constraints.
func generatePromoterResourceName(compName string, cr *argoproj.ArgoCD) string {
	return argoutil.NameWithSuffix(cr.ObjectMeta, compName)
}

// generatePromoterResourceNameWithNamespace generates a resource name for the resource with namespace included.
// Useful for cluster scoped resources.
func generatePromoterResourceNameWithNamespace(compName string, cr *argoproj.ArgoCD) string {
	return fmt.Sprintf("%s-%s-%s", cr.Name, cr.Namespace, compName)
}

// ReconcilePromoterServiceAccount reconciles a ServiceAccount needed for the Promoter's workloads. Handles creation, updating, and deletion.
func ReconcilePromoterServiceAccount(client client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme, enabled bool, imagePullSecrets []corev1.LocalObjectReference) (*corev1.ServiceAccount, error) {
	sa := buildPromoterServiceAccount(compName, cr)

	allowed := argoutil.IsNamespaceClusterConfigNamespace(cr.Namespace)

	exists := true
	if err := argoutil.FetchObject(client, cr.Namespace, sa.Name, sa); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get existing promoter service account %s in namespace %s: %v", sa.Name, sa.Namespace, err)
		}
		exists = false
	}

	if exists {
		if !cr.Spec.Promoter.IsEnabled() || !enabled || !allowed {
			argoutil.LogResourceDeletion(log, sa, fmt.Sprintf("promoter service account for component %s is being deleted due to being disabled", compName))
			if err := client.Delete(context.Background(), sa); err != nil {
				return nil, fmt.Errorf("failed to delete promoter service account %s: %v", sa.Name, err)
			}
			return sa, nil
		}
		// nil imagePullSecrets means the caller chose not to manage this field
		// (e.g. on OpenShift where the platform injects dockercfg secrets).
		existing := sa.ImagePullSecrets
		if existing == nil {
			existing = []corev1.LocalObjectReference{}
		}
		if imagePullSecrets != nil && !reflect.DeepEqual(existing, imagePullSecrets) {
			sa.ImagePullSecrets = imagePullSecrets
			argoutil.LogResourceUpdate(log, sa, "imagePullSecrets changed")
			if err := client.Update(context.Background(), sa); err != nil {
				return nil, fmt.Errorf("failed to update promoter service account %s: %v", sa.Name, err)
			}
		}
		return sa, nil
	}

	if !cr.Spec.Promoter.IsEnabled() || !enabled || !allowed {
		return sa, nil
	}

	if imagePullSecrets != nil {
		sa.ImagePullSecrets = imagePullSecrets
	}
	if err := controllerutil.SetControllerReference(cr, sa, scheme); err != nil {
		return nil, fmt.Errorf("failed to set argocd cr %s as owner for service account %s: %v", cr.Name, sa.Name, err)
	}

	argoutil.LogResourceCreation(log, sa)
	if err := client.Create(context.Background(), sa); err != nil {
		return nil, fmt.Errorf("failed to create promoter service account %s: %v", sa.Name, err)
	}
	return sa, nil
}

// buildPromoterServiceAccount creates a ServiceAccountObject with metadata
func buildPromoterServiceAccount(compName string, cr *argoproj.ArgoCD) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generatePromoterResourceName(compName, cr),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForPromoterResources(compName, cr),
		},
	}
}

// buildLabelsForPromoterResources builds the labels to be used in it's resources metadata
func buildLabelsForPromoterResources(compName string, cr *argoproj.ArgoCD) map[string]string {
	return map[string]string{
		common.ArgoCDKeyName:      argoutil.TruncateWithHash(generatePromoterResourceName(compName, cr), argoutil.GetMaxLabelLength()),
		common.ArgoCDKeyComponent: compName,
		common.ArgoCDKeyPartOf:    "promoter",
		common.ArgoCDKeyManagedBy: cr.Name,
	}
}
