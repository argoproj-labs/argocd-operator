package argocd

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"

	oappsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestArgoCDReconciler_reconcileStatusKeycloak_K8s(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCDForKeycloak()
	r := makeTestReconciler(t, a)
	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	d := newKeycloakDeployment(a)

	// keycloak not installed
	_ = r.reconcileStatusKeycloak(a)
	assert.Equal(t, "Unknown", a.Status.SSO)

	// keycloak installation started
	r.Client.Create(context.TODO(), d)

	_ = r.reconcileStatusKeycloak(a)
	assert.Equal(t, "Pending", a.Status.SSO)

	// keycloak installation completed
	d.Status.ReadyReplicas = *d.Spec.Replicas
	r.Client.Status().Update(context.TODO(), d)

	_ = r.reconcileStatusKeycloak(a)
	assert.Equal(t, "Running", a.Status.SSO)
}

func TestArgoCDReconciler_reconcileStatusKeycloak_OpenShift(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCDForKeycloak()
	r := makeTestReconciler(t, a)
	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	assert.NoError(t, oappsv1.AddToScheme(r.Scheme))
	workloads.SetTemplateAPIFound(true)
	defer removeTemplateAPI()

	dc := getKeycloakDeploymentConfigTemplate(a)
	dc.ObjectMeta.Name = defaultKeycloakIdentifier

	// keycloak not installed
	_ = r.reconcileStatusKeycloak(a)
	assert.Equal(t, "Unknown", a.Status.SSO)

	// keycloak installation started
	r.Client.Create(context.TODO(), dc)

	_ = r.reconcileStatusKeycloak(a)
	assert.Equal(t, "Pending", a.Status.SSO)

	// keycloak installation completed
	dc.Status.ReadyReplicas = dc.Spec.Replicas
	r.Client.Status().Update(context.TODO(), dc)

	_ = r.reconcileStatusKeycloak(a)
	assert.Equal(t, "Running", a.Status.SSO)
}

func TestArgoCDReconciler_reconcileStatusSSO(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name          string
		argoCD        *argoprojv1alpha1.ArgoCD
		wantSSOStatus string
	}{
		{
			name: "both dex and keycloak configured",
			argoCD: makeTestArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoprojv1alpha1.SSOProviderTypeKeycloak,
					Dex: &v1alpha1.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}
			}),
			wantSSOStatus: "Failed",
		},
		{
			name: "sso provider dex but no .spec.sso.dex provided",
			argoCD: makeTestArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoprojv1alpha1.SSOProviderTypeDex,
				}
			}),
			wantSSOStatus: "Failed",
		},
		{
			name: "no sso configured",
			argoCD: makeTestArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = nil
			}),
			wantSSOStatus: "Unknown",
		},
		{
			name: "unsupported sso configured",
			argoCD: makeTestArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: "Unsupported",
					Dex: &v1alpha1.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}
			}),
			wantSSOStatus: "Failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			r := makeTestReconciler(t, test.argoCD)
			assert.NoError(t, createNamespace(r, test.argoCD.Namespace, ""))

			r.reconcileSSO(test.argoCD)

			r.reconcileStatusSSO(test.argoCD)

			assert.Equal(t, test.wantSSOStatus, test.argoCD.Status.SSO)
		})
	}
}

func TestArgoCDReconciler_reconcileStatusHost(t *testing.T) {
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

func TestArgoCDReconciler_reconcileStatusNotificationsController(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	assert.NoError(t, r.reconcileStatusNotifications(a))
	assert.Equal(t, "", a.Status.NotificationsController)

	a.Spec.Notifications.Enabled = true
	assert.NoError(t, r.NotificationsController.Reconcile())
	assert.NoError(t, r.reconcileStatusNotifications(a))
	assert.Equal(t, "Pending", a.Status.NotificationsController)

	a.Spec.Notifications.Enabled = false
	assert.NoError(t, r.NotificationsController.DeleteResources())
	assert.NoError(t, r.reconcileStatusNotifications(a))
	assert.Equal(t, "", a.Status.NotificationsController)
}

func TestArgoCDReconciler_reconcileStatusApplicationSetController(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	assert.NoError(t, r.reconcileStatusApplicationSetController(a))
	assert.Equal(t, "Unknown", a.Status.ApplicationSetController)

	a.Spec.ApplicationSet = &v1alpha1.ArgoCDApplicationSet{}
	assert.NoError(t, r.reconcileApplicationSetController(a))
	assert.NoError(t, r.reconcileStatusApplicationSetController(a))
	assert.Equal(t, "Pending", a.Status.ApplicationSetController)
}
