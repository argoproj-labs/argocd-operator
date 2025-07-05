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

	appv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

func ReconcileAppProject(client client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) error {
	appProject := buildAppProject("default", cr)
	exists := true
	if err := argoutil.FetchObject(client, cr.Namespace, appProject.Name, appProject); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get existing appproject %s in namespace %s: %v", appProject.Name, cr.Namespace, err)
		}
		exists = false
	}

	if exists {
		if appProject.Spec.SourceNamespaces == nil {
			appProject.Spec.SourceNamespaces = []string{"*"}

			if err := client.Update(context.TODO(), appProject); err != nil {
				return fmt.Errorf("failed to update appproject %s in namespace %s: %v", appProject.Name, cr.Namespace, err)
			}
		}
		return nil
	}

	if !exists {
		if err := client.Create(context.TODO(), appProject); err != nil {
			return fmt.Errorf("failed to create appproject %s in namespace %s: %v", appProject.Name, cr.Namespace, err)
		}
		return nil
	}

	return nil
}

// buildService creates a new service for the principal
func buildAppProject(name string, cr *argoproj.ArgoCD) *appv1alpha1.AppProject {
	return &appv1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
		},
		Spec: appv1alpha1.AppProjectSpec{
			ClusterResourceWhitelist: []metav1.GroupKind{
				{
					Group: "*",
					Kind:  "*",
				},
			},
			SourceRepos: []string{"*"},
			Destinations: []appv1alpha1.ApplicationDestination{
				{
					Server:    "*",
					Namespace: "*",
				},
			},
			SourceNamespaces: []string{"*"},
		},
	}
}
