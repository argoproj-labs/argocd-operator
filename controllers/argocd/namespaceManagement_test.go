package argocd

import (
	"context"
	"os"
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestReconcileNamespaceManagement_FeatureEnabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.NamespaceManagement = []argoproj.ManagedNamespaces{{
			Name:           "managed-ns",
			AllowManagedBy: true,
		},
		}
	})

	nm := &argoproj.NamespaceManagement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespace-mgmt",
			Namespace: "managed-ns",
		},
		Spec: argoproj.NamespaceManagementSpec{
			ManagedBy: a.Namespace,
		},
	}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a, nm}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	// Namespace management enabled, ensure namespaces are processed
	os.Setenv(common.EnableManagedNamespace, "true")

	err := r.Client.Create(context.Background(), nm)
	assert.NoError(t, err)

	err = r.reconcileNamespaceManagement(a)
	assert.NoError(t, err)

	assert.NotNil(t, r.ManagedNamespaces)
	assert.Contains(t, getNamespaceNames(r.ManagedNamespaces), "managed-ns")

	// Verify Status Conditions are Updated Properly for Success
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      nm.Name,
		Namespace: nm.Namespace,
	}, nm)
	assert.NoError(t, err)

	var reconciledCondition *metav1.Condition
	for _, cond := range nm.Status.Conditions {
		if cond.Type == "Reconciled" {
			reconciledCondition = &cond
			break
		}
	}

	assert.Equal(t, metav1.ConditionTrue, reconciledCondition.Status, "Reconciled condition should be True")
	assert.Equal(t, "Success", reconciledCondition.Reason, "Reconciled condition reason should be Success")

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      nm.Name,
		Namespace: nm.Namespace,
	}, nm)
	assert.NoError(t, err)

	nm.Spec.ManagedBy = "other-argocd"
	err = r.Client.Update(context.Background(), nm)
	assert.NoError(t, err)

	err = r.reconcileNamespaceManagement(a)
	expectedError := "error: ArgoCD does not allow management of this namespace"
	assert.Error(t, err, expectedError)

	// Verify Status Conditions are Updated Properly for ErrorOccurred
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      nm.Name,
		Namespace: nm.Namespace,
	}, nm)
	assert.NoError(t, err)

	for _, cond := range nm.Status.Conditions {
		if cond.Type == "Reconciled" {
			reconciledCondition = &cond
			break
		}
	}

	assert.Equal(t, metav1.ConditionFalse, reconciledCondition.Status, "Reconciled condition should be False")
	assert.Equal(t, "ErrorOccurred", reconciledCondition.Reason, "Reconciled condition reason should be ErrorOccurred")

}

func TestReconcileNamespaceManagement_FeatureDisabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.NamespaceManagement = []argoproj.ManagedNamespaces{{
			Name:           "managed-ns",
			AllowManagedBy: true,
		},
		}
	})

	nm := &argoproj.NamespaceManagement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespace-mgmt",
			Namespace: "managed-ns",
		},
		Spec: argoproj.NamespaceManagementSpec{
			ManagedBy: a.Namespace,
		},
	}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a, nm}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	// 1) Namespace management disabled and there exists a a NamespaceManagement CR and related ArgoCD instances with namespace management,
	// ensure feature cleanup is called and fields are removed
	os.Setenv(common.EnableManagedNamespace, "false")
	defer os.Unsetenv(common.EnableManagedNamespace)

	err := r.Client.Create(context.Background(), nm)
	assert.NoError(t, err)

	err = r.reconcileNamespaceManagement(a)
	assert.NoError(t, err)

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      nm.Name,
		Namespace: nm.Namespace,
	}, nm)
	assert.NoError(t, err)

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      a.Name,
		Namespace: a.Namespace,
	}, a)
	assert.NoError(t, err)

	assert.Nil(t, a.Spec.NamespaceManagement, "NamespaceManagement should be removed")
	assert.Empty(t, nm.Spec.ManagedBy, "ManagedBy should be removed")
}

func TestReconcileNamespaceManagement_FeatureDisabled_NoResources(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.NamespaceManagement = []argoproj.ManagedNamespaces{{
			Name:           "managed-ns",
			AllowManagedBy: true,
		},
		}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	// If namespace management is disabled and neither a NamespaceManagement CR
	// and any related ArgoCD instances with namespace management are found, return nil
	os.Setenv(common.EnableManagedNamespace, "false")
	defer os.Unsetenv(common.EnableManagedNamespace)

	err := r.reconcileNamespaceManagement(a)
	assert.NoError(t, err)
}

func TestMatchesNamespaceManagementRules(t *testing.T) {
	argocd := &argoproj.ArgoCD{
		Spec: argoproj.ArgoCDSpec{
			NamespaceManagement: []argoproj.ManagedNamespaces{
				{Name: "allowed-*"},
			},
		},
	}
	assert.True(t, matchesNamespaceManagementRules(argocd, "allowed-ns"))
	assert.True(t, matchesNamespaceManagementRules(argocd, "allowed-ns1"))
	assert.False(t, matchesNamespaceManagementRules(argocd, "denied-ns"))
}

func getNamespaceNames(nsList *corev1.NamespaceList) []string {
	var names []string
	for _, ns := range nsList.Items {
		names = append(names, ns.Name)
	}
	return names
}
