package server

import (
	"context"
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	autoscaling "k8s.io/api/autoscaling/v1"
)

func TestServerReconciler_createUpdateAndDeleteHPA(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sr := makeTestServerReconciler(t, ns)

	// configure autoscale in ArgoCD
	sr.Instance.Spec.Server.Autoscale =  argoproj.ArgoCDServerAutoscaleSpec{
		Enabled: true,
	}

	err := sr.reconcileHorizontalPodAutoscaler()
	assert.NoError(t, err)

	// hpa resource should be created with default values
	hpa := &autoscaling.HorizontalPodAutoscaler{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-server", Namespace: "argocd",}, hpa)
	assert.NoError(t, err)
	assert.Equal(t, int32(3), hpa.Spec.MaxReplicas)

	// modify hpa resource in ArgoCD
	sr.Instance.Spec.Server.Autoscale =  argoproj.ArgoCDServerAutoscaleSpec{
		Enabled: true,
		HPA: &autoscaling.HorizontalPodAutoscalerSpec{
			MaxReplicas: int32(2),
		},
	}

	err = sr.reconcileHorizontalPodAutoscaler()
	assert.NoError(t, err)

	// hpa resource should be updated
	hpa = &autoscaling.HorizontalPodAutoscaler{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-server", Namespace: "argocd",}, hpa)
	assert.NoError(t, err)
	assert.Equal(t, int32(2), hpa.Spec.MaxReplicas)

	// disable autosacle  in ArgoCD
	sr.Instance.Spec.Server.Autoscale.Enabled = false
	err = sr.reconcileHorizontalPodAutoscaler()
	assert.NoError(t, err)

	// hpa resource should be deleted
	hpa = &autoscaling.HorizontalPodAutoscaler{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-server", Namespace: "argocd",}, hpa)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}