package argocd

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

func Test_ArgoCDReconciler_ReconcileRepoTLSSecret(t *testing.T) {
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
		service := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-repo-server",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "argoproj.io/v1beta1",
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
		objs := []runtime.Object{
			argocd,
			secret,
			service,
			serverDepl,
			repoDepl,
			ctrlSts,
		}

		r := makeReconciler(t, argocd, objs...)

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

func Test_ArgoCDReconciler_ReconcileRedisTLSSecret(t *testing.T) {
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
		service := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-redis",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "argoproj.io/v1beta1",
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
		objs := []runtime.Object{
			argocd,
			secret,
			service,
			serverDepl,
			repoDepl,
			redisDepl,
			ctrlSts,
		}

		r := makeReconciler(t, argocd, objs...)

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
