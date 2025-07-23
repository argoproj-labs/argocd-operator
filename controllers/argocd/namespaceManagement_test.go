package argocd

import (
	"context"
	"os"
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestReconcileNamespaceManagement_FeatureEnabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.NamespaceManagement = []argoproj.ManagedNamespaces{{
			Name:           "managed-ns",
			AllowManagedBy: true,
		}}
	})

	// Allowed NamespaceManagement
	nm := &argoproj.NamespaceManagement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespace-mgmt",
			Namespace: "managed-ns",
		},
		Spec: argoproj.NamespaceManagementSpec{
			ManagedBy: a.Namespace,
		},
	}

	// Disallowed NamespaceManagement (should trigger error)
	nmDisallowed := &argoproj.NamespaceManagement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespace-mgmt-disallowed",
			Namespace: "disallowed-ns",
		},
		Spec: argoproj.NamespaceManagementSpec{
			ManagedBy: a.Namespace,
		},
	}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a, nm, nmDisallowed}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	// Enable namespace management
	os.Setenv(common.EnableManagedNamespace, "true")
	defer os.Unsetenv(common.EnableManagedNamespace)

	// Create both CRs
	err := r.Client.Create(context.Background(), nm)
	assert.NoError(t, err)

	err = r.Client.Create(context.Background(), nmDisallowed)
	assert.NoError(t, err)

	// Reconcile
	err = r.reconcileNamespaceManagement(a)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Namespace disallowed-ns is not permitted for management by ArgoCD instance argocd based on NamespaceManagement rules")

	// Verify success status on allowed namespace
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
	assert.NotNil(t, reconciledCondition)
	assert.Equal(t, metav1.ConditionTrue, reconciledCondition.Status)
	assert.Equal(t, "Success", reconciledCondition.Reason)

	// Verify error status on disallowed namespace
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      nmDisallowed.Name,
		Namespace: nmDisallowed.Namespace,
	}, nmDisallowed)
	assert.NoError(t, err)

	reconciledCondition = nil
	for _, cond := range nmDisallowed.Status.Conditions {
		if cond.Type == "Reconciled" {
			reconciledCondition = &cond
			break
		}
	}
	assert.NotNil(t, reconciledCondition)
	assert.Equal(t, metav1.ConditionFalse, reconciledCondition.Status)
	assert.Equal(t, "ErrorOccurred", reconciledCondition.Reason)
}

func TestHandleFeatureDisable_NoNamespaceManagement(t *testing.T) {
	// fake Kubernetes client
	testClient := testclient.NewSimpleClientset()

	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	err := r.disableNamespaceManagement(a, testClient)
	// Assert: Should return no error since there are no NamespaceManagement CR and ArgoCD .spec.NamespaceManagement field is nil
	assert.NoError(t, err)
}

func TestMatchesNamespaceManagementRules(t *testing.T) {
	tests := []struct {
		name        string
		argocd      *argoproj.ArgoCD
		namespace   string
		expectMatch bool
	}{
		{
			name: "matches allowed pattern",
			argocd: &argoproj.ArgoCD{
				Spec: argoproj.ArgoCDSpec{
					NamespaceManagement: []argoproj.ManagedNamespaces{
						{Name: "allowed-*", AllowManagedBy: true},
					},
				},
			},
			namespace:   "allowed-ns",
			expectMatch: true,
		},
		{
			name: "does not match when AllowManagedBy is false",
			argocd: &argoproj.ArgoCD{
				Spec: argoproj.ArgoCDSpec{
					NamespaceManagement: []argoproj.ManagedNamespaces{
						{Name: "allowed-*", AllowManagedBy: false},
					},
				},
			},
			namespace:   "allowed-ns",
			expectMatch: false,
		},
		{
			name: "does not match pattern",
			argocd: &argoproj.ArgoCD{
				Spec: argoproj.ArgoCDSpec{
					NamespaceManagement: []argoproj.ManagedNamespaces{
						{Name: "allowed-*", AllowManagedBy: true},
					},
				},
			},
			namespace:   "denied-ns",
			expectMatch: false,
		},
		{
			name: "no namespace management configured",
			argocd: &argoproj.ArgoCD{
				Spec: argoproj.ArgoCDSpec{
					NamespaceManagement: nil,
				},
			},
			namespace:   "random-ns",
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := matchesNamespaceManagementRules(tt.argocd, tt.namespace)
			assert.Equal(t, tt.expectMatch, match)
		})
	}
}
