package argocd

import (
	"context"
	"fmt"
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReconcileNetworkPolicies(t *testing.T) {

	a := makeTestArgoCD()
	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	err := r.ReconcileRedisNetworkPolicy(a)
	assert.NoError(t, err)

	err = r.ReconcileRedisHANetworkPolicy(a)
	assert.NoError(t, err)
}

func TestRedisNetworkPolicy(t *testing.T) {
	a := makeTestArgoCD()
	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	err := r.ReconcileRedisNetworkPolicy(a)
	assert.NoError(t, err)

	// Check if the network policy was created
	np := &networkingv1.NetworkPolicy{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: fmt.Sprintf("%s-%s", a.Name, RedisNetworkPolicy), Namespace: a.Namespace}, np)
	assert.NoError(t, err)

	// Check if the network policy has the correct pod selector
	assert.Equal(t, fmt.Sprintf("%s-%s", a.Name, "redis"), np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])

	// Check if the network policy has the correct policy types
	assert.Equal(t, networkingv1.PolicyTypeIngress, np.Spec.PolicyTypes[0])

	// Check if the network policy has the correct ingress rules
	assert.Equal(t, 3, len(np.Spec.Ingress[0].From))
	assert.Equal(t, "argocd-application-controller", np.Spec.Ingress[0].From[0].PodSelector.MatchLabels["app.kubernetes.io/name"])
	assert.Equal(t, "argocd-repo-server", np.Spec.Ingress[0].From[1].PodSelector.MatchLabels["app.kubernetes.io/name"])
	assert.Equal(t, "argocd-server", np.Spec.Ingress[0].From[2].PodSelector.MatchLabels["app.kubernetes.io/name"])
	assert.Equal(t, 1, len(np.Spec.Ingress[0].Ports))
	assert.Equal(t, intstr.FromInt(6379), *np.Spec.Ingress[0].Ports[0].Port)
}

func TestRedisHANetworkPolicy(t *testing.T) {
	a := makeTestArgoCD()
	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	err := r.ReconcileRedisHANetworkPolicy(a)
	assert.NoError(t, err)

	// Check if the network policy was created
	np := &networkingv1.NetworkPolicy{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: fmt.Sprintf("%s-%s", a.Name, RedisHANetworkPolicy), Namespace: a.Namespace}, np)
	assert.NoError(t, err)

	// Check if the network policy has the correct pod selector
	assert.Equal(t, fmt.Sprintf("%s-%s", a.Name, "redis-ha-haproxy"), np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])

	// Check if the network policy has the correct policy types
	assert.Equal(t, networkingv1.PolicyTypeIngress, np.Spec.PolicyTypes[0])

	// Check if the network policy has the correct ingress rules
	assert.Equal(t, 3, len(np.Spec.Ingress[0].From))
	assert.Equal(t, "argocd-application-controller", np.Spec.Ingress[0].From[0].PodSelector.MatchLabels["app.kubernetes.io/name"])
	assert.Equal(t, "argocd-repo-server", np.Spec.Ingress[0].From[1].PodSelector.MatchLabels["app.kubernetes.io/name"])
	assert.Equal(t, "argocd-server", np.Spec.Ingress[0].From[2].PodSelector.MatchLabels["app.kubernetes.io/name"])
	assert.Equal(t, 2, len(np.Spec.Ingress[0].Ports))
	assert.Equal(t, intstr.FromInt(6379), *np.Spec.Ingress[0].Ports[0].Port)
	assert.Equal(t, intstr.FromInt(26379), *np.Spec.Ingress[0].Ports[1].Port)
}
