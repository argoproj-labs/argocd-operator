package clientwrapper

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/cacheutils"
)

// ClientWrapper wraps a cached client and only falls back to
// live GET when the cached object appears stripped OR is missing required labels.
type ClientWrapper struct {
	ctrlclient.Client                   // cached client
	liveClient        ctrlclient.Client // direct API client
}

func NewClientWrapper(cached, live ctrlclient.Client) *ClientWrapper {
	return &ClientWrapper{
		Client:     cached,
		liveClient: live,
	}
}

func (cw *ClientWrapper) Get(ctx context.Context, key types.NamespacedName, obj ctrlclient.Object, opts ...ctrlclient.GetOption) error {
	// 1) Cache read (no live fallback here on error)
	if err := cw.Client.Get(ctx, key, obj, opts...); err != nil {
		return err
	}

	// 2) Post-read check: only specific kinds may need live refresh
	switch o := obj.(type) {
	case *corev1.Secret:
		if secretNeedsLiveRefresh(o) {
			// Re-fetch from live to get the full object
			if err := cw.liveClient.Get(ctx, key, obj, opts...); err != nil {
				return err
			}
			// Ensure it becomes tracked for future full-caching (best-effort)
			cw.ensureTrackedLabel(ctx, obj)
		}
	case *corev1.ConfigMap:
		if configmapNeedsLiveRefresh(o) {
			// Re-fetch from live to get the full object
			if err := cw.liveClient.Get(ctx, key, obj, opts...); err != nil {
				return err
			}
			// Ensure it becomes tracked for future full-caching (best-effort)
			cw.ensureTrackedLabel(ctx, obj)
		}

		// add more kinds here (e.g., *corev1.ConfigMap) if you apply similar striping
	}

	return nil
}

// secretNeedsLiveRefresh returns true if the cached secret looks stripped or untracked.
func secretNeedsLiveRefresh(s *corev1.Secret) bool {
	// Untracked → we likely cached a slimmed object; refresh live.
	if !cacheutils.IsTrackedByOperator(s.GetLabels()) {
		return true
	}
	// Heuristic: a "slimmed" secret from our transform has nil Data/StringData.
	// (Note: a legitimately empty secret may also match; adjust if you add a marker label/annotation.)
	if s.Data == nil && s.StringData == nil {
		return true
	}
	return false
}

// configmapNeedsLiveRefresh returns true if the cached cm looks stripped or untracked.
func configmapNeedsLiveRefresh(cm *corev1.ConfigMap) bool {
	if !cacheutils.IsTrackedByOperator(cm.GetLabels()) {
		return true
	}
	if cm.Data == nil && cm.BinaryData == nil {
		return true
	}
	return false
}

// ensureTrackedLabel adds the operator tracking label (best-effort, ignores patch error).
func (cw *ClientWrapper) ensureTrackedLabel(ctx context.Context, obj ctrlclient.Object) {
	if cacheutils.IsTrackedByOperator(obj.GetLabels()) {
		return
	}
	orig := obj.DeepCopyObject().(ctrlclient.Object)

	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string, 1)
	}
	labels[common.ArgoCDTrackedByOperatorLabel] = common.ArgoCDAppName
	obj.SetLabels(labels)

	_ = cw.liveClient.Patch(ctx, obj, ctrlclient.MergeFrom(orig)) // best-effort
}
