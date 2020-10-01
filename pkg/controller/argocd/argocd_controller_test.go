package argocd

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/argoproj-labs/argocd-operator/pkg/apis"
	argov1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
)

var (
	testNamespace  = "argocd"
	testArgoCDName = "argocd"
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

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
	}
	res, err := r.Reconcile(req)
	fatalIfError(t, err)
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

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
	}
	res, err := r.Reconcile(req)
	fatalIfError(t, err)
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

func makeTestReconciler(t *testing.T, objs ...runtime.Object) *ReconcileArgoCD {
	s := scheme.Scheme
	fatalIfError(t, apis.AddToScheme(s))

	cl := fake.NewFakeClient(objs...)
	return &ReconcileArgoCD{
		client: cl,
		scheme: s,
	}
}

func fatalIfError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

type argoCDOpt func(*argov1alpha1.ArgoCD)

func makeTestArgoCD(opts ...argoCDOpt) *argov1alpha1.ArgoCD {
	a := &argov1alpha1.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testArgoCDName,
			Namespace: testNamespace,
		},
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func deletedAt(now time.Time) argoCDOpt {
	return func(a *argov1alpha1.ArgoCD) {
		wrapped := metav1.NewTime(now)
		a.ObjectMeta.DeletionTimestamp = &wrapped
	}
}
