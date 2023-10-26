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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

var _ reconcile.Reconciler = &ReconcileArgoCD{}

// When the ArgoCD object has been marked as deleting, we should not reconcile,
// and trigger the creation of new objects.
//
// We have owner references set on created resources, this triggers automatic
// deletion of the associated objects.
func TestReconcileArgoCD_Reconcile_with_deleted(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(deletedAt(time.Now()))

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
	}
	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	if res.Requeue {
		t.Fatal("reconcile requeued request")
	}

	deployment := &appsv1.Deployment{}
	if !apierrors.IsNotFound(r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-redis",
		Namespace: testNamespace,
	}, deployment)) {
		t.Fatalf("expected not found error, got %#v\n", err)
	}
}

func TestReconcileArgoCD_Reconcile(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
	}

	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	if res.Requeue {
		t.Fatal("reconcile requeued request")
	}

	deployment := &appsv1.Deployment{}
	if err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-redis",
		Namespace: testNamespace,
	}, deployment); err != nil {
		t.Fatalf("failed to find the redis deployment: %#v\n", err)
	}
}

func TestReconcileArgoCD_Reconcile_RemoveManagedByLabelOnArgocdDeletion(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		testName                                  string
		nsName                                    string
		isRemoveManagedByLabelOnArgoCDDeletionSet bool
	}{
		{
			testName: "Without REMOVE_MANAGED_BY_LABEL_ON_ARGOCD_DELETION set",
			nsName:   "newNamespaceTest1",
			isRemoveManagedByLabelOnArgoCDDeletionSet: false,
		},
		{
			testName: "With REMOVE_MANAGED_BY_LABEL_ON_ARGOCD_DELETION set",
			nsName:   "newNamespaceTest2",
			isRemoveManagedByLabelOnArgoCDDeletionSet: true,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			a := makeTestArgoCD(deletedAt(time.Now()), addFinalizer(common.ArgoCDDeletionFinalizer))

			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch)

			nsArgocd := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
				Name: a.Namespace,
			}}
			err := r.Client.Create(context.TODO(), nsArgocd)
			assert.NoError(t, err)

			if test.isRemoveManagedByLabelOnArgoCDDeletionSet {
				t.Setenv("REMOVE_MANAGED_BY_LABEL_ON_ARGOCD_DELETION", "true")
			}

			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
				Name: test.nsName,
				Labels: map[string]string{
					common.ArgoCDManagedByLabel: a.Namespace,
				}},
			}
			err = r.Client.Create(context.TODO(), ns)
			assert.NoError(t, err)

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      a.Name,
					Namespace: a.Namespace,
				},
			}

			_, err = r.Reconcile(context.TODO(), req)
			assert.NoError(t, err)

			assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: ns.Name}, ns))
			if test.isRemoveManagedByLabelOnArgoCDDeletionSet {
				// Check if the managed-by label gets removed from the new namespace
				if _, ok := ns.Labels[common.ArgoCDManagedByLabel]; ok {
					t.Errorf("Expected the label[%v] to be removed from the namespace[%v]", common.ArgoCDManagedByLabel, ns.Name)
				}
			} else {
				// Check if the managed-by label still exists in the new namespace
				assert.Equal(t, ns.Labels[common.ArgoCDManagedByLabel], a.Namespace)
			}
		})
	}
}

func deletedAt(now time.Time) argoCDOpt {
	return func(a *argoproj.ArgoCD) {
		wrapped := metav1.NewTime(now)
		a.ObjectMeta.DeletionTimestamp = &wrapped
		a.Finalizers = []string{"test: finalizaer"}
	}
}

func TestReconcileArgoCD_CleanUp(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(deletedAt(time.Now()), addFinalizer(common.ArgoCDDeletionFinalizer))

	resources := []client.Object{a}
	resources = append(resources, clusterResources(a)...)

	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resources, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
	}
	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	if res.Requeue {
		t.Fatal("reconcile requeued request")
	}

	// check if cluster resources are deleted
	tt := []struct {
		name     string
		resource client.Object
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
			newClusterRoleBinding(a),
		},
		{
			fmt.Sprintf("ClusterRoleBinding %s", common.ArgoCDServerComponent),
			newClusterRoleBinding(a),
		},
	}

	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			if argoutil.IsObjectFound(r.Client, "", test.name, test.resource) {
				t.Errorf("Expected %s to be deleted", test.name)
			}
		})
	}

	// check if namespace label was removed
	ns := &corev1.Namespace{}
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: a.Namespace}, ns))
	if _, ok := ns.Labels[common.ArgoCDManagedByLabel]; ok {
		t.Errorf("Expected the label[%v] to be removed from the namespace[%v]", common.ArgoCDManagedByLabel, a.Namespace)
	}
}

func addFinalizer(finalizer string) argoCDOpt {
	return func(a *argoproj.ArgoCD) {
		a.Finalizers = append(a.Finalizers, finalizer)
	}
}

func clusterResources(argocd *argoproj.ArgoCD) []client.Object {
	return []client.Object{
		newClusterRole(common.ArgoCDApplicationControllerComponent, []v1.PolicyRule{}, argocd),
		newClusterRole(common.ArgoCDServerComponent, []v1.PolicyRule{}, argocd),
		newClusterRoleBindingWithname(common.ArgoCDApplicationControllerComponent, argocd),
		newClusterRoleBindingWithname(common.ArgoCDServerComponent, argocd),
	}
}
