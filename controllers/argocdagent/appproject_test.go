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
	"testing"

	appv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func makeTestReconcilerSchemeWithAppProject() *runtime.Scheme {
	s := makeTestReconcilerScheme()
	_ = appv1alpha1.AddToScheme(s)
	return s
}

func TestReconcileAppProject_AppProjectDoesNotExist_ShouldCreateAppProject(t *testing.T) {
	// Test case: AppProject doesn't exist
	// Expected behavior: Should create the AppProject with expected spec

	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerSchemeWithAppProject()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAppProject(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify AppProject was created
	appProject := &appv1alpha1.AppProject{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      "default",
		Namespace: cr.Namespace,
	}, appProject)
	assert.NoError(t, err)

	// Verify AppProject has expected metadata
	assert.Equal(t, "default", appProject.Name)
	assert.Equal(t, cr.Namespace, appProject.Namespace)

	// Verify AppProject has expected spec
	assert.Equal(t, []string{"*"}, appProject.Spec.SourceRepos)
	assert.Equal(t, []string{"*"}, appProject.Spec.SourceNamespaces)
	assert.Len(t, appProject.Spec.ClusterResourceWhitelist, 1)
	assert.Equal(t, "*", appProject.Spec.ClusterResourceWhitelist[0].Group)
	assert.Equal(t, "*", appProject.Spec.ClusterResourceWhitelist[0].Kind)
	assert.Len(t, appProject.Spec.Destinations, 1)
	assert.Equal(t, "*", appProject.Spec.Destinations[0].Server)
	assert.Equal(t, "*", appProject.Spec.Destinations[0].Namespace)
}

func TestReconcileAppProject_AppProjectExists_WithSourceNamespaces_ShouldDoNothing(t *testing.T) {
	// Test case: AppProject exists and already has SourceNamespaces set
	// Expected behavior: Should do nothing (no update)

	cr := makeTestArgoCD()

	// Create existing AppProject with SourceNamespaces already set
	existingAppProject := &appv1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
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

	resObjs := []client.Object{cr, existingAppProject}
	sch := makeTestReconcilerSchemeWithAppProject()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAppProject(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify AppProject still exists with same SourceNamespaces
	appProject := &appv1alpha1.AppProject{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      "default",
		Namespace: cr.Namespace,
	}, appProject)
	assert.NoError(t, err)
	assert.Equal(t, []string{"*"}, appProject.Spec.SourceNamespaces)
}

func TestReconcileAppProject_AppProjectExists_WithoutSourceNamespaces_ShouldUpdateAppProject(t *testing.T) {
	// Test case: AppProject exists but doesn't have SourceNamespaces set
	// Expected behavior: Should update the AppProject to set SourceNamespaces to ["*"]

	cr := makeTestArgoCD()

	// Create existing AppProject without SourceNamespaces
	existingAppProject := &appv1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
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
			// SourceNamespaces is nil/empty
		},
	}

	resObjs := []client.Object{cr, existingAppProject}
	sch := makeTestReconcilerSchemeWithAppProject()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAppProject(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify AppProject was updated with SourceNamespaces
	appProject := &appv1alpha1.AppProject{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      "default",
		Namespace: cr.Namespace,
	}, appProject)
	assert.NoError(t, err)
	assert.Equal(t, []string{"*"}, appProject.Spec.SourceNamespaces)
}

func TestReconcileAppProject_AppProjectExists_WithEmptySourceNamespaces_ShouldUpdateAppProject(t *testing.T) {
	// Test case: AppProject exists but has empty SourceNamespaces slice
	// Expected behavior: Should update the AppProject to set SourceNamespaces to ["*"]

	cr := makeTestArgoCD()

	// Create existing AppProject with empty SourceNamespaces slice
	existingAppProject := &appv1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
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
			SourceNamespaces: []string{}, // Empty slice
		},
	}

	resObjs := []client.Object{cr, existingAppProject}
	sch := makeTestReconcilerSchemeWithAppProject()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAppProject(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify AppProject was updated with SourceNamespaces
	appProject := &appv1alpha1.AppProject{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      "default",
		Namespace: cr.Namespace,
	}, appProject)
	assert.NoError(t, err)
	assert.Equal(t, []string{"*"}, appProject.Spec.SourceNamespaces)
}
