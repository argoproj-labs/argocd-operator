package argocd

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
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

func TestRedisNetworkPolicyWithLongName(t *testing.T) {
	// Create ArgoCD with a very long name that will trigger truncation
	longName := "this-is-a-very-long-argocd-instance-name-that-will-exceed-the-kubernetes-name-limit-and-require-truncation"
	a := makeTestArgoCD()
	a.Name = longName

	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	err := r.ReconcileRedisNetworkPolicy(a)
	assert.NoError(t, err)

	// Check if the network policy was created
	np := &networkingv1.NetworkPolicy{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: fmt.Sprintf("%s-%s", a.Name, RedisNetworkPolicy), Namespace: a.Namespace}, np)
	assert.NoError(t, err)

	// Verify that the pod selector uses the truncated name with preserved suffix
	expectedSelector := argoutil.TruncateWithHash(nameWithSuffix("redis", a))
	assert.Equal(t, expectedSelector, np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])

	// Verify the suffix "redis" is preserved in the selector
	assert.Contains(t, np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"], "redis")
}

func TestRedisHANetworkPolicyWithLongName(t *testing.T) {
	// Create ArgoCD with a very long name that will trigger truncation
	longName := "this-is-another-very-long-argocd-instance-name-that-will-exceed-the-kubernetes-name-limit-and-require-truncation"
	a := makeTestArgoCD()
	a.Name = longName

	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	err := r.ReconcileRedisHANetworkPolicy(a)
	assert.NoError(t, err)

	// Check if the network policy was created
	np := &networkingv1.NetworkPolicy{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: fmt.Sprintf("%s-%s", a.Name, RedisHANetworkPolicy), Namespace: a.Namespace}, np)
	assert.NoError(t, err)

	// Verify that the pod selector uses the truncated name with preserved suffix
	expectedSelector := argoutil.TruncateWithHash(nameWithSuffix("redis-ha-haproxy", a))
	assert.Equal(t, expectedSelector, np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])

	// Verify the suffix "redis-ha-haproxy" is preserved in the selector
	assert.Contains(t, np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"], "redis-ha-haproxy")
}

// Test the new truncation functions
func TestTruncateCRName(t *testing.T) {
	// Test with short name (should not be truncated)
	shortName := "short-name"
	truncated := argoutil.TruncateCRName(shortName)
	assert.Equal(t, shortName, truncated)

	// Test with long name (should be truncated)
	longName := "this-is-a-very-long-name-that-exceeds-the-maximum-length-for-kubernetes-resource-names"
	truncated = argoutil.TruncateCRName(longName)
	assert.Len(t, truncated, 37)       // MaxCRNameLength includes the hash suffix
	assert.Contains(t, truncated, "-") // Should contain hash separator
}

func TestGetTruncatedCRName(t *testing.T) {
	// Test with short name
	a := makeTestArgoCD()
	a.Name = "short-name"
	truncated := argoutil.GetTruncatedCRName(a)
	assert.Equal(t, "short-name", truncated)

	// Test with long name
	a.Name = "this-is-a-very-long-name-that-exceeds-the-maximum-length-for-kubernetes-resource-names"
	truncated = argoutil.GetTruncatedCRName(a)
	assert.Len(t, truncated, 37)       // MaxCRNameLength includes the hash suffix
	assert.Contains(t, truncated, "-") // Should contain hash separator
}

func TestNameWithSuffix(t *testing.T) {
	// Test with short name
	a := makeTestArgoCD()
	a.Name = "short-name"
	result := nameWithSuffix("redis", a)
	assert.Equal(t, "short-name-redis", result)

	// Test with long name (should preserve suffix)
	a.Name = "this-is-a-very-long-name-that-exceeds-the-maximum-length-for-kubernetes-resource-names"
	result = nameWithSuffix("redis", a)
	assert.Contains(t, result, "-redis") // Suffix should be preserved
	assert.Len(t, result, 37+6)          // truncated name (37) + "-redis" (6) = 43
}
