package argocd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
)

func TestReconcileArgoCD_reconcile_ServerIngress_ingressClassName(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	nginx := "nginx"

	tests := []struct {
		name             string
		ingressClassName *string
	}{
		{
			name:             "undefined ingress class name",
			ingressClassName: nil,
		},
		{
			name:             "ingress class name specified",
			ingressClassName: &nginx,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.Server.Ingress.Enabled = true
				a.Spec.Server.Ingress.IngressClassName = test.ingressClassName
			})

			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			err := r.reconcileArgoServerIngress(a)
			assert.NoError(t, err)

			ingress := &networkingv1.Ingress{}
			err = r.Client.Get(context.TODO(), types.NamespacedName{
				Name:      "argocd-server",
				Namespace: testNamespace,
			}, ingress)
			assert.NoError(t, err)
			assert.Equal(t, test.ingressClassName, ingress.Spec.IngressClassName)
		})
	}
}
func TestReconcileArgoCD_reconcile_ServerIngress_serverHost(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	nginx := "nginx"

	tests := []struct {
		name             string
		ingressClassName *string
		host             string
	}{
		{
			name:             "New Server host specified",
			ingressClassName: &nginx,
			host:             "foo.bar",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.Server.Ingress.Enabled = true
				a.Spec.Server.Ingress.IngressClassName = test.ingressClassName
			})

			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
			err := r.reconcileArgoServerIngress(a)
			assert.NoError(t, err)

			ingress := &networkingv1.Ingress{}
			err = r.Client.Get(context.TODO(), types.NamespacedName{
				Name:      "argocd-server",
				Namespace: testNamespace,
			}, ingress)
			assert.NoError(t, err)
			assert.Equal(t, test.ingressClassName, ingress.Spec.IngressClassName)
			assert.Equal(t, "argocd", ingress.Spec.TLS[0].Hosts[0])
			a = makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.Server.Ingress.Enabled = true
				a.Spec.Server.Ingress.IngressClassName = test.ingressClassName
				a.Spec.Server.Host = test.host
			})

			err = r.reconcileArgoServerIngress(a)
			assert.NoError(t, err)
			err = r.Client.Get(context.TODO(), types.NamespacedName{
				Name:      "argocd-server",
				Namespace: testNamespace,
			}, ingress)
			assert.NoError(t, err)
			assert.Equal(t, test.host, ingress.Spec.TLS[0].Hosts[0])
			assert.Equal(t, test.host, ingress.Spec.Rules[0].Host)
			assert.Equal(t, test.ingressClassName, ingress.Spec.IngressClassName)
		})
	}
}
func TestReconcileArgoCD_reconcile_ServerIngress_ingressClassName_update(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	nginx := "nginx"
	existingIngressClassName := "test-name"

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Server.Ingress.Enabled = true
		a.Spec.Server.Ingress.IngressClassName = &nginx
	})

	// Existing ingress with different ingressClassName
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-server",
			Namespace: a.Namespace,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &existingIngressClassName,
		},
	}

	resObjs := []client.Object{a, ingress}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := r.reconcileArgoServerIngress(a)
	assert.NoError(t, err)

	updatedIngress := &networkingv1.Ingress{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-server",
		Namespace: testNamespace,
	}, updatedIngress)
	assert.NoError(t, err)
	assert.Equal(t, *a.Spec.Server.Ingress.IngressClassName, *updatedIngress.Spec.IngressClassName)

}

func TestReconcileArgoCD_reconcile_ServerGRPCIngress_ingressClassName(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	nginx := "nginx"

	tests := []struct {
		name             string
		ingressClassName *string
	}{
		{
			name:             "undefined ingress class name",
			ingressClassName: nil,
		},
		{
			name:             "ingress class name specified",
			ingressClassName: &nginx,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.Server.GRPC.Ingress.Enabled = true
				a.Spec.Server.GRPC.Ingress.IngressClassName = test.ingressClassName
			})

			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			err := r.reconcileArgoServerGRPCIngress(a)
			assert.NoError(t, err)

			ingress := &networkingv1.Ingress{}
			err = r.Client.Get(context.TODO(), types.NamespacedName{
				Name:      "argocd-grpc",
				Namespace: testNamespace,
			}, ingress)
			assert.NoError(t, err)
			assert.Equal(t, test.ingressClassName, ingress.Spec.IngressClassName)
		})
	}
}

func TestReconcileArgoCD_reconcile_PrometheusIngress_ingressClassName(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	nginx := "nginx"

	tests := []struct {
		name             string
		ingressClassName *string
	}{
		{
			name:             "undefined ingress class name",
			ingressClassName: nil,
		},
		{
			name:             "ingress class name specified",
			ingressClassName: &nginx,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.Prometheus.Enabled = true
				a.Spec.Prometheus.Ingress.Enabled = true
				a.Spec.Prometheus.Ingress.IngressClassName = test.ingressClassName
			})

			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			err := r.reconcilePrometheusIngress(a)
			assert.NoError(t, err)

			ingress := &networkingv1.Ingress{}
			err = r.Client.Get(context.TODO(), types.NamespacedName{
				Name:      "argocd-prometheus",
				Namespace: testNamespace,
			}, ingress)
			assert.NoError(t, err)
			assert.Equal(t, test.ingressClassName, ingress.Spec.IngressClassName)
		})
	}
}

func TestReconcileApplicationSetService_Ingress(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	obj := argoproj.ArgoCDApplicationSet{
		WebhookServer: argoproj.WebhookServerSpec{
			Ingress: argoproj.ArgoCDIngressSpec{
				Enabled: true,
			},
		},
	}
	a.Spec.ApplicationSet = &obj

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	ingress := newIngressWithSuffix(common.ApplicationSetServiceNameSuffix, a)
	assert.NoError(t, r.reconcileApplicationSetControllerIngress(a))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: ingress.Namespace, Name: ingress.Name}, ingress))
}
