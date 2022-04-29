package argocd

import (
	"context"
	"testing"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestReconcileArgoCD_reconcileStatusSSOConfig_multi_sso_configured(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCDForKeycloakWithDex()

	templateAPIFound = true
	r := makeTestReconciler(t, a)
	assert.NoError(t, r.reconcileStatusSSOConfig(a))
	assert.Equal(t, a.Status.SSOConfig, "Failed")
}
func TestReconcileArgoCD_reconcileStatusSSOConfig_only_keycloak_configured(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCDForKeycloak()

	templateAPIFound = true
	r := makeTestReconciler(t, a)
	assert.NoError(t, r.reconcileStatusSSOConfig(a))
	assert.Equal(t, a.Status.SSOConfig, "Success")
}
func TestReconcileArgoCD_reconcileStatusSSOConfig_only_dex_configured(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCDWithResources()

	templateAPIFound = true
	r := makeTestReconciler(t, a)
	assert.NoError(t, r.reconcileStatusSSOConfig(a))
	assert.Equal(t, a.Status.SSOConfig, "Success")
}
func TestReconcileArgoCD_reconcileStatusSSOConfig_no_sso_configured(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	templateAPIFound = true
	r := makeTestReconciler(t, a)
	assert.NoError(t, r.reconcileStatusSSOConfig(a))
	assert.Equal(t, a.Status.SSOConfig, "Unknown")
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
		phase             string
	}{
		{
			name:              "",
			routeEnabled:      true,
			testRouteAPIFound: true,
			ingressEnabled:    false,
			expectedNil:       false,
			host:              "argocd",
			phase:             "Available",
		},
		{
			name:              "",
			routeEnabled:      false,
			testRouteAPIFound: false,
			ingressEnabled:    true,
			expectedNil:       false,
			host:              "argocd, 12.0.0.5",
			phase:             "Available",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			routeAPIFound = test.testRouteAPIFound

			a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
				a.Spec.Server.Route.Enabled = test.routeEnabled
				a.Spec.Server.Ingress.Enabled = test.ingressEnabled
			})

			objs := []runtime.Object{
				a,
			}

			route := &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testArgoCDName + "-server",
					Namespace: testNamespace,
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
					LoadBalancer: v1.LoadBalancerStatus{
						Ingress: []v1.LoadBalancerIngress{
							{
								IP:       "12.0.0.1",
								Hostname: "argocd",
								Ports:    []v1.PortStatus{},
							},
							{
								IP:       "12.0.0.5",
								Hostname: "",
							},
						},
					},
				},
			}

			r := makeReconciler(t, a, objs...)
			if test.routeEnabled {
				err := r.Client.Create(context.TODO(), route)
				assert.NoError(t, err)

			} else if test.ingressEnabled {
				err := r.Client.Create(context.TODO(), ingress)
				assert.NoError(t, err)
				assert.NotEqual(t, "Pending", a.Status.Phase)
			}

			err := r.reconcileStatusHost(a)
			assert.NoError(t, err)

			assert.Equal(t, test.host, a.Status.Host)
			assert.Equal(t, test.phase, a.Status.Phase)
		})
	}
}

func TestReconcileArgoCD_reconcileStatusNotificationsController(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	assert.NoError(t, r.reconcileStatusNotifications(a))
	assert.Equal(t, "", a.Status.NotificationsController)

	a.Spec.Notifications.Enabled = true
	assert.NoError(t, r.reconcileNotificationsController(a))
	assert.NoError(t, r.reconcileStatusNotifications(a))
	assert.Equal(t, "Running", a.Status.NotificationsController)

	a.Spec.Notifications.Enabled = false
	assert.NoError(t, r.deleteNotificationsResources(a))
	assert.NoError(t, r.reconcileStatusNotifications(a))
	assert.Equal(t, "", a.Status.NotificationsController)
}
