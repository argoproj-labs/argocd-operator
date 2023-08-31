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
	"sort"
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

	argov1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
)

var _ reconcile.Reconciler = &ArgoCDReconciler{}

func deletedAt(now time.Time) argoCDOpt {
	return func(a *argov1alpha1.ArgoCD) {
		wrapped := metav1.NewTime(now)
		a.ObjectMeta.DeletionTimestamp = &wrapped
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

// When the ArgoCD object has been marked as deleting, we should not reconcile,
// and trigger the creation of new objects.
//
// We have owner references set on created resources, this triggers automatic
// deletion of the associated objects.
func TestArgoCDReconciler_Reconcile_with_deleted(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(deletedAt(time.Now()))

	r := makeTestReconciler(t, a)
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

func TestArgoCDReconciler_Reconcile(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	r := makeTestReconciler(t, a)
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

func TestArgoCDReconciler_Reconcile_RemoveManagedByLabelOnArgocdDeletion(t *testing.T) {
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
			a := makeTestArgoCD(deletedAt(time.Now()), addFinalizer(common.ArgoprojKeyFinalizer))
			r := makeTestReconciler(t, a)

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
					common.ArgoCDArgoprojKeyManagedBy: a.Namespace,
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
				if _, ok := ns.Labels[common.ArgoCDArgoprojKeyManagedBy]; ok {
					t.Errorf("Expected the label[%v] to be removed from the namespace[%v]", common.ArgoCDArgoprojKeyManagedBy, ns.Name)
				}
			} else {
				// Check if the managed-by label still exists in the new namespace
				assert.Equal(t, ns.Labels[common.ArgoCDArgoprojKeyManagedBy], a.Namespace)
			}
		})
	}
}

func TestArgoCDReconciler_CleanUp(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(deletedAt(time.Now()), addFinalizer(common.ArgoprojKeyFinalizer))

	resources := []runtime.Object{a}
	resources = append(resources, clusterResources(a)...)
	r := makeTestReconciler(t, resources...)
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
	if _, ok := ns.Labels[common.ArgoCDArgoprojKeyManagedBy]; ok {
		t.Errorf("Expected the label[%v] to be removed from the namespace[%v]", common.ArgoCDArgoprojKeyManagedBy, a.Namespace)
	}
}

func TestSetResourceManagedNamespaces(t *testing.T) {
	r := makeTestReconciler(t,
		makeTestNs(func(n *corev1.Namespace) {
			n.Name = "instance-1"
		}),
		makeTestNs(func(n *corev1.Namespace) {
			n.Name = "instance-2"
		}),
		makeTestNs(func(n *corev1.Namespace) {
			n.Name = "test-ns-1"
			n.Labels[common.ArgoCDArgoprojKeyManagedBy] = "instance-1"
		}),
		makeTestNs(func(n *corev1.Namespace) {
			n.Name = "test-ns-2"
			n.Labels[common.ArgoCDArgoprojKeyManagedBy] = "instance-2"
		}),
		makeTestNs(func(n *corev1.Namespace) {
			n.Name = "test-ns-3"
			n.Labels[common.ArgoCDArgoprojKeyManagedBy] = "instance-2"
		}),
		makeTestNs(func(n *corev1.Namespace) {
			n.Name = "test-ns-4"
			n.Labels[common.ArgoCDArgoprojKeyManagedBy] = "instance-1"
		}),
		makeTestNs(func(n *corev1.Namespace) {
			n.Name = "test-ns-5"
			n.Labels["something"] = "random"
		}),
		makeTestNs(func(n *corev1.Namespace) {
			n.Name = "test-ns-6"
			n.Labels[common.ArgoCDArgoprojKeyManagedBy] = "instance-3"
		}),
	)

	instanceOne := makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
		ac.Namespace = "instance-1"
	})
	instanceTwo := makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
		ac.Namespace = "instance-2"
	})

	expectedNsMap := map[string]string{
		"instance-1": "",
		"test-ns-1":  "",
		"test-ns-4":  "",
	}
	r.Instance = instanceOne
	r.setResourceManagedNamespaces()
	assert.Equal(t, expectedNsMap, r.ResourceManagedNamespaces)

	expectedNsMap = map[string]string{
		"instance-2": "",
		"test-ns-2":  "",
		"test-ns-3":  "",
	}
	r.Instance = instanceTwo
	r.setResourceManagedNamespaces()
	assert.Equal(t, expectedNsMap, r.ResourceManagedNamespaces)
}

func TestSetAppManagedNamespaces(t *testing.T) {
	r := makeTestReconciler(t,
		makeTestNs(func(n *corev1.Namespace) {
			n.Name = "instance-1"
		}),
		makeTestNs(func(n *corev1.Namespace) {
			n.Name = "instance-2"
		}),
		makeTestNs(func(n *corev1.Namespace) {
			n.Name = "test-ns-1"
		}),
		makeTestNs(func(n *corev1.Namespace) {
			n.Name = "test-ns-2"
			n.Labels[common.ArgoCDArgoprojKeyManagedByClusterArgoCD] = "instance-2"
		}),
		makeTestNs(func(n *corev1.Namespace) {
			n.Name = "test-ns-3"
		}),
		makeTestNs(func(n *corev1.Namespace) {
			n.Name = "test-ns-4"
		}),
		makeTestNs(func(n *corev1.Namespace) {
			n.Name = "test-ns-5"
			n.Labels["something"] = "random"
		}),
		makeTestNs(func(n *corev1.Namespace) {
			n.Name = "test-ns-6"
			n.Labels[common.ArgoCDArgoprojKeyManagedBy] = "instance-1"
		}),
	)

	instance := makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
		ac.Namespace = "instance-1"
		ac.Spec.SourceNamespaces = []string{"test-ns-1", "test-ns-2", "test-ns-3"}
	})

	// test with namespace scoped instance
	r.Instance = instance
	r.ClusterScoped = false
	r.setAppManagedNamespaces()
	expectedNsMap := map[string]string{}
	expectedLabelledNsList := []string{}
	assert.Equal(t, expectedNsMap, r.AppManagedNamespaces)

	listOptions := []client.ListOption{
		client.MatchingLabels{
			common.ArgoCDArgoprojKeyManagedByClusterArgoCD: r.Instance.Namespace,
		},
	}
	existingManagedNamespaces, _ := cluster.ListNamespaces(r.Client, listOptions)
	labelledNs := []string{}
	for _, n := range existingManagedNamespaces.Items {
		labelledNs = append(labelledNs, n.Name)
	}
	sort.Strings(labelledNs)
	assert.Equal(t, expectedLabelledNsList, labelledNs)

	// change instance to clusterscoped
	r.ClusterScoped = true
	r.setAppManagedNamespaces()
	expectedNsMap = map[string]string{
		"test-ns-1": "",
		"test-ns-3": "",
	}
	expectedLabelledNsList = []string{"test-ns-1", "test-ns-3"}
	assert.Equal(t, expectedNsMap, r.AppManagedNamespaces)

	existingManagedNamespaces, _ = cluster.ListNamespaces(r.Client, listOptions)
	labelledNs = []string{}
	for _, n := range existingManagedNamespaces.Items {
		labelledNs = append(labelledNs, n.Name)
	}
	sort.Strings(labelledNs)
	assert.Equal(t, expectedLabelledNsList, labelledNs)

	// update source namespace list
	r.Instance.Spec.SourceNamespaces = []string{"test-ns-4", "test-ns-5"}
	r.setAppManagedNamespaces()
	expectedNsMap = map[string]string{
		"test-ns-4": "",
		"test-ns-5": "",
	}
	expectedLabelledNsList = []string{"test-ns-4", "test-ns-5"}
	assert.Equal(t, expectedNsMap, r.AppManagedNamespaces)

	// check that namespace labels are updated
	existingManagedNamespaces, _ = cluster.ListNamespaces(r.Client, listOptions)
	labelledNs = []string{}
	for _, n := range existingManagedNamespaces.Items {
		labelledNs = append(labelledNs, n.Name)
	}
	sort.Strings(labelledNs)
	assert.Equal(t, expectedLabelledNsList, labelledNs)

}
