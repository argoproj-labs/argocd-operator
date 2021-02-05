package argocd

import (
	"context"
	"fmt"
	"testing"

	"gotest.tools/assert"

	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	testRedisImage        = "redis"
	testRedisImageVersion = "test"
)

func TestReconcileArgoCD_reconcileRedisStatefulSet_HA_disabled(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))

	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)
	s := newStatefulSetWithSuffix("redis-ha-server", "redis", a)

	assert.NilError(t, r.reconcileRedisStatefulSet(a))
	// resource Creation should fail as HA was disabled
	assert.ErrorContains(t, r.client.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s), "not found")
}

func TestReconcileArgoCD_reconcileRedisStatefulSet_HA_enabled(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))

	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)
	s := newStatefulSetWithSuffix("redis-ha-server", "redis", a)

	a.Spec.HA.Enabled = true
	// test resource is Created when HA is enabled
	assert.NilError(t, r.reconcileRedisStatefulSet(a))
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s))

	// test resource is Updated on reconciliation
	a.Spec.Redis.Image = testRedisImage
	a.Spec.Redis.Version = testRedisImageVersion
	assert.NilError(t, r.reconcileRedisStatefulSet(a))
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s))
	assert.Equal(t, s.Spec.Template.Spec.Containers[0].Image, fmt.Sprintf("%s:%s", testRedisImage, testRedisImageVersion))

	// test resource is Deleted, when HA is disabled
	a.Spec.HA.Enabled = false
	assert.NilError(t, r.reconcileRedisStatefulSet(a))
	assert.ErrorContains(t, r.client.Get(context.TODO(), types.NamespacedName{Name: s.Name, Namespace: a.Namespace}, s), "not found")
}
