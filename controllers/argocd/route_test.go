package argocd

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/google/go-cmp/cmp"
	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
)

func TestReconcileRouteSetLabels(t *testing.T) {
	routeAPIFound = true
	ctx := context.Background()
	logf.SetLogger(ZapLogger(true))
	argoCD := makeArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Server.Route.Enabled = true
		labels := make(map[string]string)
		labels["my-key"] = "my-value"
		a.Spec.Server.Route.Labels = labels
	})

	resObjs := []client.Object{argoCD}
	subresObjs := []client.Object{argoCD}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, configv1.Install, routev1.Install)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, createNamespace(r, argoCD.Namespace, ""))

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      testArgoCDName,
			Namespace: testNamespace,
		},
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	loaded := &routev1.Route{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: testArgoCDName + "-server", Namespace: testNamespace}, loaded)
	fatalIfError(t, err, "failed to load route %q: %s", testArgoCDName+"-server", err)

	if diff := cmp.Diff("my-value", loaded.Labels["my-key"]); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}

}
func TestReconcileRouteSetsInsecure(t *testing.T) {
	routeAPIFound = true
	ctx := context.Background()
	logf.SetLogger(ZapLogger(true))
	argoCD := makeArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Server.Route.Enabled = true
	})

	resObjs := []client.Object{argoCD}
	subresObjs := []client.Object{argoCD}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, configv1.Install, routev1.Install)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, createNamespace(r, argoCD.Namespace, ""))

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      testArgoCDName,
			Namespace: testNamespace,
		},
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	loaded := &routev1.Route{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: testArgoCDName + "-server", Namespace: testNamespace}, loaded)
	fatalIfError(t, err, "failed to load route %q: %s", testArgoCDName+"-server", err)

	wantTLSConfig := &routev1.TLSConfig{
		Termination:                   routev1.TLSTerminationReencrypt,
		InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
	}
	if diff := cmp.Diff(wantTLSConfig, loaded.Spec.TLS); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}
	wantPort := &routev1.RoutePort{
		TargetPort: intstr.FromString("https"),
	}
	if diff := cmp.Diff(wantPort, loaded.Spec.Port); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}

	// second reconciliation after changing the Insecure flag.
	err = r.Client.Get(ctx, req.NamespacedName, argoCD)
	fatalIfError(t, err, "failed to load ArgoCD %q: %s", testArgoCDName+"-server", err)

	argoCD.Spec.Server.Insecure = true
	err = r.Client.Update(ctx, argoCD)
	fatalIfError(t, err, "failed to update the ArgoCD: %s", err)

	_, err = r.Reconcile(context.TODO(), req)
	fatalIfError(t, err, "reconcile: (%v): %s", req, err)

	loaded = &routev1.Route{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: testArgoCDName + "-server", Namespace: testNamespace}, loaded)
	fatalIfError(t, err, "failed to load route %q: %s", testArgoCDName+"-server", err)

	wantTLSConfig = &routev1.TLSConfig{
		InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
		Termination:                   routev1.TLSTerminationEdge,
	}
	if diff := cmp.Diff(wantTLSConfig, loaded.Spec.TLS); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}
	wantPort = &routev1.RoutePort{
		TargetPort: intstr.FromString("http"),
	}
	if diff := cmp.Diff(wantPort, loaded.Spec.Port); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}
}

func TestReconcileRouteUnsetsInsecure(t *testing.T) {
	routeAPIFound = true
	ctx := context.Background()
	logf.SetLogger(ZapLogger(true))
	argoCD := makeArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Server.Route.Enabled = true
		a.Spec.Server.Insecure = true
	})

	resObjs := []client.Object{argoCD}
	subresObjs := []client.Object{argoCD}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, configv1.Install, routev1.Install)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, createNamespace(r, argoCD.Namespace, ""))

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      testArgoCDName,
			Namespace: testNamespace,
		},
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	loaded := &routev1.Route{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: testArgoCDName + "-server", Namespace: testNamespace}, loaded)
	fatalIfError(t, err, "failed to load route %q: %s", testArgoCDName+"-server", err)

	wantTLSConfig := &routev1.TLSConfig{
		InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
		Termination:                   routev1.TLSTerminationEdge,
	}
	if diff := cmp.Diff(wantTLSConfig, loaded.Spec.TLS); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}
	wantPort := &routev1.RoutePort{
		TargetPort: intstr.FromString("http"),
	}
	if diff := cmp.Diff(wantPort, loaded.Spec.Port); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}

	// second reconciliation after changing the Insecure flag.
	err = r.Client.Get(ctx, req.NamespacedName, argoCD)
	fatalIfError(t, err, "failed to load ArgoCD %q: %s", testArgoCDName+"-server", err)

	argoCD.Spec.Server.Insecure = false
	err = r.Client.Update(ctx, argoCD)
	fatalIfError(t, err, "failed to update the ArgoCD: %s", err)

	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	loaded = &routev1.Route{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: testArgoCDName + "-server", Namespace: testNamespace}, loaded)
	fatalIfError(t, err, "failed to load route %q: %s", testArgoCDName+"-server", err)

	wantTLSConfig = &routev1.TLSConfig{
		Termination:                   routev1.TLSTerminationReencrypt,
		InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
	}
	if diff := cmp.Diff(wantTLSConfig, loaded.Spec.TLS); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}
	wantPort = &routev1.RoutePort{
		TargetPort: intstr.FromString("https"),
	}
	if diff := cmp.Diff(wantPort, loaded.Spec.Port); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}
}

func TestReconcileRouteApplicationSetHost(t *testing.T) {
	routeAPIFound = true
	ctx := context.Background()
	logf.SetLogger(ZapLogger(true))
	argoCD := makeArgoCD(func(a *argoproj.ArgoCD) {

		a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
			WebhookServer: argoproj.WebhookServerSpec{
				Host: "webhook-test.org",
				Route: argoproj.ArgoCDRouteSpec{
					Enabled: true,
				},
			},
		}
	})

	resObjs := []client.Object{argoCD}
	subresObjs := []client.Object{argoCD}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, configv1.Install, routev1.Install)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, createNamespace(r, argoCD.Namespace, ""))

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      testArgoCDName,
			Namespace: testNamespace,
		},
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	loaded := &routev1.Route{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-%s-%s", testArgoCDName, common.ApplicationSetServiceNameSuffix, "webhook"), Namespace: testNamespace}, loaded)
	fatalIfError(t, err, "failed to load route %q: %s", testArgoCDName+"-server", err)

	wantTLSConfig := &routev1.TLSConfig{
		InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
		Termination:                   routev1.TLSTerminationEdge,
	}
	if diff := cmp.Diff(wantTLSConfig, loaded.Spec.TLS); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}

	if diff := cmp.Diff(argoCD.Spec.ApplicationSet.WebhookServer.Host, loaded.Spec.Host); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}
}

func TestReconcileRouteApplicationSetTlsTermination(t *testing.T) {
	routeAPIFound = true
	ctx := context.Background()
	logf.SetLogger(ZapLogger(true))
	argoCD := makeArgoCD(func(a *argoproj.ArgoCD) {

		a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
			WebhookServer: argoproj.WebhookServerSpec{
				Host: "webhook-test.org",
				Route: argoproj.ArgoCDRouteSpec{
					Enabled: true,
					TLS: &routev1.TLSConfig{
						Termination:                   routev1.TLSTerminationPassthrough,
						InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
					},
				},
			},
		}
	})

	resObjs := []client.Object{argoCD}
	subresObjs := []client.Object{argoCD}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, configv1.Install, routev1.Install)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, createNamespace(r, argoCD.Namespace, ""))

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      testArgoCDName,
			Namespace: testNamespace,
		},
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	loaded := &routev1.Route{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-%s-%s", testArgoCDName, common.ApplicationSetServiceNameSuffix, "webhook"), Namespace: testNamespace}, loaded)
	fatalIfError(t, err, "failed to load route %q: %s", testArgoCDName+"-server", err)

	wantTLSConfig := &routev1.TLSConfig{
		InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
		Termination:                   routev1.TLSTerminationPassthrough,
	}
	if diff := cmp.Diff(wantTLSConfig, loaded.Spec.TLS); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}

	if diff := cmp.Diff(argoCD.Spec.ApplicationSet.WebhookServer.Host, loaded.Spec.Host); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}
}

func TestReconcileRouteApplicationSetTls(t *testing.T) {
	routeAPIFound = true
	ctx := context.Background()
	logf.SetLogger(ZapLogger(true))
	wildcardPolicy := routev1.WildcardPolicyType("subdomain")

	argoCD := makeArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
			WebhookServer: argoproj.WebhookServerSpec{
				Route: argoproj.ArgoCDRouteSpec{
					Enabled: true,
					TLS: &routev1.TLSConfig{
						Certificate:                   "test-certificate",
						Key:                           "test-key",
						CACertificate:                 "test-ca-certificate",
						DestinationCACertificate:      "test-destination-ca-certificate",
						InsecureEdgeTerminationPolicy: "Redirect",
					},
					Annotations:    map[string]string{"my-annotation-key": "my-annotation-value"},
					Labels:         map[string]string{"my-label-key": "my-label-value"},
					WildcardPolicy: &wildcardPolicy,
				},
			},
		}
	})

	resObjs := []client.Object{argoCD}
	subresObjs := []client.Object{argoCD}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, configv1.Install, routev1.Install)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, createNamespace(r, argoCD.Namespace, ""))

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      testArgoCDName,
			Namespace: testNamespace,
		},
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	loaded := &routev1.Route{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-%s-%s", testArgoCDName, common.ApplicationSetServiceNameSuffix, "webhook"), Namespace: testNamespace}, loaded)
	fatalIfError(t, err, "failed to load route %q: %s", testArgoCDName+"-server", err)

	wantTLSConfig := &routev1.TLSConfig{
		Termination:                   routev1.TLSTerminationEdge,
		Certificate:                   "test-certificate",
		Key:                           "test-key",
		CACertificate:                 "test-ca-certificate",
		DestinationCACertificate:      "test-destination-ca-certificate",
		InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
	}
	if diff := cmp.Diff(wantTLSConfig, loaded.Spec.TLS); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}

	assert.Empty(t, loaded.Spec.Host)

	wantPort := &routev1.RoutePort{
		TargetPort: intstr.FromString("webhook"),
	}
	if diff := cmp.Diff(wantPort, loaded.Spec.Port); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}

	if diff := cmp.Diff("my-annotation-value", loaded.Annotations["my-annotation-key"]); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}

	if diff := cmp.Diff("my-label-value", loaded.Labels["my-label-key"]); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}

	if diff := cmp.Diff(wildcardPolicy, loaded.Spec.WildcardPolicy); diff != "" {
		t.Fatalf("failed to reconcile route:\n%s", diff)
	}
}

func TestReconcileRouteForShorteningHostname(t *testing.T) {
	routeAPIFound = true
	ctx := context.Background()
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		testName string
		expected string
		hostname string
	}{
		{
			testName: "longHostname",
			hostname: "myhostnameaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.redhat.com",
			expected: "myhostnameaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.redhat.com",
		},
		{
			testName: "twentySixLetterHostname",
			hostname: "myhostnametwentysixletteraaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.redhat.com",
			expected: "myhostnametwentysixletteraaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.redhat.com",
		},
	}

	for _, v := range tests {
		t.Run(v.testName, func(t *testing.T) {

			argoCD := makeArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.Server.Route.Enabled = true
				a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
					WebhookServer: argoproj.WebhookServerSpec{
						Route: argoproj.ArgoCDRouteSpec{
							Enabled: true,
						},
						Host: v.hostname,
					},
				}
			})

			resObjs := []client.Object{argoCD}
			subresObjs := []client.Object{argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme, configv1.Install, routev1.Install)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch)

			assert.NoError(t, createNamespace(r, argoCD.Namespace, ""))

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      testArgoCDName,
					Namespace: testNamespace,
				},
			}

			// Check if it returns nil when hostname is empty
			_, err := r.Reconcile(context.TODO(), req)
			assert.NoError(t, err)

			// second reconciliation after changing the hostname.
			err = r.Client.Get(ctx, req.NamespacedName, argoCD)
			fatalIfError(t, err, "failed to load ArgoCD %q: %s", testArgoCDName+"-server", err)

			argoCD.Spec.Server.Host = v.hostname
			err = r.Client.Update(ctx, argoCD)
			fatalIfError(t, err, "failed to update the ArgoCD: %s", err)

			_, err = r.Reconcile(context.TODO(), req)
			assert.NoError(t, err)

			loaded := &routev1.Route{}
			err = r.Client.Get(ctx, types.NamespacedName{Name: testArgoCDName + "-server", Namespace: testNamespace}, loaded)
			fatalIfError(t, err, "failed to load route %q: %s", testArgoCDName+"-server", err)

			if diff := cmp.Diff(v.expected, loaded.Spec.Host); diff != "" {
				t.Fatalf("failed to reconcile route:\n%s", diff)
			}

			// Check if first label is greater than 20
			labels := strings.Split(loaded.Spec.Host, ".")
			assert.True(t, len(labels[0]) > 20)

		})
	}
}

func TestReconcileRouteTLSConfig(t *testing.T) {
	routeAPIFound = true
	ctx := context.Background()
	logf.SetLogger(ZapLogger(true))

	tt := []struct {
		name            string
		want            routev1.TLSTerminationType
		updateArgoCD    func(cr *argoproj.ArgoCD)
		createResources func(k8sClient client.Client, cr *argoproj.ArgoCD)
	}{
		{
			name: "should set the default termination policy to renencrypt",
			want: routev1.TLSTerminationReencrypt,
			updateArgoCD: func(cr *argoproj.ArgoCD) {
				cr.Spec.Server.Route.Enabled = true
			},
			createResources: func(k8sClient client.Client, cr *argoproj.ArgoCD) {},
		},
		{
			name: "shouldn't overwrite the TLS config if it's already configured",
			want: routev1.TLSTerminationEdge,
			updateArgoCD: func(cr *argoproj.ArgoCD) {
				cr.Spec.Server.Route.Enabled = true
				cr.Spec.Server.Route.TLS = &routev1.TLSConfig{
					Termination: routev1.TLSTerminationEdge,
				}
			},
			createResources: func(k8sClient client.Client, cr *argoproj.ArgoCD) {},
		},
		{
			// We don't want to change the default value to reencrypt if the user has already
			// configured a TLS secret for passthrough (previous default value).
			name: "shouldn't overwrite if the Route was previously configured with passthrough",
			want: routev1.TLSTerminationPassthrough,
			updateArgoCD: func(cr *argoproj.ArgoCD) {
				cr.Spec.Server.Route.Enabled = true
			},
			createResources: func(k8sClient client.Client, cr *argoproj.ArgoCD) {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      common.ArgoCDServerTLSSecretName,
						Namespace: cr.Namespace,
					},
				}
				err := k8sClient.Create(context.Background(), secret)
				assert.NoError(t, err)

				// create a Route with passthrough policy.
				route := newRouteWithSuffix("server", cr)
				route.Spec.TLS = &routev1.TLSConfig{
					Termination: routev1.TLSTerminationPassthrough,
				}
				err = k8sClient.Create(context.Background(), route)
				assert.NoError(t, err)
			},
		},
	}

	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			argoCD := makeArgoCD(test.updateArgoCD)

			resObjs := []client.Object{argoCD}
			subresObjs := []client.Object{argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme, configv1.Install, routev1.Install)
			fakeClient := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			reconciler := makeTestReconciler(fakeClient, sch)

			test.createResources(fakeClient, argoCD)
			req := reconcile.Request{
				NamespacedName: testNamespacedName(testArgoCDName),
			}

			_, err := reconciler.Reconcile(ctx, req)
			assert.Nil(t, err)

			route := &routev1.Route{}
			err = reconciler.Client.Get(ctx, types.NamespacedName{Name: argoCD.Name + "-server", Namespace: argoCD.Namespace}, route)
			assert.Nil(t, err)
			assert.Equal(t, test.want, route.Spec.TLS.Termination)

		})
	}
}

func makeReconciler(t *testing.T, acd *argoproj.ArgoCD, objs ...runtime.Object) *ReconcileArgoCD {
	t.Helper()
	s := scheme.Scheme
	s.AddKnownTypes(argoproj.GroupVersion, acd)
	routev1.Install(s)
	configv1.Install(s)

	clientObjs := []client.Object{}
	for _, obj := range objs {
		clientObj := obj.(client.Object)
		clientObjs = append(clientObjs, clientObj)
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).WithStatusSubresource(clientObjs...).Build()

	return &ReconcileArgoCD{
		Client: cl,
		Scheme: s,
	}
}

func makeArgoCD(opts ...func(*argoproj.ArgoCD)) *argoproj.ArgoCD {
	argoCD := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testArgoCDName,
			Namespace: testNamespace,
		},
		Spec: argoproj.ArgoCDSpec{},
	}
	for _, o := range opts {
		o(argoCD)
	}
	return argoCD
}

func fatalIfError(t *testing.T, err error, format string, a ...interface{}) {
	t.Helper()
	if err != nil {
		t.Fatalf(format, a...)
	}
}

func loadSecret(t *testing.T, c client.Client, name string) *corev1.Secret {
	t.Helper()
	secret := &corev1.Secret{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: testNamespace}, secret)
	fatalIfError(t, err, "failed to load secret %q", name)
	return secret
}

func testNamespacedName(name string) types.NamespacedName {
	return types.NamespacedName{
		Name:      name,
		Namespace: testNamespace,
	}
}
