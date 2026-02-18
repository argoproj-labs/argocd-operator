package argocd

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

func TestReconcileNetworkPolicies(t *testing.T) {

	a := makeTestArgoCD()
	a.Spec.Notifications.Enabled = true
	a.Spec.SSO = &argoproj.ArgoCDSSOSpec{
		Provider: argoproj.SSOProviderTypeDex,
		Dex:      &argoproj.ArgoCDDexSpec{},
	}
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
		Enabled: boolPtr(true),
	}
	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	err := r.ReconcileNetworkPolicies(a)
	assert.NoError(t, err)
}

func TestReconcileNetworkPolicies_DisabledDeletesExisting(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.Notifications.Enabled = true
	a.Spec.SSO = &argoproj.ArgoCDSSOSpec{
		Provider: argoproj.SSOProviderTypeDex,
		Dex:      &argoproj.ArgoCDDexSpec{},
	}
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
		Enabled: boolPtr(true),
	}
	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	// Create all policies
	err := r.ReconcileNetworkPolicies(a)
	assert.NoError(t, err)

	// Disable and ensure policies are deleted
	a.Spec.NetworkPolicy.Enabled = boolPtr(false)
	err = r.ReconcileNetworkPolicies(a)
	assert.NoError(t, err)

	nps := []string{
		fmt.Sprintf("%s-%s", a.Name, ArgoCDNotificationsControllerNetworkPolicy),
		fmt.Sprintf("%s-%s", a.Name, ArgoCDDexServerNetworkPolicy),
		fmt.Sprintf("%s-%s", a.Name, ArgoCDApplicationSetControllerNetworkPolicy),
		fmt.Sprintf("%s-%s", a.Name, ArgoCDServerNetworkPolicy),
		fmt.Sprintf("%s-%s", a.Name, ArgoCDApplicationControllerNetworkPolicy),
		fmt.Sprintf("%s-%s", a.Name, ArgoCDRepoServerNetworkPolicy),
	}
	for _, name := range nps {
		np := &networkingv1.NetworkPolicy{}
		err = r.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: a.Namespace}, np)
		assert.Error(t, err)
	}
}

func TestReconcileNetworkPolicies_RecreatesDeletedNetworkPolicy(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.Notifications.Enabled = true
	a.Spec.SSO = &argoproj.ArgoCDSSOSpec{
		Provider: argoproj.SSOProviderTypeDex,
		Dex:      &argoproj.ArgoCDDexSpec{},
	}
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
		Enabled: boolPtr(true),
	}

	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	// Create policies
	err := r.ReconcileNetworkPolicies(a)
	assert.NoError(t, err)

	// Delete one policy manually (simulate kubectl delete)
	name := fmt.Sprintf("%s-%s", a.Name, ArgoCDRepoServerNetworkPolicy)
	toDelete := &networkingv1.NetworkPolicy{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: a.Namespace}, toDelete)
	assert.NoError(t, err)
	assert.NoError(t, r.Delete(context.TODO(), toDelete))

	// Ensure it's gone
	err = r.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: a.Namespace}, &networkingv1.NetworkPolicy{})
	assert.Error(t, err)

	// Next reconcile should recreate it
	err = r.ReconcileNetworkPolicies(a)
	assert.NoError(t, err)
	err = r.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: a.Namespace}, &networkingv1.NetworkPolicy{})
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
	expectedSelector := nameWithSuffix("redis", a)
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
	expectedSelector := nameWithSuffix("redis-ha-haproxy", a)
	assert.Equal(t, expectedSelector, np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])

	// Verify the suffix "redis-ha-haproxy" is preserved in the selector
	assert.Contains(t, np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"], "redis-ha-haproxy")
}

func TestNotificationsControllerNetworkPolicy(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.Notifications.Enabled = true
	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	err := r.ReconcileNotificationsControllerNetworkPolicy(a)
	assert.NoError(t, err)

	np := &networkingv1.NetworkPolicy{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: fmt.Sprintf("%s-%s", a.Name, ArgoCDNotificationsControllerNetworkPolicy), Namespace: a.Namespace}, np)
	assert.NoError(t, err)

	// Check if the network policy has the correct pod selector
	assert.Equal(t, fmt.Sprintf("%s-%s", a.Name, "notifications-controller"), np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])

	// Check if the network policy has the correct policy types
	assert.Equal(t, networkingv1.PolicyTypeIngress, np.Spec.PolicyTypes[0])

	// Check if the network policy has the correct ingress rules
	assert.Equal(t, 1, len(np.Spec.Ingress))
	assert.Equal(t, 1, len(np.Spec.Ingress[0].From))
	assert.NotNil(t, np.Spec.Ingress[0].From[0].NamespaceSelector)
	assert.Equal(t, metav1.LabelSelector{}, *np.Spec.Ingress[0].From[0].NamespaceSelector)
	assert.Equal(t, 1, len(np.Spec.Ingress[0].Ports))
	assert.Equal(t, intstr.FromInt(9001), *np.Spec.Ingress[0].Ports[0].Port)
}

func TestNotificationsControllerNetworkPolicyWithLongName(t *testing.T) {
	// Create ArgoCD with a very long name that will trigger truncation
	longName := "this-is-a-very-long-argocd-instance-name-that-will-exceed-the-kubernetes-name-limit-and-require-truncation"
	a := makeTestArgoCD()
	a.Name = longName
	a.Spec.Notifications.Enabled = true

	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	err := r.ReconcileNotificationsControllerNetworkPolicy(a)
	assert.NoError(t, err)

	np := &networkingv1.NetworkPolicy{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: fmt.Sprintf("%s-%s", a.Name, ArgoCDNotificationsControllerNetworkPolicy), Namespace: a.Namespace}, np)
	assert.NoError(t, err)

	// Verify that the pod selector uses the truncated name with preserved suffix
	expectedSelector := nameWithSuffix("notifications-controller", a)
	assert.Equal(t, expectedSelector, np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])
	assert.Contains(t, np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"], "notifications-controller")
}

func TestDexServerNetworkPolicy(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.SSO = &argoproj.ArgoCDSSOSpec{
		Provider: argoproj.SSOProviderTypeDex,
		Dex:      &argoproj.ArgoCDDexSpec{},
	}
	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	err := r.ReconcileDexServerNetworkPolicy(a)
	assert.NoError(t, err)

	np := &networkingv1.NetworkPolicy{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: fmt.Sprintf("%s-%s", a.Name, ArgoCDDexServerNetworkPolicy), Namespace: a.Namespace}, np)
	assert.NoError(t, err)

	// podSelector: dex-server
	assert.Equal(t, fmt.Sprintf("%s-%s", a.Name, "dex-server"), np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])

	// ingress: (1) from argocd-server on 5556/5557, (2) from any namespace on 5558
	assert.Equal(t, networkingv1.PolicyTypeIngress, np.Spec.PolicyTypes[0])
	assert.Equal(t, 2, len(np.Spec.Ingress))

	// Rule 1: from argocd-server
	assert.Equal(t, 1, len(np.Spec.Ingress[0].From))
	assert.NotNil(t, np.Spec.Ingress[0].From[0].PodSelector)
	assert.Equal(t, fmt.Sprintf("%s-%s", a.Name, "server"), np.Spec.Ingress[0].From[0].PodSelector.MatchLabels["app.kubernetes.io/name"])
	assert.Equal(t, 2, len(np.Spec.Ingress[0].Ports))
	assert.Equal(t, intstr.FromInt(common.ArgoCDDefaultDexHTTPPort), *np.Spec.Ingress[0].Ports[0].Port)
	assert.Equal(t, intstr.FromInt(common.ArgoCDDefaultDexGRPCPort), *np.Spec.Ingress[0].Ports[1].Port)

	// Rule 2: from any namespace (metrics)
	assert.Equal(t, 1, len(np.Spec.Ingress[1].From))
	assert.NotNil(t, np.Spec.Ingress[1].From[0].NamespaceSelector)
	assert.Equal(t, metav1.LabelSelector{}, *np.Spec.Ingress[1].From[0].NamespaceSelector)
	assert.Equal(t, 1, len(np.Spec.Ingress[1].Ports))
	assert.Equal(t, intstr.FromInt(common.ArgoCDDefaultDexMetricsPort), *np.Spec.Ingress[1].Ports[0].Port)
}

func TestDexServerNetworkPolicyDisabledDeletesExisting(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.SSO = &argoproj.ArgoCDSSOSpec{
		Provider: argoproj.SSOProviderTypeDex,
		Dex:      &argoproj.ArgoCDDexSpec{},
	}
	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	// create NP
	err := r.ReconcileDexServerNetworkPolicy(a)
	assert.NoError(t, err)

	// disable dex and ensure NP is deleted
	a.Spec.SSO = nil
	err = r.ReconcileDexServerNetworkPolicy(a)
	assert.NoError(t, err)

	np := &networkingv1.NetworkPolicy{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: fmt.Sprintf("%s-%s", a.Name, ArgoCDDexServerNetworkPolicy), Namespace: a.Namespace}, np)
	assert.Error(t, err)
}

func TestDexServerNetworkPolicyWithLongName(t *testing.T) {
	longName := "this-is-a-very-long-argocd-instance-name-that-will-exceed-the-kubernetes-name-limit-and-require-truncation"
	a := makeTestArgoCD()
	a.Name = longName
	a.Spec.SSO = &argoproj.ArgoCDSSOSpec{
		Provider: argoproj.SSOProviderTypeDex,
		Dex:      &argoproj.ArgoCDDexSpec{},
	}
	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	err := r.ReconcileDexServerNetworkPolicy(a)
	assert.NoError(t, err)

	np := &networkingv1.NetworkPolicy{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: fmt.Sprintf("%s-%s", a.Name, ArgoCDDexServerNetworkPolicy), Namespace: a.Namespace}, np)
	assert.NoError(t, err)

	expectedSelector := nameWithSuffix("dex-server", a)
	assert.Equal(t, expectedSelector, np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])
	assert.Contains(t, np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"], "dex-server")
}

func TestApplicationSetControllerNetworkPolicy(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
		Enabled: boolPtr(true),
	}
	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	err := r.ReconcileApplicationSetControllerNetworkPolicy(a)
	assert.NoError(t, err)

	np := &networkingv1.NetworkPolicy{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: fmt.Sprintf("%s-%s", a.Name, ArgoCDApplicationSetControllerNetworkPolicy), Namespace: a.Namespace}, np)
	assert.NoError(t, err)

	// podSelector: argocd-applicationset-controller
	assert.Equal(t, "argocd-applicationset-controller", np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])
	assert.Equal(t, networkingv1.PolicyTypeIngress, np.Spec.PolicyTypes[0])

	// ingress: from any namespace on 7000 and 8080
	assert.Equal(t, 1, len(np.Spec.Ingress))
	assert.Equal(t, 1, len(np.Spec.Ingress[0].From))
	assert.NotNil(t, np.Spec.Ingress[0].From[0].NamespaceSelector)
	assert.Equal(t, metav1.LabelSelector{}, *np.Spec.Ingress[0].From[0].NamespaceSelector)
	assert.Equal(t, 2, len(np.Spec.Ingress[0].Ports))
	assert.Equal(t, intstr.FromInt(7000), *np.Spec.Ingress[0].Ports[0].Port)
	assert.Equal(t, intstr.FromInt(8080), *np.Spec.Ingress[0].Ports[1].Port)
}

func TestApplicationSetControllerNetworkPolicyDisabledDeletesExisting(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
		Enabled: boolPtr(true),
	}
	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	// create NP
	err := r.ReconcileApplicationSetControllerNetworkPolicy(a)
	assert.NoError(t, err)

	// disable and ensure NP is deleted
	a.Spec.ApplicationSet = nil
	err = r.ReconcileApplicationSetControllerNetworkPolicy(a)
	assert.NoError(t, err)

	np := &networkingv1.NetworkPolicy{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: fmt.Sprintf("%s-%s", a.Name, ArgoCDApplicationSetControllerNetworkPolicy), Namespace: a.Namespace}, np)
	assert.Error(t, err)
}

func TestArgoCDServerNetworkPolicy(t *testing.T) {
	a := makeTestArgoCD()
	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	err := r.ReconcileArgoCDServerNetworkPolicy(a)
	assert.NoError(t, err)

	np := &networkingv1.NetworkPolicy{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: fmt.Sprintf("%s-%s", a.Name, ArgoCDServerNetworkPolicy), Namespace: a.Namespace}, np)
	assert.NoError(t, err)

	assert.Equal(t, nameWithSuffix("server", a), np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])
	assert.Equal(t, networkingv1.PolicyTypeIngress, np.Spec.PolicyTypes[0])

	// One empty ingress rule (allows all ingress)
	assert.Equal(t, 1, len(np.Spec.Ingress))
	assert.Equal(t, 0, len(np.Spec.Ingress[0].From))
	assert.Equal(t, 0, len(np.Spec.Ingress[0].Ports))
}

func TestArgoCDApplicationControllerNetworkPolicy(t *testing.T) {
	a := makeTestArgoCD()
	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	err := r.ReconcileArgoCDApplicationControllerNetworkPolicy(a)
	assert.NoError(t, err)

	np := &networkingv1.NetworkPolicy{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: fmt.Sprintf("%s-%s", a.Name, ArgoCDApplicationControllerNetworkPolicy), Namespace: a.Namespace}, np)
	assert.NoError(t, err)

	assert.Equal(t, nameWithSuffix("application-controller", a), np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])
	assert.Equal(t, networkingv1.PolicyTypeIngress, np.Spec.PolicyTypes[0])

	assert.Equal(t, 1, len(np.Spec.Ingress))
	assert.Equal(t, 1, len(np.Spec.Ingress[0].From))
	assert.NotNil(t, np.Spec.Ingress[0].From[0].NamespaceSelector)
	assert.Equal(t, metav1.LabelSelector{}, *np.Spec.Ingress[0].From[0].NamespaceSelector)
	assert.Equal(t, 1, len(np.Spec.Ingress[0].Ports))
	assert.Equal(t, intstr.FromInt(8082), *np.Spec.Ingress[0].Ports[0].Port)
}

func TestArgoCDRepoServerNetworkPolicy(t *testing.T) {
	a := makeTestArgoCD()
	r := makeTestReconciler(makeTestReconcilerClient(makeTestReconcilerScheme(argoproj.AddToScheme), []client.Object{a}, []client.Object{a}, []runtime.Object{}), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	err := r.ReconcileArgoCDRepoServerNetworkPolicy(a)
	assert.NoError(t, err)

	np := &networkingv1.NetworkPolicy{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: fmt.Sprintf("%s-%s", a.Name, ArgoCDRepoServerNetworkPolicy), Namespace: a.Namespace}, np)
	assert.NoError(t, err)

	assert.Equal(t, nameWithSuffix("repo-server", a), np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])
	assert.Equal(t, networkingv1.PolicyTypeIngress, np.Spec.PolicyTypes[0])

	assert.Equal(t, 2, len(np.Spec.Ingress))

	// Rule 1: allow internal components on 8081
	assert.Equal(t, 5, len(np.Spec.Ingress[0].From))
	internalFrom := make([]string, 0, len(np.Spec.Ingress[0].From))
	for _, peer := range np.Spec.Ingress[0].From {
		if peer.PodSelector != nil {
			internalFrom = append(internalFrom, peer.PodSelector.MatchLabels["app.kubernetes.io/name"])
		}
	}
	assert.ElementsMatch(t, []string{
		nameWithSuffix("application-controller", a),
		nameWithSuffix("server", a),
		nameWithSuffix("notifications-controller", a),
		"argocd-applicationset-controller",
		nameWithSuffix("applicationset-controller", a),
	}, internalFrom)
	assert.Equal(t, 1, len(np.Spec.Ingress[0].Ports))
	assert.Equal(t, intstr.FromInt(8081), *np.Spec.Ingress[0].Ports[0].Port)

	// Rule 2: allow any namespace on 8084 (metrics)
	assert.Equal(t, 1, len(np.Spec.Ingress[1].From))
	assert.NotNil(t, np.Spec.Ingress[1].From[0].NamespaceSelector)
	assert.Equal(t, metav1.LabelSelector{}, *np.Spec.Ingress[1].From[0].NamespaceSelector)
	assert.Equal(t, 1, len(np.Spec.Ingress[1].Ports))
	assert.Equal(t, intstr.FromInt(8084), *np.Spec.Ingress[1].Ports[0].Port)
}

func TestArgoCDRepoServerNetworkPolicyUpdatesExisting(t *testing.T) {
	a := makeTestArgoCD()

	// Create an existing NP with wrong spec
	existing := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", a.Name, ArgoCDRepoServerNetworkPolicy),
			Namespace: a.Namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "wrong",
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: TCPProtocol,
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 9999},
						},
					},
				},
			},
		},
	}

	r := makeTestReconciler(makeTestReconcilerClient(
		makeTestReconcilerScheme(argoproj.AddToScheme),
		[]client.Object{a, existing},
		[]client.Object{a},
		[]runtime.Object{},
	), makeTestReconcilerScheme(argoproj.AddToScheme), testclient.NewSimpleClientset())

	// Reconcile should update it to the desired spec
	err := r.ReconcileArgoCDRepoServerNetworkPolicy(a)
	assert.NoError(t, err)

	np := &networkingv1.NetworkPolicy{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: existing.Name, Namespace: existing.Namespace}, np)
	assert.NoError(t, err)

	assert.Equal(t, nameWithSuffix("repo-server", a), np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])
	assert.Equal(t, 2, len(np.Spec.Ingress))
	assert.Equal(t, intstr.FromInt(8081), *np.Spec.Ingress[0].Ports[0].Port)
	assert.Equal(t, intstr.FromInt(8084), *np.Spec.Ingress[1].Ports[0].Port)

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
	assert.Len(t, truncated, 37)       // maxCRNameLength includes the hash suffix
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
	assert.Len(t, truncated, 37)       // maxCRNameLength includes the hash suffix
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
