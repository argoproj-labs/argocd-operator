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
	"fmt"
	"testing"
	"time"

	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	argov1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
)

var _ reconcile.Reconciler = &ReconcileArgoCD{}

// When the ArgoCD object has been marked as deleting, we should not reconcile,
// and trigger the creation of new objects.
//
// We have owner references set on created resources, this triggers automatic
// deletion of the associated objects.
func TestReconcileArgoCD_Reconcile_with_deleted(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD(deletedAt(time.Now()))

	r := makeTestReconciler(t, a)
	createNamespace(r, a.Namespace, false)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
	}
	res, err := r.Reconcile(req)
	assert.NilError(t, err)
	if res.Requeue {
		t.Fatal("reconcile requeued request")
	}

	deployment := &appsv1.Deployment{}
	if !apierrors.IsNotFound(r.client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-redis",
		Namespace: testNamespace,
	}, deployment)) {
		t.Fatalf("expected not found error, got %#v\n", err)
	}
}

func TestReconcileArgoCD_Reconcile(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()

	r := makeTestReconciler(t, a)
	createNamespace(r, a.Namespace, false)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
	}

	res, err := r.Reconcile(req)
	assert.NilError(t, err)
	if res.Requeue {
		t.Fatal("reconcile requeued request")
	}

	deployment := &appsv1.Deployment{}
	if err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-redis",
		Namespace: testNamespace,
	}, deployment); err != nil {
		t.Fatalf("failed to find the redis deployment: %#v\n", err)
	}
}

func deletedAt(now time.Time) argoCDOpt {
	return func(a *argov1alpha1.ArgoCD) {
		wrapped := metav1.NewTime(now)
		a.ObjectMeta.DeletionTimestamp = &wrapped
	}
}

func TestReconcileArgoCD_CleanUp(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD(deletedAt(time.Now()), addFinalizer(common.ArgoCDDeletionFinalizer))

	resources := []runtime.Object{a}
	resources = append(resources, clusterResources(a)...)
	r := makeTestReconciler(t, resources...)
	createNamespace(r, a.Namespace, false)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
	}
	res, err := r.Reconcile(req)
	assert.NilError(t, err)
	if res.Requeue {
		t.Fatal("reconcile requeued request")
	}

	// check if cluster resources are deleted
	tt := []struct {
		name     string
		resource runtime.Object
	}{
		{
			fmt.Sprintf("ClusterRole %s", common.ArgoCDApplicationControllerComponent),
			newClusterRole(common.ArgoCDApplicationControllerComponent, []v1.PolicyRule{}, a),
		},
		{
			fmt.Sprintf("ClusterRole %s", common.ArgoCDServerComponent),
			newClusterRole(common.ArgoCDServerComponent, []v1.PolicyRule{}, a),
		},
		{
			fmt.Sprintf("ClusterRoleBinding %s", common.ArgoCDApplicationControllerComponent),
			newClusterRoleBinding(common.ArgoCDApplicationControllerComponent, a),
		},
		{
			fmt.Sprintf("ClusterRoleBinding %s", common.ArgoCDServerComponent),
			newClusterRoleBinding(common.ArgoCDServerComponent, a),
		},
	}

	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			if argoutil.IsObjectFound(r.client, "", test.name, test.resource) {
				t.Errorf("Expected %s to be deleted", test.name)
			}
		})
	}
}

func addFinalizer(finalizer string) argoCDOpt {
	return func(a *argov1alpha1.ArgoCD) {
		a.Finalizers = append(a.Finalizers, finalizer)
	}
}

func clusterResources(argocd *argov1alpha1.ArgoCD) []runtime.Object {
	return []runtime.Object{
		newClusterRole(common.ArgoCDApplicationControllerComponent, []v1.PolicyRule{}, argocd),
		newClusterRole(common.ArgoCDServerComponent, []v1.PolicyRule{}, argocd),
		newClusterRoleBindingWithname(common.ArgoCDApplicationControllerComponent, argocd),
		newClusterRoleBindingWithname(common.ArgoCDServerComponent, argocd),
	}
}
