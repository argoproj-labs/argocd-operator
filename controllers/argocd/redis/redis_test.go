package redis

import (
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func makeTestRedisReconciler(cr *argoproj.ArgoCD, objs ...client.Object) *RedisReconciler {
	schemeOpt := func(s *runtime.Scheme) {
		argoproj.AddToScheme(s)
	}
	sch := test.MakeTestReconcilerScheme(schemeOpt)

	client := test.MakeTestReconcilerClient(sch, objs, []client.Object{cr}, []runtime.Object{cr})

	return &RedisReconciler{
		Client:   client,
		Scheme:   sch,
		Instance: cr,
		Logger:   util.NewLogger(common.RedisComponent),
	}
}

func Test_Reconcile(t *testing.T) {
	tests := []struct {
		name              string
		instance          *argoproj.ArgoCD
		expectedResources []client.Object
	}{
		{
			name: "non HA mode",
			instance: test.MakeTestArgoCD(nil,
				func(cr *argoproj.ArgoCD) {
					cr.Spec.HA.Enabled = false
				},
			),
			expectedResources: []client.Object{
				test.MakeTestRole(nil,
					func(r *rbacv1.Role) {
						r.Name = "test-argocd-redis"
					},
				),
				test.MakeTestServiceAccount(
					func(sa *corev1.ServiceAccount) {
						sa.Name = "test-argocd-redis"
					},
				),
				test.MakeTestRoleBinding(nil,
					func(rb *rbacv1.RoleBinding) {
						rb.Name = "test-argocd-redis"
					},
				),
				test.MakeTestService(nil,
					func(s *corev1.Service) {
						s.Name = "test-argocd-redis"
					},
				),
				test.MakeTestDeployment(nil,
					func(d *appsv1.Deployment) {
						d.Name = "test-argocd-redis"
					},
				),
			},
		},
		{
			name: "HA mode",
			instance: test.MakeTestArgoCD(nil,
				func(cr *argoproj.ArgoCD) {
					cr.Spec.HA.Enabled = true
				},
			),
			expectedResources: []client.Object{
				test.MakeTestRole(nil,
					func(r *rbacv1.Role) {
						r.Name = "test-argocd-redis-ha"
					},
				),
				test.MakeTestServiceAccount(
					func(sa *corev1.ServiceAccount) {
						sa.Name = "test-argocd-redis"
					},
				),
				test.MakeTestRoleBinding(nil,
					func(rb *rbacv1.RoleBinding) {
						rb.Name = "test-argocd-redis"
					},
				),
				test.MakeTestConfigMap(nil,
					func(cm *corev1.ConfigMap) {
						cm.Name = "argocd-redis-ha-configmap"
					},
				),
				test.MakeTestConfigMap(nil,
					func(cm *corev1.ConfigMap) {
						cm.Name = "argocd-redis-ha-health-configmap"
					},
				),
				test.MakeTestService(nil,
					func(s *corev1.Service) {
						s.Name = "test-argocd-redis-ha"
					},
				),
				test.MakeTestService(nil,
					func(s *corev1.Service) {
						s.Name = "test-argocd-redis-ha-haproxy"
					},
				),
				test.MakeTestService(nil,
					func(s *corev1.Service) {
						s.Name = "test-argocd-redis-ha-announce-0"
					},
				),
				test.MakeTestService(nil,
					func(s *corev1.Service) {
						s.Name = "test-argocd-redis-ha-announce-1"
					},
				),
				test.MakeTestService(nil,
					func(s *corev1.Service) {
						s.Name = "test-argocd-redis-ha-announce-2"
					},
				),
				test.MakeTestDeployment(nil,
					func(d *appsv1.Deployment) {
						d.Name = "test-argocd-redis-ha-haproxy"
					},
				),
				test.MakeTestStatefulSet(nil,
					func(ss *appsv1.StatefulSet) {
						ss.Name = "test-argocd-redis-ha-server"
					},
				),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := makeTestRedisReconciler(
				tt.instance,
			)

			reconciler.varSetter()

			err := reconciler.Reconcile()
			assert.NoError(t, err)

			for _, obj := range tt.expectedResources {
				_, err := resource.GetObject(obj.GetName(), test.TestNamespace, obj, reconciler.Client)
				assert.NoError(t, err)
			}
		})
	}

}

func Test_deleteResources(t *testing.T) {
	tests := []struct {
		name      string
		instance  *argoproj.ArgoCD
		resources []client.Object
	}{
		{
			name: "non HA mode",
			instance: test.MakeTestArgoCD(nil,
				func(cr *argoproj.ArgoCD) {
					cr.Spec.HA.Enabled = false
				},
			),
			resources: []client.Object{
				test.MakeTestRole(nil,
					func(r *rbacv1.Role) {
						r.Name = "test-argocd-redis"
					},
				),
				test.MakeTestServiceAccount(
					func(sa *corev1.ServiceAccount) {
						sa.Name = "test-argocd-redis"
					},
				),
				test.MakeTestRoleBinding(nil,
					func(rb *rbacv1.RoleBinding) {
						rb.Name = "test-argocd-redis"
					},
				),
				test.MakeTestService(nil,
					func(s *corev1.Service) {
						s.Name = "test-argocd-redis"
					},
				),
				test.MakeTestDeployment(nil,
					func(d *appsv1.Deployment) {
						d.Name = "test-argocd-redis"
					},
				),
			},
		},
		{
			name: "HA mode",
			instance: test.MakeTestArgoCD(nil,
				func(cr *argoproj.ArgoCD) {
					cr.Spec.HA.Enabled = true
				},
			),
			resources: []client.Object{
				test.MakeTestRole(nil,
					func(r *rbacv1.Role) {
						r.Name = "test-argocd-redis-ha"
					},
				),
				test.MakeTestServiceAccount(
					func(sa *corev1.ServiceAccount) {
						sa.Name = "test-argocd-redis"
					},
				),
				test.MakeTestRoleBinding(nil,
					func(rb *rbacv1.RoleBinding) {
						rb.Name = "test-argocd-redis"
					},
				),
				test.MakeTestConfigMap(nil,
					func(cm *corev1.ConfigMap) {
						cm.Name = "argocd-redis-ha-configmap"
					},
				),
				test.MakeTestConfigMap(nil,
					func(cm *corev1.ConfigMap) {
						cm.Name = "argocd-redis-ha-health-configmap"
					},
				),
				test.MakeTestService(nil,
					func(s *corev1.Service) {
						s.Name = "test-argocd-redis-ha"
					},
				),
				test.MakeTestService(nil,
					func(s *corev1.Service) {
						s.Name = "test-argocd-redis-ha-haproxy"
					},
				),
				test.MakeTestService(nil,
					func(s *corev1.Service) {
						s.Name = "test-argocd-redis-ha-announce-0"
					},
				),
				test.MakeTestService(nil,
					func(s *corev1.Service) {
						s.Name = "test-argocd-redis-ha-announce-1"
					},
				),
				test.MakeTestService(nil,
					func(s *corev1.Service) {
						s.Name = "test-argocd-redis-ha-announce-2"
					},
				),
				test.MakeTestDeployment(nil,
					func(d *appsv1.Deployment) {
						d.Name = "test-argocd-redis-ha-haproxy"
					},
				),
				test.MakeTestStatefulSet(nil,
					func(ss *appsv1.StatefulSet) {
						ss.Name = "test-argocd-redis-ha-server"
					},
				),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := makeTestRedisReconciler(
				tt.instance,
			)

			reconciler.varSetter()

			err := reconciler.DeleteResources()
			assert.NoError(t, err)

			for _, obj := range tt.resources {
				_, err := resource.GetObject(obj.GetName(), test.TestNamespace, obj, reconciler.Client)
				assert.True(t, apierrors.IsNotFound(err))
			}
		})
	}
}
