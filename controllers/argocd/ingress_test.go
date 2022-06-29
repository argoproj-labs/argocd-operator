package argocd

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestReconcileApplicationSetService_Ingress(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	obj := v1alpha1.ArgoCDApplicationSet{
		WebhookServerSpec: v1alpha1.WebhookServerSpec{
			Ingress: v1alpha1.ArgoCDIngressSpec{
				Enabled: true,
			},
		},
	}
	a.Spec.ApplicationSet = &obj
	r := makeTestReconciler(t, a)
	ingress := newIngressWithSuffix(common.ApplicationSetServiceNameSuffix, a)
	assert.NoError(t, r.reconcileApplicationSetControllerIngress(a))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: ingress.Namespace, Name: ingress.Name}, ingress))
}
