package argocd

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"

	argopass "github.com/argoproj/argo-cd/v3/util/password"

	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	testclient "k8s.io/client-go/kubernetes/fake"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

func Test_newCASecret(t *testing.T) {
	cr := &argoproj.ArgoCD{
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

func Test_ReconcileArgoCD_ReconcileRepoTLSSecret(t *testing.T) {
	argocd := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd",
			Namespace: "argocd-operator",
			UID:       "abcd",
		},
	}
	crt := []byte("foo")
	key := []byte("bar")
	t.Run("Reconcile TLS secret", func(t *testing.T) {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-repo-server",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "argoproj.io/v1alpha1",
						Kind:       "ArgoCD",
						Name:       "argocd",
						UID:        argocd.GetUID(),
					},
				},
				UID: "service-123",
			},
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-repo-server-tls",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Service",
						Name:       "argocd-repo-server",
						UID:        service.GetUID(),
					},
				},
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       crt,
				corev1.TLSPrivateKeyKey: key,
			},
		}
		var sumOver []byte
		sumOver = append(sumOver, crt...)
		sumOver = append(sumOver, key...)
		shasum := fmt.Sprintf("%x", sha256.Sum256(sumOver))
		serverDepl := newDeploymentWithSuffix("server", "server", argocd)
		repoDepl := newDeploymentWithSuffix("repo-server", "repo-server", argocd)
		ctrlSts := newStatefulSetWithSuffix("application-controller", "application-controller", argocd)

		resObjs := []client.Object{argocd,
			secret,
			service,
			serverDepl,
			repoDepl,
			ctrlSts}
		subresObjs := []client.Object{argocd,
			serverDepl,
			repoDepl,
			ctrlSts}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme, configv1.Install, routev1.Install)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		err := r.reconcileRepoServerTLSSecret(argocd)
		if err != nil {
			t.Errorf("Error should be nil, but is %v", err)
		}
		if shasum != argocd.Status.RepoTLSChecksum {
			t.Errorf("Error in SHA256 sum of secret, want=%s got=%s", shasum, argocd.Status.RepoTLSChecksum)
		}

		// Workloads should have been requested to re-rollout on a change
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-server", Namespace: "argocd-operator"}, serverDepl)
		deplRollout, ok := serverDepl.Spec.Template.ObjectMeta.Labels["repo.tls.cert.changed"]
		if !ok {
			t.Errorf("Expected rollout of argocd-server, but it didn't happen: %v", serverDepl.Spec.Template.ObjectMeta.Labels)
		}
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-repo-server", Namespace: "argocd-operator"}, repoDepl)
		repoRollout, ok := repoDepl.Spec.Template.ObjectMeta.Labels["repo.tls.cert.changed"]
		if !ok {
			t.Errorf("Expected rollout of argocd-repo-server, but it didn't happen: %v", repoDepl.Spec.Template.ObjectMeta.Labels)
		}
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-application-controller", Namespace: "argocd-operator"}, ctrlSts)
		ctrlRollout, ok := ctrlSts.Spec.Template.ObjectMeta.Labels["repo.tls.cert.changed"]
		if !ok {
			t.Errorf("Expected rollout of argocd-application-server, but it didn't happen: %v", ctrlSts.Spec.Template.ObjectMeta.Labels)
		}

		// Second run - no change
		err = r.reconcileRepoServerTLSSecret(argocd)
		if err != nil {
			t.Errorf("Error should be nil, but is %v", err)
		}
		if shasum != argocd.Status.RepoTLSChecksum {
			t.Errorf("Error in SHA256 sum of secret, want=%s got=%s", shasum, argocd.Status.RepoTLSChecksum)
		}

		// This time, label should not have changed
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-server", Namespace: "argocd-operator"}, serverDepl)
		deplRolloutNew, ok := serverDepl.Spec.Template.ObjectMeta.Labels["repo.tls.cert.changed"]
		if !ok || deplRollout != deplRolloutNew {
			t.Errorf("Did not expect rollout of argocd-server, but it did happen: %v", serverDepl.Spec.Template.ObjectMeta.Labels)
		}
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-repo-server", Namespace: "argocd-operator"}, repoDepl)
		repoRolloutNew, ok := repoDepl.Spec.Template.ObjectMeta.Labels["repo.tls.cert.changed"]
		if !ok || repoRollout != repoRolloutNew {
			t.Errorf("Did not expect rollout of argocd-repo-server, but it did happen: %v", repoDepl.Spec.Template.ObjectMeta.Labels)
		}
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-application-controller", Namespace: "argocd-operator"}, ctrlSts)
		ctrlRolloutNew, ok := ctrlSts.Spec.Template.ObjectMeta.Labels["repo.tls.cert.changed"]
		if !ok || ctrlRollout != ctrlRolloutNew {
			t.Errorf("Did not expect rollout of argocd-application-server, but it did happen: %v", ctrlSts.Spec.Template.ObjectMeta.Labels)
		}

		// Update certificate in the secret must trigger new rollout
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-repo-server-tls", Namespace: "argocd-operator"}, secret)
		secret.Data["tls.crt"] = []byte("bar")
		r.Client.Update(context.TODO(), secret)

		sumOver = []byte{}
		sumOver = append(sumOver, []byte("bar")...)
		sumOver = append(sumOver, key...)
		shasum = fmt.Sprintf("%x", sha256.Sum256(sumOver))

		// Second run - no change
		err = r.reconcileRepoServerTLSSecret(argocd)
		if err != nil {
			t.Errorf("Error should be nil, but is %v", err)
		}
		if shasum != argocd.Status.RepoTLSChecksum {
			t.Errorf("Error in SHA256 sum of secret, want=%s got=%s", shasum, argocd.Status.RepoTLSChecksum)
		}

		// This time, label should have changed
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-server", Namespace: "argocd-operator"}, serverDepl)
		deplRolloutNew, ok = serverDepl.Spec.Template.ObjectMeta.Labels["repo.tls.cert.changed"]
		if !ok || deplRollout == deplRolloutNew {
			t.Errorf("Expected rollout of argocd-server, but it didn't happen: %v", serverDepl.Spec.Template.ObjectMeta.Labels)
		}
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-repo-server", Namespace: "argocd-operator"}, repoDepl)
		repoRolloutNew, ok = repoDepl.Spec.Template.ObjectMeta.Labels["repo.tls.cert.changed"]
		if !ok || repoRollout == repoRolloutNew {
			t.Errorf("Expected rollout of argocd-repo-server, but it didn't happen: %v", repoDepl.Spec.Template.ObjectMeta.Labels)
		}
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-application-controller", Namespace: "argocd-operator"}, ctrlSts)
		ctrlRolloutNew, ok = ctrlSts.Spec.Template.ObjectMeta.Labels["repo.tls.cert.changed"]
		if !ok || ctrlRollout == ctrlRolloutNew {
			t.Errorf("Expected rollout of argocd-application-controller, but it didn't happen: %v", ctrlSts.Spec.Template.ObjectMeta.Labels)
		}

	})

}

func Test_ReconcileArgoCD_ReconcileExistingArgoSecret(t *testing.T) {
	argocd := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd",
			Namespace: "argocd-operator",
		},
	}

	clusterSecret := argoutil.NewSecretWithSuffix(argocd, "cluster")
	clusterSecret.Data = map[string][]byte{common.ArgoCDKeyAdminPassword: []byte("something")}
	tlsSecret := argoutil.NewSecretWithSuffix(argocd, "tls")

	resObjs := []client.Object{argocd}
	subresObjs := []client.Object{argocd}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	r.Client.Create(context.TODO(), clusterSecret)
	r.Client.Create(context.TODO(), tlsSecret)

	err := r.reconcileArgoSecret(argocd)

	assert.NoError(t, err)

	testSecret := &corev1.Secret{}
	secretErr := r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: "argocd-operator"}, testSecret)
	assert.NoError(t, secretErr)

	// if you remove the secret.Data it should come back, including the secretKey
	testSecret.Data = nil
	r.Client.Update(context.TODO(), testSecret)

	_ = r.reconcileExistingArgoSecret(argocd, testSecret, clusterSecret, tlsSecret)
	_ = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: "argocd-operator"}, testSecret)

	if testSecret.Data == nil {
		t.Errorf("Expected data for data.server but got nothing")
	}

	if testSecret.Data[common.ArgoCDKeyServerSecretKey] == nil {
		t.Errorf("Expected data for data.server.secretKey but got nothing")
	}
	assert.True(t, argoutil.IsTrackedByOperator(testSecret.ObjectMeta.Labels))
}

func Test_ReconcileArgoCD_ReconcileShouldNotChangeWhenUpdatedAdminPass(t *testing.T) {
	argocd := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd",
			Namespace: "argocd-operator",
		},
	}

	clusterSecret := argoutil.NewSecretWithSuffix(argocd, "cluster")
	clusterSecret.Data = map[string][]byte{common.ArgoCDKeyAdminPassword: []byte("something")}
	tlsSecret := argoutil.NewSecretWithSuffix(argocd, "tls")

	resObjs := []client.Object{argocd}
	subresObjs := []client.Object{argocd}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	r.Client.Create(context.TODO(), clusterSecret)
	r.Client.Create(context.TODO(), tlsSecret)

	err := r.reconcileArgoSecret(argocd)

	assert.NoError(t, err)

	testSecret := &corev1.Secret{}
	secretErr := r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: "argocd-operator"}, testSecret)
	assert.NoError(t, secretErr)
	assert.True(t, argoutil.IsTrackedByOperator(testSecret.ObjectMeta.Labels))

	// simulating update of argo-cd Admin password from cli or argocd dashboard
	hashedPassword, _ := argopass.HashPassword("updated_password")
	testSecret.Data[common.ArgoCDKeyAdminPassword] = []byte(hashedPassword)
	mTime := nowBytes()
	testSecret.Data[common.ArgoCDKeyAdminPasswordMTime] = mTime
	r.Client.Update(context.TODO(), testSecret)

	_ = r.reconcileExistingArgoSecret(argocd, testSecret, clusterSecret, tlsSecret)
	_ = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: "argocd-operator"}, testSecret)

	// checking if reconciliation updates the ArgoCDKeyAdminPassword and ArgoCDKeyAdminPasswordMTime
	if string(testSecret.Data[common.ArgoCDKeyAdminPassword]) != hashedPassword {
		t.Errorf("Expected hashedPassword to reamin unchanged but got updated")
	}
	if string(testSecret.Data[common.ArgoCDKeyAdminPasswordMTime]) != string(mTime) {
		t.Errorf("Expected ArgoCDKeyAdminPasswordMTime to reamin unchanged but got updated")
	}

	// if you remove the secret.Data it should come back, including the secretKey
	testSecret.Data = nil
	r.Client.Update(context.TODO(), testSecret)

	_ = r.reconcileExistingArgoSecret(argocd, testSecret, clusterSecret, tlsSecret)
	_ = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: "argocd-operator"}, testSecret)

	if testSecret.Data == nil {
		t.Errorf("Expected data for data.server but got nothing")
	}

	if testSecret.Data[common.ArgoCDKeyServerSecretKey] == nil {
		t.Errorf("Expected data for data.server.secretKey but got nothing")
	}
	assert.True(t, argoutil.IsTrackedByOperator(testSecret.ObjectMeta.Labels))
}

func Test_ReconcileArgoCD_ReconcileRedisTLSSecret(t *testing.T) {
	argocd := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd",
			Namespace: "argocd-operator",
			UID:       "abcd",
		},
	}
	crt := []byte("foo")
	key := []byte("bar")
	t.Run("Reconcile TLS secret", func(t *testing.T) {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-redis",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "argoproj.io/v1alpha1",
						Kind:       "ArgoCD",
						Name:       "argocd",
						UID:        argocd.GetUID(),
					},
				},
				UID: "service-123",
			},
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-operator-redis-tls",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Service",
						Name:       "argocd-redis",
						UID:        service.GetUID(),
					},
				},
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       crt,
				corev1.TLSPrivateKeyKey: key,
			},
		}
		var sumOver []byte
		sumOver = append(sumOver, crt...)
		sumOver = append(sumOver, key...)
		shasum := fmt.Sprintf("%x", sha256.Sum256(sumOver))
		serverDepl := newDeploymentWithSuffix("server", "server", argocd)
		repoDepl := newDeploymentWithSuffix("repo-server", "repo-server", argocd)
		redisDepl := newDeploymentWithSuffix("redis", "redis", argocd)
		ctrlSts := newStatefulSetWithSuffix("application-controller", "application-controller", argocd)

		resObjs := []client.Object{argocd,
			secret,
			service,
			serverDepl,
			repoDepl,
			ctrlSts,
			redisDepl}
		subresObjs := []client.Object{argocd,
			serverDepl,
			repoDepl,
			ctrlSts,
			redisDepl}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme, configv1.Install, routev1.Install)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		err := r.reconcileRedisTLSSecret(argocd, true)
		if err != nil {
			t.Errorf("Error should be nil, but is %v", err)
		}
		if shasum != argocd.Status.RedisTLSChecksum {
			t.Errorf("Error in SHA256 sum of secret, want=%s got=%s", shasum, argocd.Status.RedisTLSChecksum)
		}

		certChangedLabel := "redis.tls.cert.changed"

		// Workloads should have been requested to re-rollout on a change
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-server", Namespace: "argocd-operator"}, serverDepl)
		deplRollout, ok := serverDepl.Spec.Template.ObjectMeta.Labels[certChangedLabel]
		if !ok {
			t.Errorf("Expected rollout of argocd-server, but it didn't happen: %v", serverDepl.Spec.Template.ObjectMeta.Labels)
		}
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-repo-server", Namespace: "argocd-operator"}, repoDepl)
		repoRollout, ok := repoDepl.Spec.Template.ObjectMeta.Labels[certChangedLabel]
		if !ok {
			t.Errorf("Expected rollout of argocd-repo-server, but it didn't happen: %v", repoDepl.Spec.Template.ObjectMeta.Labels)
		}
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-redis", Namespace: "argocd-operator"}, redisDepl)
		redisRollout, ok := redisDepl.Spec.Template.ObjectMeta.Labels[certChangedLabel]
		if !ok {
			t.Errorf("Expected rollout of argocd-redis, but it didn't happen: %v", redisDepl.Spec.Template.ObjectMeta.Labels)
		}
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-application-controller", Namespace: "argocd-operator"}, ctrlSts)
		ctrlRollout, ok := ctrlSts.Spec.Template.ObjectMeta.Labels[certChangedLabel]
		if !ok {
			t.Errorf("Expected rollout of argocd-application-server, but it didn't happen: %v", ctrlSts.Spec.Template.ObjectMeta.Labels)
		}

		// Second run - no change
		err = r.reconcileRedisTLSSecret(argocd, true)
		if err != nil {
			t.Errorf("Error should be nil, but is %v", err)
		}
		if shasum != argocd.Status.RedisTLSChecksum {
			t.Errorf("Error in SHA256 sum of secret, want=%s got=%s", shasum, argocd.Status.RepoTLSChecksum)
		}

		// This time, label should not have changed
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-server", Namespace: "argocd-operator"}, serverDepl)
		deplRolloutNew, ok := serverDepl.Spec.Template.ObjectMeta.Labels[certChangedLabel]
		if !ok || deplRollout != deplRolloutNew {
			t.Errorf("Did not expect rollout of argocd-server, but it did happen: %v", serverDepl.Spec.Template.ObjectMeta.Labels)
		}
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-repo-server", Namespace: "argocd-operator"}, repoDepl)
		repoRolloutNew, ok := repoDepl.Spec.Template.ObjectMeta.Labels[certChangedLabel]
		if !ok || repoRollout != repoRolloutNew {
			t.Errorf("Did not expect rollout of argocd-repo-server, but it did happen: %v", repoDepl.Spec.Template.ObjectMeta.Labels)
		}
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-redis", Namespace: "argocd-operator"}, redisDepl)
		redisRolloutNew, ok := redisDepl.Spec.Template.ObjectMeta.Labels[certChangedLabel]
		if !ok || redisRollout != redisRolloutNew {
			t.Errorf("Did not expect rollout of argocd-redis, but it did happen: %v", redisDepl.Spec.Template.ObjectMeta.Labels)
		}
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-application-controller", Namespace: "argocd-operator"}, ctrlSts)
		ctrlRolloutNew, ok := ctrlSts.Spec.Template.ObjectMeta.Labels[certChangedLabel]
		if !ok || ctrlRollout != ctrlRolloutNew {
			t.Errorf("Did not expect rollout of argocd-application-server, but it did happen: %v", ctrlSts.Spec.Template.ObjectMeta.Labels)
		}

		// Update certificate in the secret must trigger new rollout
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-repo-server-tls", Namespace: "argocd-operator"}, secret)
		secret.Data["tls.crt"] = []byte("bar")
		r.Client.Update(context.TODO(), secret)

		sumOver = []byte{}
		sumOver = append(sumOver, []byte("bar")...)
		sumOver = append(sumOver, key...)
		shasum = fmt.Sprintf("%x", sha256.Sum256(sumOver))

		// Second run - no change
		err = r.reconcileRedisTLSSecret(argocd, true)
		if err != nil {
			t.Errorf("Error should be nil, but is %v", err)
		}
		if shasum != argocd.Status.RedisTLSChecksum {
			t.Errorf("Error in SHA256 sum of secret, want=%s got=%s", shasum, argocd.Status.RedisTLSChecksum)
		}

		// This time, label should have changed
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-server", Namespace: "argocd-operator"}, serverDepl)
		deplRolloutNew, ok = serverDepl.Spec.Template.ObjectMeta.Labels[certChangedLabel]
		if !ok || deplRollout == deplRolloutNew {
			t.Errorf("Expected rollout of argocd-server, but it didn't happen: %v", serverDepl.Spec.Template.ObjectMeta.Labels)
		}
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-repo-server", Namespace: "argocd-operator"}, repoDepl)
		repoRolloutNew, ok = repoDepl.Spec.Template.ObjectMeta.Labels[certChangedLabel]
		if !ok || repoRollout == repoRolloutNew {
			t.Errorf("Expected rollout of argocd-repo-server, but it didn't happen: %v", repoDepl.Spec.Template.ObjectMeta.Labels)
		}
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-redis", Namespace: "argocd-operator"}, redisDepl)
		redisRolloutNew, ok = repoDepl.Spec.Template.ObjectMeta.Labels[certChangedLabel]
		if !ok || redisRollout == redisRolloutNew {
			t.Errorf("Expected rollout of argocd-redis, but it didn't happen: %v", redisDepl.Spec.Template.ObjectMeta.Labels)
		}
		r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-application-controller", Namespace: "argocd-operator"}, ctrlSts)
		ctrlRolloutNew, ok = ctrlSts.Spec.Template.ObjectMeta.Labels[certChangedLabel]
		if !ok || ctrlRollout == ctrlRolloutNew {
			t.Errorf("Expected rollout of argocd-application-controller, but it didn't happen: %v", ctrlSts.Spec.Template.ObjectMeta.Labels)
		}
	})
}

func Test_ReconcileArgoCD_ClusterPermissionsSecret(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	testSecret := argoutil.NewSecretWithSuffix(a, "default-cluster-config")
	//assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: testSecret.Name, Namespace: testSecret.Namespace}, testSecret), "not found")
	//TODO: https://github.com/stretchr/testify/pull/1022 introduced ErrorContains, but is not yet available in a tagged release. Revert to ErrorContains once this becomes available
	assert.Error(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: testSecret.Name, Namespace: testSecret.Namespace}, testSecret))
	assert.Contains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: testSecret.Name, Namespace: testSecret.Namespace}, testSecret).Error(), "not found")

	assert.NoError(t, r.reconcileClusterPermissionsSecret(a))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: testSecret.Name, Namespace: testSecret.Namespace}, testSecret))
	assert.Equal(t, string(testSecret.Data["namespaces"]), a.Namespace)

	want := "argocd,someRandomNamespace"
	testSecret.Data["namespaces"] = []byte("someRandomNamespace")
	r.Client.Update(context.TODO(), testSecret)

	// reconcile to check namespace with the label gets added
	assert.NoError(t, r.reconcileClusterPermissionsSecret(a))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: testSecret.Name, Namespace: testSecret.Namespace}, testSecret))
	assert.Equal(t, string(testSecret.Data["namespaces"]), want)

	assert.NoError(t, createNamespace(r, "xyz", a.Namespace))
	want = "argocd,someRandomNamespace,xyz"
	// reconcile to check namespace with the label gets added
	assert.NoError(t, r.reconcileClusterPermissionsSecret(a))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: testSecret.Name, Namespace: testSecret.Namespace}, testSecret))
	assert.Equal(t, string(testSecret.Data["namespaces"]), want)

	t.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", a.Namespace)

	assert.NoError(t, r.reconcileClusterPermissionsSecret(a))
	//assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: testSecret.Name, Namespace: testSecret.Namespace}, testSecret), "not found")
	//TODO: https://github.com/stretchr/testify/pull/1022 introduced ErrorContains, but is not yet available in a tagged release. Revert to ErrorContains once this becomes available
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: testSecret.Name, Namespace: testSecret.Namespace}, testSecret))
	assert.Nil(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: testSecret.Name, Namespace: testSecret.Namespace}, testSecret))
	assert.True(t, argoutil.IsTrackedByOperator(testSecret.ObjectMeta.Labels))
}

func TestGenerateSortedManagedNamespaceListForArgoCDCR(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(cr *argoproj.ArgoCD) {
		cr.Spec.NamespaceManagement = []argoproj.ManagedNamespaces{
			{
				Name:           "aaaa-second-when-sorted-nm",
				AllowManagedBy: true,
			},
			{
				Name:           "bbbb-third-when-sorted-nm",
				AllowManagedBy: true,
			},
		}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	// 1) The result of call should include both 'my-managed-namespace' and a.Namespace
	managedByNamespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "my-managed-namespace",
			Labels: map[string]string{common.ArgoCDManagedByLabel: a.Namespace},
		},
	}
	err := cl.Create(context.Background(), &managedByNamespace)
	assert.NoError(t, err)

	res, err := generateSortedManagedNamespaceListForArgoCDCR(a, r.Client)
	assert.NoError(t, err)
	assert.Equal(t, res, []string{a.Namespace, managedByNamespace.Name})

	// 2) Ensure that results returned by this function are sorted by namespace name
	err = cl.Delete(context.Background(), &managedByNamespace)
	assert.NoError(t, err)

	managedByNamespace = corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "aaaa-first-when-sorted",
			Labels: map[string]string{common.ArgoCDManagedByLabel: a.Namespace},
		},
	}
	err = cl.Create(context.Background(), &managedByNamespace)
	assert.NoError(t, err)

	res, err = generateSortedManagedNamespaceListForArgoCDCR(a, r.Client)
	assert.NoError(t, err)
	assert.Equal(t, res, []string{managedByNamespace.Name, a.Namespace})

	// If Namespace management is enabled, fetch from NamespaceManagement CRs
	// and Ensure that results returned by this function are sorted by namespace name
	os.Setenv(common.EnableManagedNamespace, "true")
	defer os.Unsetenv(common.EnableManagedNamespace)

	nsMgmt1 := argoproj.NamespaceManagement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespace-mgmt-1",
			Namespace: "aaaa-second-when-sorted-nm",
		},
		Spec: argoproj.NamespaceManagementSpec{
			ManagedBy: a.Namespace,
		},
	}

	nsMgmt2 := argoproj.NamespaceManagement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespace-mgmt-2",
			Namespace: "bbbb-third-when-sorted-nm",
		},
		Spec: argoproj.NamespaceManagementSpec{
			ManagedBy: a.Namespace,
		},
	}

	err = cl.Create(context.Background(), &nsMgmt1)
	assert.NoError(t, err)
	err = cl.Create(context.Background(), &nsMgmt2)
	assert.NoError(t, err)

	res, err = generateSortedManagedNamespaceListForArgoCDCR(a, r.Client)
	assert.NoError(t, err)
	assert.Equal(t, res, []string{managedByNamespace.Name, nsMgmt1.Namespace, a.Namespace, nsMgmt2.Namespace})

}

func TestGenerateSortedManagedNamespaceListForArgoCDCR1(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(cr *argoproj.ArgoCD) {
		cr.Spec.NamespaceManagement = []argoproj.ManagedNamespaces{
			{
				Name:           "aaaa-second-when-sorted-nm",
				AllowManagedBy: true,
			},
			{
				Name:           "bbbb-third-when-sorted-nm",
				AllowManagedBy: true,
			},
		}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	t.Run("should return default and managedBy namespaces", func(t *testing.T) {
		ns := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "my-managed-namespace",
				Labels: map[string]string{common.ArgoCDManagedByLabel: a.Namespace},
			},
		}
		err := cl.Create(context.Background(), &ns)
		assert.NoError(t, err)

		res, err := generateSortedManagedNamespaceListForArgoCDCR(a, r.Client)
		assert.NoError(t, err)
		assert.Equal(t, res, []string{a.Namespace, ns.Name})
	})

	t.Run("should return sorted managedBy namespaces", func(t *testing.T) {
		err := cl.Delete(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-managed-namespace"}})
		assert.NoError(t, err)

		ns := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "aaaa-first-when-sorted",
				Labels: map[string]string{common.ArgoCDManagedByLabel: a.Namespace},
			},
		}
		err = cl.Create(context.Background(), &ns)
		assert.NoError(t, err)

		res, err := generateSortedManagedNamespaceListForArgoCDCR(a, r.Client)
		assert.NoError(t, err)
		assert.Equal(t, res, []string{ns.Name, a.Namespace})
	})

	t.Run("should ignore NamespaceManagement when feature is disabled", func(t *testing.T) {
		nsMgmt1 := argoproj.NamespaceManagement{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "namespace-mgmt-1",
				Namespace: "aaaa-second-when-sorted-nm",
			},
			Spec: argoproj.NamespaceManagementSpec{
				ManagedBy: a.Namespace,
			},
		}
		nsMgmt2 := argoproj.NamespaceManagement{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "namespace-mgmt-2",
				Namespace: "bbbb-third-when-sorted-nm",
			},
			Spec: argoproj.NamespaceManagementSpec{
				ManagedBy: a.Namespace,
			},
		}
		err := cl.Create(context.Background(), &nsMgmt1)
		assert.NoError(t, err)
		err = cl.Create(context.Background(), &nsMgmt2)
		assert.NoError(t, err)

		res, err := generateSortedManagedNamespaceListForArgoCDCR(a, r.Client)
		assert.NoError(t, err)
		assert.NotContains(t, res, nsMgmt1.Namespace)
		assert.NotContains(t, res, nsMgmt2.Namespace)
	})

	t.Run("should ignore NamespaceManagement CRs with different managedBy", func(t *testing.T) {
		nsMgmtOther := argoproj.NamespaceManagement{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "namespace-mgmt-other",
				Namespace: "zzzz-should-not-be-included",
			},
			Spec: argoproj.NamespaceManagementSpec{
				ManagedBy: "some-other-namespace",
			},
		}
		err := cl.Create(context.Background(), &nsMgmtOther)
		assert.NoError(t, err)

		os.Setenv(common.EnableManagedNamespace, "true")
		defer os.Unsetenv(common.EnableManagedNamespace)

		res, err := generateSortedManagedNamespaceListForArgoCDCR(a, r.Client)
		assert.NoError(t, err)
		assert.NotContains(t, res, nsMgmtOther.Namespace)
	})

	t.Run("should return all sorted namespaces including NamespaceManagement when feature enabled", func(t *testing.T) {
		os.Setenv(common.EnableManagedNamespace, "true")
		defer os.Unsetenv(common.EnableManagedNamespace)

		res, err := generateSortedManagedNamespaceListForArgoCDCR(a, r.Client)
		assert.NoError(t, err)
		assert.Equal(t, res, []string{
			"aaaa-first-when-sorted",     // from label
			"aaaa-second-when-sorted-nm", // from nsMgmt1
			a.Namespace,                  // default
			"bbbb-third-when-sorted-nm",  // from nsMgmt2
		})
	})
}

func TestCombineClusterSecretNamespacesWithManagedNamespaces(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	// 1) Namespaces already listed in the cluster secret should be preserved
	res := combineClusterSecretNamespacesWithManagedNamespaces(corev1.Secret{
		Data: map[string][]byte{
			"namespaces": ([]byte)("a,b"),
		},
	}, []string{})
	assert.Equal(t, "a,b", res)

	// 2) Duplicates between managed namespaces and existing namespaces should be removed
	res = combineClusterSecretNamespacesWithManagedNamespaces(corev1.Secret{
		Data: map[string][]byte{
			"namespaces": ([]byte)("a,b"),
		},
	}, []string{"b", "c"})
	assert.Equal(t, "a,b,c", res)

	// 3) Namespace list should be fully sorted by name
	res = combineClusterSecretNamespacesWithManagedNamespaces(corev1.Secret{
		Data: map[string][]byte{
			"namespaces": ([]byte)("b,a"),
		},
	}, []string{"a", "d", "c", "b"})
	assert.Equal(t, "a,b,c,d", res)

	// 4) Remove duplicates in Secret
	res = combineClusterSecretNamespacesWithManagedNamespaces(corev1.Secret{
		Data: map[string][]byte{
			"namespaces": ([]byte)("b,a,b,a"),
		},
	}, []string{"c", "d"})
	assert.Equal(t, "a,b,c,d", res)

	// 5) Remove duplicates in string list
	res = combineClusterSecretNamespacesWithManagedNamespaces(corev1.Secret{
		Data: map[string][]byte{
			"namespaces": ([]byte)("a,b"),
		},
	}, []string{"c", "d", "e", "c", "d"})
	assert.Equal(t, "a,b,c,d,e", res)

}
