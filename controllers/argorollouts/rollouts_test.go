package argorollouts

import (
	"context"
	"fmt"
	"testing"
	"time"

	argov1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ reconcile.Reconciler = &ArgoRolloutsReconciler{}

func TestReconcileArgoRollouts_verifyRolloutsResources(t *testing.T) {
	a := makeTestArgoRollouts()

	r := makeTestReconciler(t, a)
	assert.NoError(t, createNamespace(r, a.Namespace))

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
		Name:      "example-rollouts-argo-rollouts",
		Namespace: testNamespace,
	}, deployment); err != nil {
		t.Fatalf("failed to find the rollouts deployment: %#v\n", err)
	}

	sa := &corev1.ServiceAccount{}
	if err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "example-rollouts-argo-rollouts",
		Namespace: testNamespace,
	}, sa); err != nil {
		t.Fatalf("failed to find the rollouts serviceaccount: %#v\n", err)
	}

	role := &v1.Role{}
	if err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "example-rollouts-argo-rollouts",
		Namespace: testNamespace,
	}, role); err != nil {
		t.Fatalf("failed to find the rollouts role: %#v\n", err)
	}

	rolebinding := &v1.RoleBinding{}
	if err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "example-rollouts-argo-rollouts",
		Namespace: testNamespace,
	}, rolebinding); err != nil {
		t.Fatalf("failed to find the rollouts rolebinding: %#v\n", err)
	}

	service := &corev1.Service{}
	if err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "example-rollouts-argo-rollouts",
		Namespace: testNamespace,
	}, service); err != nil {
		t.Fatalf("failed to find the rollouts service: %#v\n", err)
	}

	secret := &corev1.Secret{}
	if err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "argo-rollouts-notification-secret",
		Namespace: testNamespace,
	}, secret); err != nil {
		t.Fatalf("failed to find the rollouts secret: %#v\n", err)
	}
}

func TestReconcileArgoRollouts_CleanUp(t *testing.T) {

	a := makeTestArgoRollouts(deletedAt(time.Now()), addFinalizer(common.ArgoCDDeletionFinalizer))

	resources := []runtime.Object{a}

	r := makeTestReconciler(t, resources...)
	assert.NoError(t, createNamespace(r, a.Namespace))

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

	// check if rollouts resources are deleted
	tt := []struct {
		name     string
		resource client.Object
	}{
		{
			fmt.Sprintf("ServiceAccount %s", "example-rollouts-argo-rollouts"),
			newServiceAccount(a),
		},
		{
			fmt.Sprintf("Role %s", "example-rollouts-argo-rollouts"),
			newRole("example-rollouts-argo-rollouts", []v1.PolicyRule{}, a),
		},
		{
			fmt.Sprintf("RoleBinding %s", "example-rollouts-argo-rollouts"),
			newRoleBinding(a),
		},
		{
			fmt.Sprintf("Deployment %s", "example-rollouts-argo-rollouts"),
			newDeployment(a),
		},
	}

	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			if argoutil.IsObjectFound(r.Client, "", test.name, test.resource) {
				t.Errorf("Expected %s to be deleted", test.name)
			}
		})
	}
}

func deletedAt(now time.Time) argoCDOpt {
	return func(a *argov1alpha1.ArgoRollouts) {
		wrapped := metav1.NewTime(now)
		a.ObjectMeta.DeletionTimestamp = &wrapped
	}
}

func addFinalizer(finalizer string) argoCDOpt {
	return func(a *argov1alpha1.ArgoRollouts) {
		a.Finalizers = append(a.Finalizers, finalizer)
	}
}
