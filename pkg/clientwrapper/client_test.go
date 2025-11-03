package clientwrapper

// Assisted by : Gemini 2.5 Pro
// The unit tests in this file were generated with the assistance of Google's Gemini 2.5 Pro.

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj-labs/argocd-operator/common"
)

// recordingClient wraps a client and notes whether Get/Patch were called.
// Used for the live client to verify fallback behavior.
type recordingClient struct {
	ctrlclient.Client
	getCalls   int
	patchCalls int
}

func (r *recordingClient) Get(ctx context.Context, key ctrlclient.ObjectKey, obj ctrlclient.Object, opts ...ctrlclient.GetOption) error {
	r.getCalls++
	return r.Client.Get(ctx, key, obj, opts...)
}

func (r *recordingClient) Patch(ctx context.Context, obj ctrlclient.Object, patch ctrlclient.Patch, opts ...ctrlclient.PatchOption) error {
	r.patchCalls++
	return r.Client.Patch(ctx, obj, patch, opts...)
}

func testScheme(t *testing.T) *runtime.Scheme {
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	return s
}

func TestClientWrapper_Get_Secret_Stripped_ShouldLiveRefreshAndLabel(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	// 1) Cached client has a *stripped* Secret (no data, no labels)
	cachedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
		},
	}

	cachedClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cachedSecret).
		Build()

	// 2) Live client has the *real* Secret, with data
	liveSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"token": []byte("real-token"),
		},
	}
	liveBase := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(liveSecret).
		Build()
	liveClient := &recordingClient{Client: liveBase}

	wrapper := NewClientWrapper(cachedClient, liveClient)

	var got corev1.Secret
	err := wrapper.Get(ctx, types.NamespacedName{Name: "my-secret", Namespace: "default"}, &got)
	if err != nil {
		t.Fatalf("wrapper.Get failed: %v", err)
	}

	// Should have fallen back to live client
	if liveClient.getCalls == 0 {
		t.Fatalf("expected live client Get to be called, but it was not")
	}

	// Should have patched to add tracking label
	if liveClient.patchCalls == 0 {
		t.Fatalf("expected live client Patch to be called to add tracking label, but it was not")
	}

	// The returned object should now have data (live version)
	if len(got.Data) == 0 {
		t.Fatalf("expected secret data to be populated from live client, got empty")
	}

	labels := got.GetLabels()
	if labels == nil {
		t.Fatalf("expected labels to be set on secret")
	}
	if labels[common.ArgoCDTrackedByOperatorLabel] != common.ArgoCDAppName {
		t.Fatalf("expected tracking label %q=%q, got=%v",
			common.ArgoCDTrackedByOperatorLabel, common.ArgoCDAppName, labels)
	}
}

func TestClientWrapper_Get_Secret_AlreadyTracked_NoLiveRefresh(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	// Cached client has a *fully tracked* secret
	cachedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tracked-secret",
			Namespace: "default",
			Labels: map[string]string{
				common.ArgoCDTrackedByOperatorLabel: common.ArgoCDAppName,
			},
		},
		Data: map[string][]byte{
			"key": []byte("cached"),
		},
	}

	cachedClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cachedSecret).
		Build()

	// Live client should not be called in this scenario
	liveBase := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()
	liveClient := &recordingClient{Client: liveBase}

	wrapper := NewClientWrapper(cachedClient, liveClient)

	var got corev1.Secret
	err := wrapper.Get(ctx, types.NamespacedName{Name: "tracked-secret", Namespace: "default"}, &got)
	if err != nil {
		t.Fatalf("wrapper.Get failed: %v", err)
	}

	// Should NOT have fallen back to live client
	if liveClient.getCalls != 0 {
		t.Fatalf("expected live client Get NOT to be called, but was called %d times", liveClient.getCalls)
	}

	// Data should be what we had in cache
	if string(got.Data["key"]) != "cached" {
		t.Fatalf("expected cached data to be returned, got %v", got.Data)
	}

	// Label should remain
	if got.Labels[common.ArgoCDTrackedByOperatorLabel] != common.ArgoCDAppName {
		t.Fatalf("expected tracking label to be preserved")
	}
}

func TestClientWrapper_Get_ConfigMap_Stripped_ShouldLiveRefreshAndLabel(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	// 1) Cached has a stripped CM (nil Data & BinaryData, no tracking label)
	cachedCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cm",
			Namespace: "default",
		},
		// Data: nil, BinaryData: nil => stripped
	}
	cachedClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cachedCM).
		Build()

	// 2) Live has the real CM with data/binaryData
	liveCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cm",
			Namespace: "default",
		},
		Data: map[string]string{
			"key": "value",
		},
		BinaryData: map[string][]byte{
			"bin": []byte{0x1, 0x2},
		},
	}
	liveBase := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(liveCM).
		Build()
	liveClient := &recordingClient{Client: liveBase}

	wrapper := NewClientWrapper(cachedClient, liveClient)

	var got corev1.ConfigMap
	err := wrapper.Get(ctx, types.NamespacedName{Name: "my-cm", Namespace: "default"}, &got)
	if err != nil {
		t.Fatalf("wrapper.Get failed: %v", err)
	}

	// Should fall back to live
	if liveClient.getCalls == 0 {
		t.Fatalf("expected live client Get to be called")
	}
	// Should attempt to add tracking label
	if liveClient.patchCalls == 0 {
		t.Fatalf("expected live client Patch to be called to add tracking label")
	}

	// Data copied from live
	if got.Data["key"] != "value" || len(got.BinaryData["bin"]) != 2 {
		t.Fatalf("expected data/binaryData to be populated from live client, got: data=%v, bin=%v", got.Data, got.BinaryData)
	}
	if got.Labels[common.ArgoCDTrackedByOperatorLabel] != common.ArgoCDAppName {
		t.Fatalf("expected tracking label to be set")
	}
}

func TestClientWrapper_Get_ConfigMap_AlreadyTracked_NoLiveRefresh(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	cachedCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tracked-cm",
			Namespace: "default",
			Labels: map[string]string{
				common.ArgoCDTrackedByOperatorLabel: common.ArgoCDAppName,
			},
		},
		Data: map[string]string{"a": "b"},
	}
	cachedClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cachedCM).
		Build()

	liveBase := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()
	liveClient := &recordingClient{Client: liveBase}

	wrapper := NewClientWrapper(cachedClient, liveClient)

	var got corev1.ConfigMap
	err := wrapper.Get(ctx, types.NamespacedName{Name: "tracked-cm", Namespace: "default"}, &got)
	if err != nil {
		t.Fatalf("wrapper.Get failed: %v", err)
	}

	// No fallback expected
	if liveClient.getCalls != 0 || liveClient.patchCalls != 0 {
		t.Fatalf("did not expect live client calls; got Get=%d Patch=%d", liveClient.getCalls, liveClient.patchCalls)
	}

	if got.Data["a"] != "b" {
		t.Fatalf("expected cached data to be returned, got: %v", got.Data)
	}
	if got.Labels[common.ArgoCDTrackedByOperatorLabel] != common.ArgoCDAppName {
		t.Fatalf("expected tracking label to be preserved")
	}
}

func TestClientWrapper_Get_OtherKinds_NoLiveRefresh(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	// Use a core kind not specially handled by the wrapper, e.g., Service
	cachedSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "plain-svc",
			Namespace: "default",
			// no labels on purpose
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.0.0.1",
		},
	}
	cachedClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cachedSvc).
		Build()

	// Create a different "live" version to prove we never read it
	liveSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "plain-svc",
			Namespace: "default",
			Labels: map[string]string{
				"live": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.9.9.9",
		},
	}
	liveBase := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(liveSvc).
		Build()
	liveClient := &recordingClient{Client: liveBase}

	wrapper := NewClientWrapper(cachedClient, liveClient)

	var got corev1.Service
	err := wrapper.Get(ctx, types.NamespacedName{Name: "plain-svc", Namespace: "default"}, &got)
	if err != nil {
		t.Fatalf("wrapper.Get failed: %v", err)
	}

	// Because Service isn't handled in switch, live should NOT be called
	if liveClient.getCalls != 0 || liveClient.patchCalls != 0 {
		t.Fatalf("did not expect live client calls for non-Secret/ConfigMap; got Get=%d Patch=%d", liveClient.getCalls, liveClient.patchCalls)
	}

	// We should get the cached version's ClusterIP, not live's
	if got.Spec.ClusterIP != "10.0.0.1" {
		t.Fatalf("expected cached service spec (ClusterIP=10.0.0.1), got %s", got.Spec.ClusterIP)
	}
	// And labels should be from cache (none), not live
	if len(got.GetLabels()) != 0 {
		t.Fatalf("expected no labels from cached service, got: %v", got.GetLabels())
	}
}
