package argocd

import (
	"context"
	"os"
	"reflect"
	"sort"
	"testing"

	"gotest.tools/assert"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
)

func Test_newCASecret(t *testing.T) {
	cr := &argoprojv1alpha1.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-argocd",
			Namespace: "argocd",
		},
	}

	s, err := newCASecret(cr)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		corev1.ServiceAccountRootCAKey,
		corev1.TLSCertKey,
		corev1.TLSPrivateKeyKey,
	}
	if k := byteMapKeys(s.Data); !reflect.DeepEqual(want, k) {
		t.Fatalf("got %#v, want %#v", k, want)
	}
}

func byteMapKeys(m map[string][]byte) []string {
	r := []string{}
	for k := range m {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}

func Test_ReconcileArgoCD_ClusterPermissionsSecret(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	testSecret := argoutil.NewSecretWithSuffix(a.ObjectMeta, "default-cluster-config")
	assert.ErrorContains(t, r.client.Get(context.TODO(), types.NamespacedName{Name: testSecret.Name, Namespace: testSecret.Namespace}, testSecret), "not found")

	assert.NilError(t, r.reconcileClusterPermissionsSecret(a))
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: testSecret.Name, Namespace: testSecret.Namespace}, testSecret))
	assert.DeepEqual(t, string(testSecret.Data["namespaces"]), a.Namespace)

	want := "someRandomNamespace"
	testSecret.Data["namespaces"] = []byte(want)
	r.client.Update(context.TODO(), testSecret)

	// reconcile to check nothing gets updated
	assert.NilError(t, r.reconcileClusterPermissionsSecret(a))
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: testSecret.Name, Namespace: testSecret.Namespace}, testSecret))
	assert.DeepEqual(t, string(testSecret.Data["namespaces"]), want)

	os.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", a.Namespace)
	defer os.Unsetenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")

	assert.NilError(t, r.reconcileClusterPermissionsSecret(a))
	assert.ErrorContains(t, r.client.Get(context.TODO(), types.NamespacedName{Name: testSecret.Name, Namespace: testSecret.Namespace}, testSecret), "not found")
}
