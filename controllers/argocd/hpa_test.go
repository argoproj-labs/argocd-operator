package argocd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	autoscaling "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

var (
	min     int32 = 5
	max     int32 = 8
	cpuUtil int32 = 45
)

func TestReconcileHPA(t *testing.T) {

	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	existingHPA := newHorizontalPodAutoscalerWithSuffix("server", a)

	defaultHPASpec := autoscaling.HorizontalPodAutoscalerSpec{
		MaxReplicas:                    maxReplicas,
		MinReplicas:                    &minReplicas,
		TargetCPUUtilizationPercentage: &tcup,
		ScaleTargetRef: autoscaling.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       util.NameWithSuffix(a.Name, "server"),
		},
	}

	updatedHPASpec := autoscaling.HorizontalPodAutoscalerSpec{
		MaxReplicas:                    max,
		MinReplicas:                    &min,
		TargetCPUUtilizationPercentage: &cpuUtil,
		ScaleTargetRef: autoscaling.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       util.NameWithSuffix(a.Name, "server"),
		},
	}

	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-server",
		Namespace: testNamespace,
	}, existingHPA)
	assert.True(t, errors.IsNotFound(err))

	a.Spec.Server.Autoscale.Enabled = true

	err = r.reconcileServerHPA(a)
	assert.NoError(t, err)

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-server",
		Namespace: testNamespace,
	}, existingHPA)
	assert.NoError(t, err)
	assert.Equal(t, defaultHPASpec, existingHPA.Spec)

	a.Spec.Server.Autoscale.HPA = &updatedHPASpec

	err = r.reconcileServerHPA(a)
	assert.NoError(t, err)

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-server",
		Namespace: testNamespace,
	}, existingHPA)
	assert.NoError(t, err)
	assert.Equal(t, updatedHPASpec, existingHPA.Spec)

	a.Spec.Server.Autoscale.Enabled = false

	err = r.reconcileServerHPA(a)
	assert.NoError(t, err)

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-server",
		Namespace: testNamespace,
	}, existingHPA)
	assert.True(t, errors.IsNotFound(err))

}
