package redis

import (
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_triggerHARollout(t *testing.T) {
	testArgoCD := test.MakeTestArgoCD(nil,
		func(ac *argoproj.ArgoCD) {
			ac.Spec.HA.Enabled = true
		},
	)

	hacm := test.MakeTestConfigMap(nil,
		func(cm *corev1.ConfigMap) {
			cm.Name = "argocd-redis-ha-configmap"
		},
	)

	hahealthcm := test.MakeTestConfigMap(nil,
		func(cm *corev1.ConfigMap) {
			cm.Name = "argocd-redis-ha-health-configmap"
		},
	)

	dep := test.MakeTestDeployment(nil,
		func(d *appsv1.Deployment) {
			d.Name = "test-argocd-redis-ha-haproxy"
		},
	)

	ss := test.MakeTestStatefulSet(nil,
		func(ss *appsv1.StatefulSet) {
			ss.Name = "test-argocd-redis-ha-server"
		},
	)

	resources := []client.Object{hacm, hahealthcm, dep, ss}

	reconciler := makeTestRedisReconciler(
		testArgoCD,
		resources...,
	)

	reconciler.varSetter()

	err := reconciler.TriggerRollout(test.TestKey)
	assert.NoError(t, err)

	cmList := []client.Object{hacm, hahealthcm}
	for _, obj := range cmList {
		_, err := resource.GetObject(obj.GetName(), test.TestNamespace, obj, reconciler.Client)
		assert.NoError(t, err)
	}

	dep, err = workloads.GetDeployment(dep.Name, test.TestNamespace, reconciler.Client)
	assert.NoError(t, err)
	assert.NotEqual(t, "", dep.Spec.Template.Labels[test.TestKey])

	_, err = workloads.GetStatefulSet(ss.Name, test.TestNamespace, reconciler.Client)
	assert.True(t, apierrors.IsNotFound(err))
}
