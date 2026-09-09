package argocd

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

func testOwnerRef(cr *argoproj.ArgoCD) metav1.OwnerReference {
	t := true
	return metav1.OwnerReference{
		APIVersion:         "argoproj.io/v1beta1",
		Kind:               "ArgoCD",
		Name:               cr.Name,
		UID:                cr.UID,
		Controller:         &t,
		BlockOwnerDeletion: &t,
	}
}

func setOperatorNamespace(t *testing.T, ns string) {
	t.Helper()
	// The in-cluster namespace file does not exist during unit tests, so
	// GetOperatorNamespace falls back to ARGOCD_OPERATOR_NAMESPACE.
	t.Setenv("ARGOCD_OPERATOR_NAMESPACE", ns)
}

func TestReconcileImagePullSecrets_CopiesFromOperatorNamespace(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD()

	srcSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pull-secret",
			Namespace: operatorNS,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{}}`),
		},
	}

	operatorNSObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: operatorNS}}
	crNSObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}

	resObjs := []client.Object{cr, srcSecret, operatorNSObj, crNSObj}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	err := r.reconcileImagePullSecrets(cr)
	assert.NoError(t, err)

	copied := &corev1.Secret{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: "my-pull-secret", Namespace: testNamespace}, copied)
	assert.NoError(t, err)
	assert.Equal(t, srcSecret.Data, copied.Data)
	assert.Equal(t, corev1.SecretTypeDockerConfigJson, copied.Type)
	assert.Equal(t, "my-pull-secret", copied.Labels[common.ArgoCDImagePullSecretCopiedLabel])
}

func TestReconcileImagePullSecrets_SkipsWhenLabelIsFalse(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD()

	disabledSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "disabled-pull-secret",
			Namespace: operatorNS,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "false",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{}}`),
		},
	}

	operatorNSObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: operatorNS}}
	crNSObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}

	resObjs := []client.Object{cr, disabledSecret, operatorNSObj, crNSObj}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	err := r.reconcileImagePullSecrets(cr)
	assert.NoError(t, err)

	copied := &corev1.Secret{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: "disabled-pull-secret", Namespace: testNamespace}, copied)
	assert.Error(t, err, "secret with label=false should not be copied")
}

func TestReconcileImagePullSecrets_SkipsWhenSameNamespace(t *testing.T) {
	setOperatorNamespace(t, testNamespace)

	cr := makeTestArgoCD()

	srcSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pull-secret",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{}}`),
		},
	}

	resObjs := []client.Object{cr, srcSecret}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	err := r.reconcileImagePullSecrets(cr)
	assert.NoError(t, err)

	secrets := &corev1.SecretList{}
	err = cl.List(context.TODO(), secrets, client.InNamespace(testNamespace), client.HasLabels{common.ArgoCDImagePullSecretCopiedLabel})
	assert.NoError(t, err)
	assert.Empty(t, secrets.Items)
}

func TestReconcileImagePullSecrets_DeletesStaleCopies(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD()
	cr.UID = "test-uid-1"

	staleCopy := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "old-pull-secret",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretCopiedLabel: "old-pull-secret",
				common.ArgoCDTrackedByOperatorLabel:     common.ArgoCDAppName,
			},
			OwnerReferences: []metav1.OwnerReference{testOwnerRef(cr)},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{}}`),
		},
	}

	operatorNSObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: operatorNS}}
	crNSObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}

	resObjs := []client.Object{cr, staleCopy, operatorNSObj, crNSObj}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	err := r.reconcileImagePullSecrets(cr)
	assert.NoError(t, err)

	deleted := &corev1.Secret{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: "old-pull-secret", Namespace: testNamespace}, deleted)
	assert.Error(t, err)
}

func TestReconcileImagePullSecrets_UpdatesCopyWhenDataChanges(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD()
	cr.UID = "test-uid-1"

	srcSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pull-secret",
			Namespace: operatorNS,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{"new":"data"}}`),
		},
	}

	existingCopy := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pull-secret",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretCopiedLabel: "my-pull-secret",
				common.ArgoCDTrackedByOperatorLabel:     common.ArgoCDAppName,
			},
			OwnerReferences: []metav1.OwnerReference{testOwnerRef(cr)},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{"old":"data"}}`),
		},
	}

	operatorNSObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: operatorNS}}
	crNSObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}

	resObjs := []client.Object{cr, srcSecret, existingCopy, operatorNSObj, crNSObj}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	err := r.reconcileImagePullSecrets(cr)
	assert.NoError(t, err)

	updated := &corev1.Secret{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: "my-pull-secret", Namespace: testNamespace}, updated)
	assert.NoError(t, err)
	assert.Equal(t, srcSecret.Data, updated.Data)
}

func TestReconcileImagePullSecrets_SkipsNonOperatorSecret(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD()

	srcSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-secret",
			Namespace: operatorNS,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{}}`),
		},
	}

	existingNonOperator := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-secret",
			Namespace: testNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"key": []byte("original-data"),
		},
	}

	operatorNSObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: operatorNS}}
	crNSObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}

	resObjs := []client.Object{cr, srcSecret, existingNonOperator, operatorNSObj, crNSObj}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	err := r.reconcileImagePullSecrets(cr)
	assert.NoError(t, err)

	unchanged := &corev1.Secret{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: "existing-secret", Namespace: testNamespace}, unchanged)
	assert.NoError(t, err)
	assert.Equal(t, []byte("original-data"), unchanged.Data["key"])
	assert.Equal(t, corev1.SecretTypeOpaque, unchanged.Type)
}

func TestGetImagePullSecretRefs_ReturnsCopiedSecrets(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD()

	copiedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pull-secret",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretCopiedLabel: "my-pull-secret",
			},
			OwnerReferences: []metav1.OwnerReference{testOwnerRef(cr)},
		},
	}

	resObjs := []client.Object{cr, copiedSecret}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	refs, err := r.getImagePullSecretRefs(cr)
	require.NoError(t, err)
	assert.Len(t, refs, 1)
	assert.Equal(t, "my-pull-secret", refs[0].Name)
}

func TestGetImagePullSecretRefs_ReturnsInNamespaceSecrets(t *testing.T) {
	setOperatorNamespace(t, testNamespace)

	cr := makeTestArgoCD()

	labeledSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pull-secret",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
	}

	resObjs := []client.Object{cr, labeledSecret}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	refs, err := r.getImagePullSecretRefs(cr)
	require.NoError(t, err)
	assert.Len(t, refs, 1)
	assert.Equal(t, "my-pull-secret", refs[0].Name)
}

func TestGetImagePullSecretRefs_ReturnsEmptyWhenNone(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	refs, err := r.getImagePullSecretRefs(cr)
	require.NoError(t, err)
	assert.Empty(t, refs)
}

func TestGetImagePullSecretRefs_SortedByName(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD()

	secretB := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "b-secret",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretCopiedLabel: "b-secret",
			},
			OwnerReferences: []metav1.OwnerReference{testOwnerRef(cr)},
		},
	}

	secretA := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a-secret",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretCopiedLabel: "a-secret",
			},
			OwnerReferences: []metav1.OwnerReference{testOwnerRef(cr)},
		},
	}

	resObjs := []client.Object{cr, secretB, secretA}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	refs, err := r.getImagePullSecretRefs(cr)
	require.NoError(t, err)
	assert.Len(t, refs, 2)
	assert.Equal(t, "a-secret", refs[0].Name)
	assert.Equal(t, "b-secret", refs[1].Name)
}

func TestReconcileServiceAccount_SetsImagePullSecrets(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD()

	copiedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pull-secret",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretCopiedLabel: "my-pull-secret",
			},
			OwnerReferences: []metav1.OwnerReference{testOwnerRef(cr)},
		},
	}

	resObjs := []client.Object{cr, copiedSecret}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	sa, err := r.reconcileServiceAccount(common.ArgoCDServerComponent, cr)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	retrieved := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: sa.Name, Namespace: testNamespace}, retrieved)
	assert.NoError(t, err)
	assert.Len(t, retrieved.ImagePullSecrets, 1)
	assert.Equal(t, "my-pull-secret", retrieved.ImagePullSecrets[0].Name)
}

func TestReconcileServiceAccount_UpdatesImagePullSecrets(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD()

	existingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getServiceAccountName(cr.Name, common.ArgoCDServerComponent),
			Namespace: testNamespace,
			Labels:    argoutil.LabelsForCluster(cr),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "argoproj.io/v1beta1",
					Kind:       "ArgoCD",
					Name:       cr.Name,
					UID:        cr.UID,
				},
			},
		},
	}

	copiedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "new-pull-secret",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretCopiedLabel: "new-pull-secret",
			},
			OwnerReferences: []metav1.OwnerReference{testOwnerRef(cr)},
		},
	}

	resObjs := []client.Object{cr, existingSA, copiedSecret}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	sa, err := r.reconcileServiceAccount(common.ArgoCDServerComponent, cr)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	retrieved := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: sa.Name, Namespace: testNamespace}, retrieved)
	assert.NoError(t, err)
	assert.Len(t, retrieved.ImagePullSecrets, 1)
	assert.Equal(t, "new-pull-secret", retrieved.ImagePullSecrets[0].Name)
}

func TestReconcileServiceAccount_ClearsImagePullSecrets(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD()

	existingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getServiceAccountName(cr.Name, common.ArgoCDServerComponent),
			Namespace: testNamespace,
			Labels:    argoutil.LabelsForCluster(cr),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "argoproj.io/v1beta1",
					Kind:       "ArgoCD",
					Name:       cr.Name,
					UID:        cr.UID,
				},
			},
		},
		ImagePullSecrets: []corev1.LocalObjectReference{
			{Name: "old-secret"},
		},
	}

	resObjs := []client.Object{cr, existingSA}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	sa, err := r.reconcileServiceAccount(common.ArgoCDServerComponent, cr)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	retrieved := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: sa.Name, Namespace: testNamespace}, retrieved)
	assert.NoError(t, err)
	assert.Empty(t, retrieved.ImagePullSecrets)
}

func TestImagePullSecretMapper_EnqueuesAllArgoCDInstances(t *testing.T) {
	cr1 := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{Name: "argocd-1", Namespace: "ns-1"},
	}
	cr2 := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{Name: "argocd-2", Namespace: "ns-2"},
	}

	resObjs := []client.Object{cr1, cr2}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	labeledSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-secret",
			Namespace: "operator-ns",
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
	}

	requests := r.imagePullSecretMapper(context.TODO(), labeledSecret)
	assert.Len(t, requests, 2)

	names := map[string]string{}
	for _, req := range requests {
		names[req.Namespace] = req.Name
	}
	assert.Equal(t, "argocd-1", names["ns-1"])
	assert.Equal(t, "argocd-2", names["ns-2"])
}

// Mapper enqueues unconditionally — predicate handles filtering.
// Label true→false appears as delete event; mapper must still enqueue for stale-copy cleanup.
func TestImagePullSecretMapper_EnqueuesOnDeleteEvent(t *testing.T) {
	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	deletedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "removed-secret",
			Namespace: "operator-ns",
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
	}

	requests := r.imagePullSecretMapper(context.TODO(), deletedSecret)
	assert.Len(t, requests, 1)
}

func TestReconcileImagePullSecrets_SkipsWhenMultipleLabeled(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD()

	secret1 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-secret-1",
			Namespace: operatorNS,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{".dockerconfigjson": []byte(`{"auths":{}}`)},
	}

	secret2 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-secret-2",
			Namespace: operatorNS,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{".dockerconfigjson": []byte(`{"auths":{}}`)},
	}

	operatorNSObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: operatorNS}}
	crNSObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}

	resObjs := []client.Object{cr, secret1, secret2, operatorNSObj, crNSObj}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	err := r.reconcileImagePullSecrets(cr)
	assert.NoError(t, err)

	copied := &corev1.SecretList{}
	err = cl.List(context.TODO(), copied, client.InNamespace(testNamespace), client.HasLabels{common.ArgoCDImagePullSecretCopiedLabel})
	assert.NoError(t, err)
	assert.Empty(t, copied.Items, "no secrets should be copied when multiple labeled secrets exist")
}

func TestReconcileImagePullSecrets_CleansUpStaleWhenMultipleLabeled(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD()
	cr.UID = "test-uid-1"

	staleCopy := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "old-pull-secret",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretCopiedLabel: "old-pull-secret",
				common.ArgoCDTrackedByOperatorLabel:     common.ArgoCDAppName,
			},
			OwnerReferences: []metav1.OwnerReference{testOwnerRef(cr)},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{".dockerconfigjson": []byte(`{"auths":{}}`)},
	}

	secret1 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-secret-1",
			Namespace: operatorNS,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{".dockerconfigjson": []byte(`{"auths":{}}`)},
	}

	secret2 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-secret-2",
			Namespace: operatorNS,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{".dockerconfigjson": []byte(`{"auths":{}}`)},
	}

	operatorNSObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: operatorNS}}
	crNSObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}

	resObjs := []client.Object{cr, staleCopy, secret1, secret2, operatorNSObj, crNSObj}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	err := r.reconcileImagePullSecrets(cr)
	assert.NoError(t, err)

	copied := &corev1.SecretList{}
	err = cl.List(context.TODO(), copied, client.InNamespace(testNamespace), client.HasLabels{common.ArgoCDImagePullSecretCopiedLabel})
	assert.NoError(t, err)
	assert.Empty(t, copied.Items, "stale copies must be cleaned up even when multiple labeled secrets cause propagation skip")
}

func TestGetImagePullSecretRefs_SkipsWhenMultipleLabeledInNamespace(t *testing.T) {
	setOperatorNamespace(t, testNamespace)

	cr := makeTestArgoCD()

	secret1 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-secret-1",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
	}

	secret2 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-secret-2",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
	}

	resObjs := []client.Object{cr, secret1, secret2}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	refs, err := r.getImagePullSecretRefs(cr)
	require.NoError(t, err)
	assert.Empty(t, refs, "no refs should be returned when multiple labeled secrets exist")
}

func TestGetImagePullSecretRefs_ErrorOnInNamespaceListFailure(t *testing.T) {
	setOperatorNamespace(t, testNamespace)

	cr := makeTestArgoCD()
	listErr := fmt.Errorf("simulated API error")

	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := fake.NewClientBuilder().WithScheme(sch).WithObjects(cr).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if _, ok := list.(*corev1.SecretList); ok {
					return listErr
				}
				return c.List(ctx, list, opts...)
			},
		}).Build()
	r := makeTestReconciler(cl, sch, nil)

	refs, err := r.getImagePullSecretRefs(cr)
	require.ErrorIs(t, err, listErr)
	assert.Nil(t, refs)
}

func TestGetImagePullSecretRefs_ErrorOnCopiedSecretListFailure(t *testing.T) {
	setOperatorNamespace(t, testNamespace)

	cr := makeTestArgoCD()
	listErr := fmt.Errorf("simulated API error")
	callCount := 0

	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := fake.NewClientBuilder().WithScheme(sch).WithObjects(cr).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if _, ok := list.(*corev1.SecretList); ok {
					callCount++
					if callCount == 1 {
						return c.List(ctx, list, opts...)
					}
					return listErr
				}
				return c.List(ctx, list, opts...)
			},
		}).Build()
	r := makeTestReconciler(cl, sch, nil)

	refs, err := r.getImagePullSecretRefs(cr)
	require.ErrorIs(t, err, listErr)
	assert.Nil(t, refs)
}

func TestReconcileImagePullSecrets_PreservesCopiedLabelSecretNotOwnedByThisCR(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD()
	cr.UID = "test-uid-1"

	srcSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-secret",
			Namespace: operatorNS,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{".dockerconfigjson": []byte(`{"auths":{"new":"creds"}}`)},
	}

	otherOwner := metav1.OwnerReference{
		APIVersion: "argoproj.io/v1beta1",
		Kind:       "ArgoCD",
		Name:       "other-argocd",
		UID:        "other-uid-999",
	}
	existingNotOwned := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-secret",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretCopiedLabel: "pull-secret",
			},
			OwnerReferences: []metav1.OwnerReference{otherOwner},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{".dockerconfigjson": []byte(`{"auths":{"old":"creds"}}`)},
	}

	operatorNSObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: operatorNS}}
	crNSObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}

	resObjs := []client.Object{cr, srcSecret, existingNotOwned, operatorNSObj, crNSObj}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	err := r.reconcileImagePullSecrets(cr)
	assert.NoError(t, err)

	preserved := &corev1.Secret{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: "pull-secret", Namespace: testNamespace}, preserved)
	assert.NoError(t, err)
	assert.Equal(t, []byte(`{"auths":{"old":"creds"}}`), preserved.Data[".dockerconfigjson"],
		"secret owned by another ArgoCD instance must not be updated")
}

func TestImagePullSecretPredicate_AcceptsOperatorNamespace(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, nil, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	pred := r.imagePullSecretFilterPredicate()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-secret",
			Namespace: operatorNS,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
	}

	assert.True(t, pred.Create(event.CreateEvent{Object: secret}))
	assert.True(t, pred.Delete(event.DeleteEvent{Object: secret}))
}

func TestImagePullSecretPredicate_RejectsOtherNamespace(t *testing.T) {
	setOperatorNamespace(t, "operator-ns")

	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, nil, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	pred := r.imagePullSecretFilterPredicate()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-secret",
			Namespace: "some-other-ns",
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
	}

	assert.False(t, pred.Create(event.CreateEvent{Object: secret}))
	assert.False(t, pred.Delete(event.DeleteEvent{Object: secret}))
}

func TestImagePullSecretPredicate_RejectsUnlabeledSecret(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, nil, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	pred := r.imagePullSecretFilterPredicate()

	oldSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unrelated-secret",
			Namespace: operatorNS,
		},
	}
	newSecret := oldSecret.DeepCopy()

	assert.False(t, pred.Update(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: newSecret}))
}

func TestImagePullSecretPredicate_AcceptsLabelTrueToFalse(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, nil, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	pred := r.imagePullSecretFilterPredicate()

	oldSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-secret",
			Namespace: operatorNS,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
	}
	newSecret := oldSecret.DeepCopy()
	newSecret.Labels[common.ArgoCDImagePullSecretPropagateLabel] = "false"

	assert.True(t, pred.Update(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: newSecret}),
		"must accept true→false transition for cleanup")
}

func TestImagePullSecretPredicate_AcceptsLabelFalseToTrue(t *testing.T) {
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, nil, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	pred := r.imagePullSecretFilterPredicate()

	oldSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-secret",
			Namespace: operatorNS,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "false",
			},
		},
	}
	newSecret := oldSecret.DeepCopy()
	newSecret.Labels[common.ArgoCDImagePullSecretPropagateLabel] = "true"

	assert.True(t, pred.Update(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: newSecret}),
		"must accept false→true transition for propagation")
}

// --- OpenShift guards: SA ImagePullSecrets must not be touched ---

func setOpenShift(t *testing.T) {
	t.Helper()
	original := versionAPIFound
	versionAPIFound = true
	t.Cleanup(func() { versionAPIFound = original })
}

func TestReconcileServiceAccount_OpenShift_DoesNotSetImagePullSecrets(t *testing.T) {
	setOpenShift(t)
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD()

	copiedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pull-secret",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretCopiedLabel: "my-pull-secret",
			},
			OwnerReferences: []metav1.OwnerReference{testOwnerRef(cr)},
		},
	}

	resObjs := []client.Object{cr, copiedSecret}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	sa, err := r.reconcileServiceAccount(common.ArgoCDServerComponent, cr)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	retrieved := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: sa.Name, Namespace: testNamespace}, retrieved)
	assert.NoError(t, err)
	assert.Empty(t, retrieved.ImagePullSecrets,
		"on OpenShift, newly created SA must not have operator-managed imagePullSecrets")
}

func TestReconcileServiceAccount_OpenShift_DoesNotOverwriteExistingSecrets(t *testing.T) {
	setOpenShift(t)
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD()

	existingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getServiceAccountName(cr.Name, common.ArgoCDServerComponent),
			Namespace: testNamespace,
			Labels:    argoutil.LabelsForCluster(cr),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "argoproj.io/v1beta1",
					Kind:       "ArgoCD",
					Name:       cr.Name,
					UID:        cr.UID,
				},
			},
		},
		ImagePullSecrets: []corev1.LocalObjectReference{
			{Name: "openshift-dockercfg-abc123"},
		},
	}

	resObjs := []client.Object{cr, existingSA}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	sa, err := r.reconcileServiceAccount(common.ArgoCDServerComponent, cr)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	retrieved := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: sa.Name, Namespace: testNamespace}, retrieved)
	assert.NoError(t, err)
	assert.Len(t, retrieved.ImagePullSecrets, 1)
	assert.Equal(t, "openshift-dockercfg-abc123", retrieved.ImagePullSecrets[0].Name,
		"on OpenShift, operator must not overwrite platform-injected imagePullSecrets")
}

func TestReconcileImagePullSecrets_OpenShift_SkipsReconciliation(t *testing.T) {
	setOpenShift(t)
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD()

	srcSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pull-secret",
			Namespace: operatorNS,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretPropagateLabel: "true",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{".dockerconfigjson": []byte(`{"auths":{}}`)},
	}

	resObjs := []client.Object{cr, srcSecret}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	assert.True(t, IsOpenShiftCluster(), "OpenShift detection must be active for this test")

	err := r.reconcileImagePullSecrets(cr)
	assert.NoError(t, err)

	copiedSecrets := &corev1.SecretList{}
	err = cl.List(context.TODO(), copiedSecrets,
		client.InNamespace(testNamespace),
		client.HasLabels{common.ArgoCDImagePullSecretCopiedLabel})
	assert.NoError(t, err)
	assert.Empty(t, copiedSecrets.Items, "on OpenShift, operator must not copy image pull secrets to ArgoCD namespace")
}

func TestReconcileApplicationSetServiceAccount_OpenShift_DoesNotSetImagePullSecrets(t *testing.T) {
	setOpenShift(t)
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}
	})

	copiedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pull-secret",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretCopiedLabel: "my-pull-secret",
			},
			OwnerReferences: []metav1.OwnerReference{testOwnerRef(cr)},
		},
	}

	resObjs := []client.Object{cr, copiedSecret}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	sa, err := r.reconcileApplicationSetServiceAccount(cr)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	retrieved := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: sa.Name, Namespace: testNamespace}, retrieved)
	assert.NoError(t, err)
	assert.Empty(t, retrieved.ImagePullSecrets,
		"on OpenShift, applicationset SA must not have operator-managed imagePullSecrets")
}

func TestReconcileImageUpdaterServiceAccount_OpenShift_DoesNotSetImagePullSecrets(t *testing.T) {
	setOpenShift(t)
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})

	copiedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pull-secret",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.ArgoCDImagePullSecretCopiedLabel: "my-pull-secret",
			},
			OwnerReferences: []metav1.OwnerReference{testOwnerRef(cr)},
		},
	}

	resObjs := []client.Object{cr, copiedSecret}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	sa, err := r.reconcileImageUpdaterServiceAccount(cr)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	retrieved := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: sa.Name, Namespace: testNamespace}, retrieved)
	assert.NoError(t, err)
	assert.Empty(t, retrieved.ImagePullSecrets,
		"on OpenShift, image-updater SA must not have operator-managed imagePullSecrets")
}

func TestReconcileApplicationSetServiceAccount_OpenShift_DoesNotOverwriteExistingSecrets(t *testing.T) {
	setOpenShift(t)
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}
	})

	existingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getServiceAccountName(cr.Name, "applicationset-controller"),
			Namespace: testNamespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
		ImagePullSecrets: []corev1.LocalObjectReference{
			{Name: "openshift-dockercfg-abc123"},
		},
	}

	resObjs := []client.Object{cr, existingSA}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	sa, err := r.reconcileApplicationSetServiceAccount(cr)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	retrieved := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: sa.Name, Namespace: testNamespace}, retrieved)
	assert.NoError(t, err)
	assert.Len(t, retrieved.ImagePullSecrets, 1)
	assert.Equal(t, "openshift-dockercfg-abc123", retrieved.ImagePullSecrets[0].Name,
		"on OpenShift, operator must not overwrite platform-injected imagePullSecrets on applicationset SA")
}

func TestReconcileImageUpdaterServiceAccount_OpenShift_DoesNotOverwriteExistingSecrets(t *testing.T) {
	setOpenShift(t)
	operatorNS := "operator-ns"
	setOperatorNamespace(t, operatorNS)

	cr := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})

	existingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getServiceAccountName(cr.Name, common.ArgoCDImageUpdaterControllerComponent),
			Namespace: testNamespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
		ImagePullSecrets: []corev1.LocalObjectReference{
			{Name: "openshift-dockercfg-abc123"},
		},
	}

	resObjs := []client.Object{cr, existingSA}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, nil, nil)
	r := makeTestReconciler(cl, sch, nil)

	sa, err := r.reconcileImageUpdaterServiceAccount(cr)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	retrieved := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: sa.Name, Namespace: testNamespace}, retrieved)
	assert.NoError(t, err)
	assert.Len(t, retrieved.ImagePullSecrets, 1)
	assert.Equal(t, "openshift-dockercfg-abc123", retrieved.ImagePullSecrets[0].Name,
		"on OpenShift, operator must not overwrite platform-injected imagePullSecrets on image-updater SA")
}
