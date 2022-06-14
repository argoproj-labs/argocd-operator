package argocd

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestReconcileApplicationSetService_Ingress(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	a.Spec.ApplicationSet.ApplicationSetControllerServerSpec.Ingress.Enabled = true
	r := makeTestReconciler(t, a)

	ingress := newIngressWithSuffix(common.ApplicationSetServiceNameSuffix, a)

	assert.NoError(t, r.reconcileApplicationSetControllerIngress(a))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: ingress.Namespace, Name: ingress.Name}, ingress))
}
