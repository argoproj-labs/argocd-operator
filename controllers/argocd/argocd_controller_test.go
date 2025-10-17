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
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	argoutil.SetRouteAPIFound(true) // Setup Route API for tests that call full reconciler
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(deletedAt(time.Now()))

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, configv1.Install, routev1.Install)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
	}
	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	deployment := &appsv1.Deployment{}
	if !apierrors.IsNotFound(r.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-redis",
		Namespace: testNamespace,
	}, deployment)) {
		t.Fatalf("expected not found error, got %#v\n", err)
	}
}

// TestReconcileArgoCD_DexWorkloads verifies that when dex is enabled, that the appropriate operator resources are created. When dex is disabled, the objects are verified to be removed.
func TestReconcileArgoCD_DexWorkloads(t *testing.T) {
	argoutil.SetRouteAPIFound(true) // Setup Route API for tests that call full reconciler
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	a.Spec.SSO = &argoproj.ArgoCDSSOSpec{
		Provider: argoproj.SSOProviderTypeDex,
		Dex: &argoproj.ArgoCDDexSpec{
			Config:         "test-config",
			OpenShiftOAuth: false,
		},
	}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, configv1.Install, routev1.Install)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	objectsToVerify := []client.Object{}

	dexRole := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "argocd-argocd-dex-server", Namespace: a.Namespace}}
	objectsToVerify = append(objectsToVerify, dexRole)

	dexRoleBinding := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "argocd-argocd-dex-server", Namespace: a.Namespace}}
	objectsToVerify = append(objectsToVerify, dexRoleBinding)

	dexServiceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "argocd-argocd-dex-server", Namespace: a.Namespace}}
	objectsToVerify = append(objectsToVerify, dexServiceAccount)

	dexService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "argocd-dex-server", Namespace: a.Namespace}}
	objectsToVerify = append(objectsToVerify, dexService)

	dexDeployment := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "argocd-dex-server", Namespace: a.Namespace}}
	objectsToVerify = append(objectsToVerify, dexDeployment)

	for _, objectToVerify := range objectsToVerify {
		t.Logf("verifying object %s", objectToVerify.GetName())
		err = r.Get(context.TODO(), client.ObjectKeyFromObject(objectToVerify), objectToVerify)
		assert.NoError(t, err)
		assert.True(t, len(objectToVerify.GetOwnerReferences()) > 0)
	}

	var secretList corev1.SecretList
	err = r.List(context.TODO(), &secretList, client.InNamespace(a.Namespace))
	assert.NoError(t, err)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: a.Namespace,
		},
	}
	err = r.Get(context.TODO(), client.ObjectKeyFromObject(configMap), configMap)
	assert.NoError(t, err)

	assert.Equal(t, configMap.Data["dex.config"], a.Spec.SSO.Dex.Config)

	var dexSecret *corev1.Secret

	for idx := range secretList.Items {
		secret := secretList.Items[idx]
		if strings.HasPrefix(secret.Name, "argocd-dex-server-token-") {
			dexSecret = &secret
			break
		}
	}
	assert.NotNil(t, dexSecret)

	err = r.Get(context.TODO(), client.ObjectKeyFromObject(a), a)
	assert.NoError(t, err)

	a.Spec.SSO = nil
	err = r.Update(context.TODO(), a)
	assert.NoError(t, err)

	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	for _, objectToVerify := range objectsToVerify {
		err = r.Get(context.TODO(), client.ObjectKeyFromObject(objectToVerify), objectToVerify)
		assert.Error(t, err)
	}

	err = r.Get(context.TODO(), client.ObjectKeyFromObject(configMap), configMap)
	assert.NoError(t, err)

	assert.Equal(t, configMap.Data["dex.config"], "")

}

func TestReconcileArgoCD_Reconcile(t *testing.T) {
	argoutil.SetRouteAPIFound(true) // Setup Route API for tests that call full reconciler
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, configv1.Install, routev1.Install)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	deployment := &appsv1.Deployment{}
	if err = r.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-redis",
		Namespace: testNamespace,
	}, deployment); err != nil {
		t.Fatalf("failed to find the redis deployment: %#v\n", err)
	}
}

func TestReconcileArgoCD_LabelSelector(t *testing.T) {
	argoutil.SetRouteAPIFound(true) // Setup Route API for tests that call full reconciler
	logf.SetLogger(ZapLogger(true))
	//ctx := context.Background()
	a := makeTestArgoCD(func(ac *argoproj.ArgoCD) {
		ac.Name = "argo-test-1"
		ac.Labels = map[string]string{"foo": "bar"}
	})
	b := makeTestArgoCD(func(ac *argoproj.ArgoCD) {
		ac.Name = "argo-test-2"
		ac.Labels = map[string]string{"testfoo": "testbar"}
	})
	c := makeTestArgoCD(func(ac *argoproj.ArgoCD) {
		ac.Name = "argo-test-3"
	})

	resObjs := []client.Object{a, b, c}
	subresObjs := []client.Object{a, b, c}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, configv1.Install, routev1.Install)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	rt := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(rt, a.Namespace, ""))

	// All ArgoCD instances should be reconciled if no label-selctor is applied to the operator.

	// Instance 'a'
	req1 := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
	}
	_, err := rt.Reconcile(context.TODO(), req1)
	assert.NoError(t, err)

	//Instance 'b'
	req2 := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      b.Name,
			Namespace: b.Namespace,
		},
	}
	_, err = rt.Reconcile(context.TODO(), req2)
	assert.NoError(t, err)

	//Instance 'c'
	req3 := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      c.Name,
			Namespace: c.Namespace,
		},
	}
	_, err = rt.Reconcile(context.TODO(), req3)
	assert.NoError(t, err)

	// Apply label-selector foo=bar to the operator.
	// Only Instance a should reconcile with matching label "foo=bar"
	// No reconciliation is expected for instance b and c and an error is expected.
	rt.LabelSelector = "foo=bar"
	reqTest := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
	}
	_, err = rt.Reconcile(context.TODO(), reqTest)
	assert.NoError(t, err)

	// Instance 'b' is not reconciled as the label does not match, error expected
	reqTest2 := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      b.Name,
			Namespace: b.Namespace,
		},
	}
	_, err = rt.Reconcile(context.TODO(), reqTest2)
	assert.Error(t, err)

	//Instance 'c' is not reconciled as there is no label, error expected
	reqTest3 := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      c.Name,
			Namespace: c.Namespace,
		},
	}
	_, err = rt.Reconcile(context.TODO(), reqTest3)
	assert.Error(t, err)
}

func TestReconcileArgoCD_Reconcile_RemoveManagedByLabelOnArgocdDeletion(t *testing.T) {
	argoutil.SetRouteAPIFound(true) // Setup Route API for tests that call full reconciler
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
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			nsArgocd := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
				Name: a.Namespace,
			}}
			err := r.Create(context.TODO(), nsArgocd)
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
			err = r.Create(context.TODO(), ns)
			assert.NoError(t, err)

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      a.Name,
					Namespace: a.Namespace,
				},
			}

			_, err = r.Reconcile(context.TODO(), req)
			assert.NoError(t, err)

			assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: ns.Name}, ns))
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
		a.DeletionTimestamp = &wrapped
		a.Finalizers = []string{"test: finalizaer"}
	}
}

func TestReconcileArgoCD_CleanUp(t *testing.T) {
	argoutil.SetRouteAPIFound(true) // Setup Route API for tests that call full reconciler
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(deletedAt(time.Now()), addFinalizer(common.ArgoCDDeletionFinalizer))

	resources := []client.Object{a}
	resources = append(resources, clusterResources(a)...)

	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resources, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
	}
	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	// check if cluster resources are deleted
	tt := []struct {
		name     string
		resource client.Object
	}{
		{
			fmt.Sprintf("ClusterRole %s", common.ArgoCDApplicationControllerComponent),
			newClusterRole(common.ArgoCDApplicationControllerComponent, []rbacv1.PolicyRule{}, a),
		},
		{
			fmt.Sprintf("ClusterRole %s", common.ArgoCDServerComponent),
			newClusterRole(common.ArgoCDServerComponent, []rbacv1.PolicyRule{}, a),
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
			found, err := argoutil.IsObjectFound(r.Client, "", test.name, test.resource)
			assert.Nil(t, err)
			if found {
				t.Errorf("Expected %s to be deleted", test.name)
			}
		})
	}

	// check if namespace label was removed
	ns := &corev1.Namespace{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: a.Namespace}, ns))
	if _, ok := ns.Labels[common.ArgoCDManagedByLabel]; ok {
		t.Errorf("Expected the label[%v] to be removed from the namespace[%v]", common.ArgoCDManagedByLabel, a.Namespace)
	}
}

func addFinalizer(finalizerParam string) argoCDOpt { //nolint:unparam
	return func(a *argoproj.ArgoCD) {
		a.Finalizers = append(a.Finalizers, finalizerParam)
	}
}

func clusterResources(argocd *argoproj.ArgoCD) []client.Object {
	return []client.Object{
		newClusterRole(common.ArgoCDApplicationControllerComponent, []rbacv1.PolicyRule{}, argocd),
		newClusterRole(common.ArgoCDServerComponent, []rbacv1.PolicyRule{}, argocd),
		newClusterRoleBindingWithname(common.ArgoCDApplicationControllerComponent, argocd),
		newClusterRoleBindingWithname(common.ArgoCDServerComponent, argocd),
	}
}

func TestReconcileArgoCD_Status_Condition(t *testing.T) {
	argoutil.SetRouteAPIFound(true) // Setup Route API for tests that call full reconciler
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCD(func(ac *argoproj.ArgoCD) {
		ac.Name = "argo-test-2"
		ac.Labels = map[string]string{"testfoo": "testbar"}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, configv1.Install, routev1.Install)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	rt := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
	rt.LabelSelector = "foo=bar"
	assert.NoError(t, createNamespace(rt, a.Namespace, ""))

	// Instance is not reconciled as the label does not match, error is expected
	reqTest := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
	}
	_, err := rt.Reconcile(context.TODO(), reqTest)
	assert.Error(t, err)

	// Verify condition is updated
	assert.NoError(t, rt.Get(context.TODO(), types.NamespacedName{Name: a.Name, Namespace: a.Namespace}, a))
	assert.Equal(t, a.Status.Conditions[0].Type, argoproj.ArgoCDConditionType)
	assert.Equal(t, a.Status.Conditions[0].Reason, argoproj.ArgoCDConditionReasonErrorOccurred)
	assert.Equal(t, a.Status.Conditions[0].Message, "the ArgoCD instance 'argocd/argo-test-2' does not match the label selector 'foo=bar' and skipping for reconciliation")
	assert.Equal(t, a.Status.Conditions[0].Status, metav1.ConditionFalse)

	rt.LabelSelector = "testfoo=testbar"

	// Now instance is reconciled as the label is same, no error is expected
	reqTest = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
	}
	_, err = rt.Reconcile(context.TODO(), reqTest)
	assert.NoError(t, err)

	// Verify condition is updated
	assert.NoError(t, rt.Get(context.TODO(), types.NamespacedName{Name: a.Name, Namespace: a.Namespace}, a))

	assert.Equal(t, a.Status.Conditions[0].Type, argoproj.ArgoCDConditionType)
	assert.Equal(t, a.Status.Conditions[0].Reason, argoproj.ArgoCDConditionReasonSuccess)
	assert.Equal(t, a.Status.Conditions[0].Message, "")
	assert.Equal(t, a.Status.Conditions[0].Status, metav1.ConditionTrue)
}

func TestReconcileArgoCD_Cleanup_RBACs_When_NamespaceManagement_Disabled(t *testing.T) {
	argoutil.SetRouteAPIFound(true) // Setup Route API for tests that call full reconciler
	namespace := testNamespace
	argoCD := makeArgoCD()
	argoCD.Spec.NamespaceManagement = nil

	// Setup a NamespaceManagement CR managed by this ArgoCD
	nsMgmt := &argoproj.NamespaceManagement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ns-mgmt",
			Namespace: namespace,
		},
		Spec: argoproj.NamespaceManagementSpec{
			ManagedBy: argoCD.Namespace,
		},
	}

	resObjs := []client.Object{argoCD, nsMgmt}
	subresObjs := []client.Object{argoCD}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, configv1.Install, routev1.Install)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, argoCD.Namespace, ""))

	// Create Role and RoleBinding
	client := r.K8sClient.(*testclient.Clientset)
	role := newRole("test-role", policyRuleForApplicationController(), argoCD)
	role.Namespace = namespace
	_, err := client.RbacV1().Roles(namespace).Create(context.TODO(), role, metav1.CreateOptions{})
	assert.NoError(t, err)

	roleBinding := newRoleBindingWithname("test-rolebinding", argoCD)
	roleBinding.Namespace = namespace
	_, err = client.RbacV1().RoleBindings(namespace).Create(context.TODO(), roleBinding, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Create secrets
	secret := argoutil.NewSecretWithSuffix(argoCD, "test")
	secret.Labels = map[string]string{common.ArgoCDSecretTypeLabel: "cluster"}
	secret.Data = map[string][]byte{
		"server":     []byte(common.ArgoCDDefaultServer),
		"namespaces": []byte(strings.Join([]string{namespace}, ",")),
	}
	_, err = client.CoreV1().Secrets(argoCD.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	assert.NoError(t, err)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      argoCD.Name,
			Namespace: argoCD.Namespace,
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	// Roles and Rolebinding should be deleted
	_, err = client.RbacV1().Roles(testNamespace).Get(context.TODO(), "test-role", metav1.GetOptions{})
	assert.ErrorContains(t, err, "not found")

	_, err = client.RbacV1().RoleBindings(testNamespace).Get(context.TODO(), "test-rolebinding", metav1.GetOptions{})
	assert.ErrorContains(t, err, "not found")

	// Secret should be deleted
	updatedSecret, err := client.CoreV1().Secrets(argoCD.Namespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "", string(updatedSecret.Data["namespaces"]))
}
