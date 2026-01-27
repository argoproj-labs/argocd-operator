package argocd

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
	reqState := &RequestState{}

	// Enable namespace management
	os.Setenv(common.EnableManagedNamespace, "true")
	defer os.Unsetenv(common.EnableManagedNamespace)

	// Create both CRs
	err := r.Create(context.Background(), nm)
	assert.NoError(t, err)

	err = r.Create(context.Background(), nmDisallowed)
	assert.NoError(t, err)

	// Reconcile
	err = r.reconcileNamespaceManagement(a, reqState)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Namespace disallowed-ns is not permitted for management by ArgoCD instance argocd based on NamespaceManagement rules")

	// Verify success status on allowed namespace
	err = r.Get(context.TODO(), types.NamespacedName{
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

	reconciledCondition = nil

	// Verify error status on disallowed namespace
	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      nmDisallowed.Name,
		Namespace: nmDisallowed.Namespace,
	}, nmDisallowed)
	assert.NoError(t, err)

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
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := r.disableNamespaceManagement(a, r.K8sClient)
	// Assert: Should return no error since there are no NamespaceManagement CR and ArgoCD .spec.NamespaceManagement field is nil
	assert.NoError(t, err)
}

func TestHandleFeatureDisable_NamespaceCRsExistButNoMatch(t *testing.T) {
	a := makeTestArgoCD()

	// Create a NamespaceManagement CR that is managed by a different ArgoCD instance
	nm := &argoproj.NamespaceManagement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nm1",
			Namespace: "ns-1",
		},
		Spec: argoproj.NamespaceManagementSpec{
			ManagedBy: "other-argocd",
		},
	}

	resObjs := []client.Object{a, nm}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := r.disableNamespaceManagement(a, r.K8sClient)
	assert.NoError(t, err)
}

func TestHandleFeatureDisable_NamespaceMatchesPattern_RBACDeleted(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.NamespaceManagement = []argoproj.ManagedNamespaces{
		{
			Name:           "ns-*",
			AllowManagedBy: true,
		},
	}

	nm := &argoproj.NamespaceManagement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nm1",
			Namespace: "ns-1",
		},
		Spec: argoproj.NamespaceManagementSpec{
			ManagedBy: "argocd",
		},
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns-1",
			Labels: map[string]string{
				"some": "label",
			},
		},
	}

	resObjs := []client.Object{a, nm, ns}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset(ns))

	err := r.disableNamespaceManagement(a, r.K8sClient)
	assert.NoError(t, err)
}

func TestHandleFeatureDisable_SkipManagedByLabel(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.NamespaceManagement = []argoproj.ManagedNamespaces{
		{
			Name:           "ns-*",
			AllowManagedBy: true,
		},
	}

	nm := &argoproj.NamespaceManagement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nm1",
			Namespace: "ns-managed",
		},
		Spec: argoproj.NamespaceManagementSpec{
			ManagedBy: "argocd",
		},
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns-managed",
			Labels: map[string]string{
				common.ArgoCDManagedByLabel: "ns-managed",
			},
		},
	}

	resObjs := []client.Object{a, nm, ns}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset(ns))

	err := r.disableNamespaceManagement(a, r.K8sClient)
	assert.NoError(t, err)
}

func TestHandleFeatureDisable_NoPatternMatch(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.NamespaceManagement = []argoproj.ManagedNamespaces{
		{
			Name:           "prod-*",
			AllowManagedBy: true,
		},
	}

	nm := &argoproj.NamespaceManagement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nm1",
			Namespace: "dev-ns",
		},
		Spec: argoproj.NamespaceManagementSpec{
			ManagedBy: "argocd",
		},
	}

	resObjs := []client.Object{a, nm}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := r.disableNamespaceManagement(a, r.K8sClient)
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
			var allowedPatterns []string
			for _, nm := range tt.argocd.Spec.NamespaceManagement {
				if nm.AllowManagedBy {
					allowedPatterns = append(allowedPatterns, nm.Name)
				}
			}
			match := matchesNamespaceManagementRules(allowedPatterns, tt.namespace)
			assert.Equal(t, tt.expectMatch, match)
		})
	}
}

func TestReconcileNamespaceManagement_FeatureEnabled_NoCRs(t *testing.T) {
	// Feature enabled but no NamespaceManagement CRs exist
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.NamespaceManagement = []argoproj.ManagedNamespaces{
			{Name: "managed-ns", AllowManagedBy: true},
		}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a} // no NamespaceManagement CRs
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
	reqState := &RequestState{}

	os.Setenv(common.EnableManagedNamespace, "true")
	defer os.Unsetenv(common.EnableManagedNamespace)

	err := r.reconcileNamespaceManagement(a, reqState)
	assert.NoError(t, err)

	// Should only include ArgoCD namespace
	found := false
	for _, ns := range reqState.ManagedNamespaces.Items {
		if ns.Name == a.Namespace {
			found = true
		}
	}
	assert.True(t, found)
	assert.Len(t, reqState.ManagedNamespaces.Items, 1)
}

func TestReconcileNamespaceManagement_DifferentManagedBy(t *testing.T) {
	// NamespaceManagement CR for a different ArgoCD instance (should be skipped)
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Namespace = "argocd"
		a.Spec.NamespaceManagement = []argoproj.ManagedNamespaces{
			{Name: "managed-ns", AllowManagedBy: true},
		}
	})

	nmOther := &argoproj.NamespaceManagement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespace-mgmt-other",
			Namespace: "managed-ns",
		},
		Spec: argoproj.NamespaceManagementSpec{
			ManagedBy: "some-other-argocd",
		},
	}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a, nmOther}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	reqState := &RequestState{}

	err := r.Create(context.Background(), nmOther)
	assert.NoError(t, err)

	os.Setenv(common.EnableManagedNamespace, "true")
	defer os.Unsetenv(common.EnableManagedNamespace)

	err = r.reconcileNamespaceManagement(a, reqState)
	assert.NoError(t, err)

	// Should only include ArgoCD namespace
	assert.Len(t, reqState.ManagedNamespaces.Items, 1)
	assert.Equal(t, "argocd", reqState.ManagedNamespaces.Items[0].Name)
}

func TestReconcileNamespaceManagement_ExplicitlyDisallowed(t *testing.T) {
	// Explicitly disallowed namespace (AllowManagedBy=false)
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Namespace = "argocd"
		a.Spec.NamespaceManagement = []argoproj.ManagedNamespaces{
			{Name: "deny-ns", AllowManagedBy: false},
		}
	})
	nm := &argoproj.NamespaceManagement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespace-mgmt-deny",
			Namespace: "deny-ns",
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
	reqState := &RequestState{}
	logf.SetLogger(ZapLogger(true))

	err := r.Create(context.Background(), nm)
	assert.NoError(t, err)

	os.Setenv(common.EnableManagedNamespace, "true")
	defer os.Unsetenv(common.EnableManagedNamespace)

	err = r.reconcileNamespaceManagement(a, reqState)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Namespace deny-ns is not permitted for management by ArgoCD instance argocd based on NamespaceManagement rules")
}

func TestReconcileNamespaceManagement_DeduplicateNamespaces(t *testing.T) {
	// Duplicate namespaces in status should not be added multiple times
	// duplicates are not added to r.ManagedNamespaces.
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.NamespaceManagement = []argoproj.ManagedNamespaces{
			{Name: "ns-1", AllowManagedBy: true},
		}
	})

	nm := &argoproj.NamespaceManagement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nm1",
			Namespace: "ns-1",
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
	reqState := &RequestState{}

	err := r.Create(context.Background(), nm)
	assert.NoError(t, err)

	reqState.ManagedNamespaces = &corev1.NamespaceList{
		Items: []corev1.Namespace{
			{ObjectMeta: metav1.ObjectMeta{Name: "ns-1"}},
		},
	}

	os.Setenv(common.EnableManagedNamespace, "true")
	defer os.Unsetenv(common.EnableManagedNamespace)

	err = r.reconcileNamespaceManagement(a, reqState)
	assert.NoError(t, err)

	count := 0
	for _, ns := range reqState.ManagedNamespaces.Items {
		if ns.Name == "ns-1" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}
