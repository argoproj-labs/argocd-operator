package secret

import (
	"sort"
	"strings"
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	argopass "github.com/argoproj/argo-cd/v2/util/password"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	testName      = "test-name"
	testNamespace = "test-ns"
	testKey       = "testKey"
	testVal       = "testVal"
	testKVP       = map[string]string{
		testKey: testVal,
	}
)

type secretOpt func(*corev1.Secret)

func makeTestSecretsReconciler(t *testing.T, objs ...runtime.Object) *SecretReconciler {
	s := scheme.Scheme
	assert.NoError(t, argoproj.AddToScheme(s))

	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
	logger := ctrl.Log.WithName(SecretsControllerName)

	return &SecretReconciler{
		Client:   cl,
		Scheme:   s,
		Instance: argocdcommon.MakeTestArgoCD(),
		Logger:   logger,
	}
}

func Test_reconcileCredentialsSecret(t *testing.T) {
	testSR := makeTestSecretsReconciler(t)
	testSR.Instance = argocdcommon.MakeTestArgoCD(func(ac *argoproj.ArgoCD) {
		ac.Name = testName
		ac.Namespace = testNamespace
		ac.Annotations = testKVP
	})

	err := testSR.reconcileCredentialsSecret()
	assert.NoError(t, err)

	_, err = workloads.GetSecret("test-name-cluster", "test-ns", testSR.Client)
	assert.NoError(t, err)

	err = workloads.DeleteSecret("test-name-cluster", "test-ns", testSR.Client)
	assert.NoError(t, err)

	err = testSR.reconcileCredentialsSecret()
	assert.NoError(t, err)

	_, err = workloads.GetSecret("test-name-cluster", "test-ns", testSR.Client)
	assert.NoError(t, err)
}

func Test_reconcileCASecret(t *testing.T) {
	testSR := makeTestSecretsReconciler(t)
	testSR.Instance = argocdcommon.MakeTestArgoCD(func(ac *argoproj.ArgoCD) {
		ac.Name = testName
		ac.Namespace = testNamespace
		ac.Annotations = testKVP
	})

	err := testSR.reconcileCASecret()
	assert.NoError(t, err)

	_, err = workloads.GetSecret("test-name-ca", "test-ns", testSR.Client)
	assert.NoError(t, err)

	err = workloads.DeleteSecret("test-name-ca", "test-ns", testSR.Client)
	assert.NoError(t, err)

	err = testSR.reconcileCASecret()
	assert.NoError(t, err)

	_, err = workloads.GetSecret("test-name-ca", "test-ns", testSR.Client)
	assert.NoError(t, err)
}

func Test_reconcileTLSSecret(t *testing.T) {
	testSR := makeTestSecretsReconciler(t)
	testSR.Instance = argocdcommon.MakeTestArgoCD(func(ac *argoproj.ArgoCD) {
		ac.Name = testName
		ac.Namespace = testNamespace
		ac.Annotations = testKVP
	})

	// expect not found error for missing CA secret
	err := testSR.reconcileTLSSecret()
	assert.Error(t, err)

	caSecret, _ := testSR.getDesiredCASecret("test-name-ca")
	err = workloads.CreateSecret(caSecret, testSR.Client)
	assert.NoError(t, err)

	err = testSR.reconcileTLSSecret()
	assert.NoError(t, err)

	_, err = workloads.GetSecret("test-name-tls", "test-ns", testSR.Client)
	assert.NoError(t, err)

	err = workloads.DeleteSecret("test-name-tls", "test-ns", testSR.Client)
	assert.NoError(t, err)

	err = testSR.reconcileTLSSecret()
	assert.NoError(t, err)

	_, err = workloads.GetSecret("test-name-tls", "test-ns", testSR.Client)
	assert.NoError(t, err)
}

func Test_reconcileClusterPermissionsSecret(t *testing.T) {
	testSR := makeTestSecretsReconciler(t)
	testSR.Instance = argocdcommon.MakeTestArgoCD(func(ac *argoproj.ArgoCD) {
		ac.Name = testName
		ac.Namespace = testNamespace
		ac.Annotations = testKVP
	})

	// set instance to cluster scoped, verify empty managed namespace list
	testSR.ClusterScoped = true
	err := testSR.reconcileClusterPermissionsSecret()
	assert.NoError(t, err)

	existingClusterPermSecret, err := workloads.GetSecret("test-name-default-cluster-config", "test-ns", testSR.Client)
	assert.NoError(t, err)

	assert.Nil(t, existingClusterPermSecret.Data["namespaces"])

	// update instance to namespace scoped, verify updated managed namespace list
	testSR.ClusterScoped = false
	testSR.ManagedNamespaces = map[string]string{
		"ns-1": "",
		"ns-3": "",
		"ns-5": "",
	}

	err = testSR.reconcileClusterPermissionsSecret()
	assert.NoError(t, err)

	existingClusterPermSecret, err = workloads.GetSecret("test-name-default-cluster-config", "test-ns", testSR.Client)
	assert.NoError(t, err)
	expectedNsList := []string{"ns-1", "ns-3", "ns-5"}

	assert.Equal(t, expectedNsList, strings.Split(string(existingClusterPermSecret.Data["namespaces"]), ","))

	// update managed ns list
	testSR.ManagedNamespaces = map[string]string{
		"ns-2": "",
		"ns-4": "",
		"ns-6": "",
	}

	err = testSR.reconcileClusterPermissionsSecret()
	assert.NoError(t, err)

	existingClusterPermSecret, err = workloads.GetSecret("test-name-default-cluster-config", "test-ns", testSR.Client)
	assert.NoError(t, err)
	expectedNsList = []string{"ns-2", "ns-4", "ns-6"}

	assert.Equal(t, expectedNsList, strings.Split(string(existingClusterPermSecret.Data["namespaces"]), ","))

	// switch back to cluster scoped, verify empty managed ns list
	testSR.ClusterScoped = true
	err = testSR.reconcileClusterPermissionsSecret()
	assert.NoError(t, err)

	existingClusterPermSecret, err = workloads.GetSecret("test-name-default-cluster-config", "test-ns", testSR.Client)
	assert.NoError(t, err)

	assert.Nil(t, existingClusterPermSecret.Data["namespaces"])
}

func Test_reconcileArgoCDSecret(t *testing.T) {
	testSR := makeTestSecretsReconciler(t)
	testSR.Instance = argocdcommon.MakeTestArgoCD(func(ac *argoproj.ArgoCD) {
		ac.Name = testName
		ac.Namespace = testNamespace
		ac.Annotations = testKVP
	})

	// expect not found error for missing secrets
	err := testSR.reconcileArgoCDSecret()
	assert.Error(t, err)

	err = testSR.reconcileCredentialsSecret()
	assert.NoError(t, err)

	// expect not found error for missing secrets
	err = testSR.reconcileArgoCDSecret()
	assert.Error(t, err)

	err = testSR.reconcileCASecret()
	assert.NoError(t, err)

	err = testSR.reconcileTLSSecret()
	assert.NoError(t, err)

	err = testSR.reconcileArgoCDSecret()
	assert.NoError(t, err)

	existingArgoCDSecret, err := workloads.GetSecret(ArgoCDSecretName, testNamespace, testSR.Client)
	assert.NoError(t, err)

	existingCredsSecret, err := workloads.GetSecret("test-name-cluster", testNamespace, testSR.Client)
	assert.NoError(t, err)

	// update existing secret data with new password and remove session key from argocd-secret, verify if argocd-secret is
	// updated appropriately
	existingCredsSecret.Data = map[string][]byte{
		common.ArgoCDKeyAdminPassword: []byte("new-pw"),
	}
	delete(existingArgoCDSecret.Data, common.ArgoCDKeyServerSecretKey)
	err = workloads.UpdateSecret(existingCredsSecret, testSR.Client)
	assert.NoError(t, err)
	err = workloads.UpdateSecret(existingArgoCDSecret, testSR.Client)
	assert.NoError(t, err)

	err = testSR.reconcileArgoCDSecret()
	assert.NoError(t, err)

	existingArgoCDSecret, err = workloads.GetSecret(ArgoCDSecretName, testNamespace, testSR.Client)
	assert.NoError(t, err)

	pwdUnchanged, _ := argopass.VerifyPassword("new-pw", string(existingArgoCDSecret.Data["admin.password"]))
	assert.True(t, pwdUnchanged)

}

func Test_getClusterSecrets(t *testing.T) {
	testSR := makeTestSecretsReconciler(t,
		getTestSecret(func(s *corev1.Secret) {
			s.Name = "secret-1"
			s.Labels[common.ArgoCDArgoprojKeySecretType] = "cluster"
			s.Namespace = testNamespace
		}),
		getTestSecret(func(s *corev1.Secret) {
			s.Name = "secret-2"
			s.Namespace = testNamespace
		}),
		getTestSecret(func(s *corev1.Secret) {
			s.Name = "secret-3"
			s.Labels[common.ArgoCDArgoprojKeySecretType] = "cluster"
			s.Namespace = testNamespace
		}),
	)
	testSR.Instance = argocdcommon.MakeTestArgoCD(func(ac *argoproj.ArgoCD) {
		ac.Name = testName
		ac.Namespace = testNamespace
		ac.Annotations = testKVP
	})

	expectedSecrets := []string{"secret-1", "secret-3"}

	actualSecretList, err := testSR.GetClusterSecrets()
	assert.NoError(t, err)

	actualSecrets := []string{}
	for _, secret := range actualSecretList.Items {
		actualSecrets = append(actualSecrets, secret.Name)
	}
	sort.Strings(actualSecrets)

	assert.Equal(t, expectedSecrets, actualSecrets)
}

func Test_getDesiredSecretTmplObj(t *testing.T) {
	testSR := makeTestSecretsReconciler(t)
	testSR.Instance = argocdcommon.MakeTestArgoCD(func(ac *argoproj.ArgoCD) {
		ac.Name = testName
		ac.Namespace = testNamespace
		ac.Annotations = testKVP
	})

	expectedSecretTmpObj := getTestSecret(func(s *corev1.Secret) {
		s.Annotations = testKVP
	})
	actualSecretTmplObj := testSR.getDesiredSecretTmplObj(testName)
	assert.Equal(t, expectedSecretTmpObj, actualSecretTmplObj)
}

func getTestSecret(opts ...secretOpt) *corev1.Secret {
	desiredSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-ns",
			Labels: map[string]string{
				"app.kubernetes.io/name":       "test-name",
				"app.kubernetes.io/part-of":    "argocd",
				"app.kubernetes.io/instance":   "test-name",
				"app.kubernetes.io/managed-by": "argocd-operator",
			},
		},
	}

	for _, opt := range opts {
		opt(desiredSecret)
	}
	return desiredSecret
}
