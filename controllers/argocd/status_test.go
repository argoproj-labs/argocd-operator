package argocd

import (
	"context"
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"

	oappsv1 "github.com/openshift/api/apps/v1"
	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestReconcileArgoCD_reconcileStatusKeycloak_K8s(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCDForKeycloak()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	d := newKeycloakDeployment(a)

	// keycloak not installed
	_ = r.reconcileStatusKeycloak(a)
	assert.Equal(t, "Unknown", a.Status.SSO)

	// keycloak installation started
	err := r.Create(context.TODO(), d)
	assert.NoError(t, err)

	_ = r.reconcileStatusKeycloak(a)
	assert.Equal(t, "Pending", a.Status.SSO)

	// keycloak installation completed
	d.Status.ReadyReplicas = *d.Spec.Replicas
	err = r.Client.Status().Update(context.TODO(), d)
	assert.NoError(t, err)

	_ = r.reconcileStatusKeycloak(a)
	assert.Equal(t, "Running", a.Status.SSO)
}

func TestReconcileArgoCD_reconcileStatusKeycloak_OpenShift(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCDForKeycloak()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	assert.NoError(t, oappsv1.Install(r.Scheme))
	templateAPIFound = true
	deploymentConfigAPIFound = true
	defer removeTemplateAPI()

	dc := getKeycloakDeploymentConfigTemplate(a)
	dc.Name = defaultKeycloakIdentifier

	// keycloak not installed
	_ = r.reconcileStatusKeycloak(a)
	assert.Equal(t, "Unknown", a.Status.SSO)

	// create new client with dc object already present, but with 0 ready replicas to simulate
	// keycloak installation started
	resObjs = append(resObjs, dc)
	subresObjs = append(subresObjs, dc)
	r.Client = makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)

	_ = r.reconcileStatusKeycloak(a)
	assert.Equal(t, "Pending", a.Status.SSO)

	// create new client with dc object already present, with 1 ready replica to simulate
	// keycloak installation completed
	dc.Status.ReadyReplicas = dc.Spec.Replicas
	r.Client = makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)

	_ = r.reconcileStatusKeycloak(a)
	assert.Equal(t, "Running", a.Status.SSO)
}

func TestReconcileArgoCD_reconcileStatusSSO(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name          string
		argoCD        *argoproj.ArgoCD
		wantSSOStatus string
	}{
		{
			name: "both dex and keycloak configured",
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeKeycloak,
					Keycloak: &argoproj.ArgoCDKeycloakSpec{
						Host: "sso.test.example.com",
					},
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}
			}),
			wantSSOStatus: "Failed",
		},
		{
			name: "both dex and keycloak configured, and keycloak host is empty",
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeKeycloak,
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
					Keycloak: &argoproj.ArgoCDKeycloakSpec{},
				}
			}),
			wantSSOStatus: "Failed",
		},
		{
			name: "sso provider dex but no .spec.sso.dex provided",
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
				}
			}),
			wantSSOStatus: "Failed",
		},
		{
			name: "no sso configured",
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = nil
			}),
			wantSSOStatus: "Unknown",
		},
		{
			name: "unsupported sso configured",
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: "Unsupported",
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}
			}),
			wantSSOStatus: "Failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			resObjs := []client.Object{test.argoCD}
			subresObjs := []client.Object{test.argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			assert.NoError(t, createNamespace(r, test.argoCD.Namespace, ""))

			_ = r.reconcileSSO(test.argoCD)

			_ = r.reconcileStatusSSO(test.argoCD)

			assert.Equal(t, test.wantSSOStatus, test.argoCD.Status.SSO)
		})
	}
}

func TestReconcileArgoCD_reconcileStatusHost(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name              string
		routeEnabled      bool
		testRouteAPIFound bool
		ingressEnabled    bool
		expectedNil       bool
		expectedHost      bool
		host              string
	}{
		{
			name:              "",
			routeEnabled:      true,
			testRouteAPIFound: true,
			ingressEnabled:    false,
			expectedNil:       false,
			host:              "argocd",
		},
		{
			name:              "",
			routeEnabled:      false,
			testRouteAPIFound: false,
			ingressEnabled:    true,
			expectedNil:       false,
			host:              "argocd, 12.0.0.5",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			routeAPIFound = test.testRouteAPIFound

			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.Server.Route.Enabled = test.routeEnabled
				a.Spec.Server.Ingress.Enabled = test.ingressEnabled
			})

			route := &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testArgoCDName + "-server",
					Namespace: testNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": testArgoCDName + "-server",
					},
				},
				Spec: routev1.RouteSpec{
					Host: "argocd",
				},
				Status: routev1.RouteStatus{
					Ingress: []routev1.RouteIngress{
						{
							Host: "argocd",
							Conditions: []routev1.RouteIngressCondition{
								{
									Type:   routev1.RouteAdmitted,
									Status: "True",
								},
							},
						},
					},
				},
			}

			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testArgoCDName + "-server",
					Namespace: testNamespace,
				},
				Status: networkingv1.IngressStatus{
					LoadBalancer: networkingv1.IngressLoadBalancerStatus{
						Ingress: []networkingv1.IngressLoadBalancerIngress{
							{
								IP:       "12.0.0.1",
								Hostname: "argocd",
								Ports:    []networkingv1.IngressPortStatus{},
							},
							{
								IP:       "12.0.0.5",
								Hostname: "",
							},
						},
					},
				},
			}

			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme, configv1.Install, routev1.Install)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			if test.routeEnabled {
				err := r.Create(context.TODO(), route)
				assert.NoError(t, err)

			} else if test.ingressEnabled {
				err := r.Create(context.TODO(), ingress)
				assert.NoError(t, err)
				assert.NotEqual(t, "Pending", a.Status.Phase)
			}

			err := r.reconcileStatusHost(a)
			assert.NoError(t, err)

			assert.Equal(t, test.host, a.Status.Host)
		})
	}
}

func TestReconcileArgoCD_reconcileStatusNotificationsController(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, r.reconcileStatusNotifications(a))
	assert.Equal(t, "", a.Status.NotificationsController)

	a.Spec.Notifications.Enabled = true
	assert.NoError(t, r.reconcileNotificationsController(a))
	assert.NoError(t, r.reconcileStatusNotifications(a))
	assert.Equal(t, "Pending", a.Status.NotificationsController)

	a.Spec.Notifications.Enabled = false
	assert.NoError(t, r.deleteNotificationsResources(a))
	assert.NoError(t, r.reconcileStatusNotifications(a))
	assert.Equal(t, "", a.Status.NotificationsController)
}

func TestReconcileArgoCD_reconcileStatusApplicationSetController(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, r.reconcileStatusApplicationSetController(a))
	assert.Equal(t, "Unknown", a.Status.ApplicationSetController)

	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}
	assert.NoError(t, r.reconcileApplicationSetController(a))
	assert.NoError(t, r.reconcileStatusApplicationSetController(a))
	assert.Equal(t, "Pending", a.Status.ApplicationSetController)
}
