package clientwrapper

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/cacheutils"
)

var log = logf.Log.WithName("clientwrapper")

// ClientWrapper wraps a cached client and only falls back to
// live GET when the cached object appears stripped OR is missing required labels.
// We currently override GET only, but could extend to other methods if needed.
type ClientWrapper struct {
	ctrlclient.Client                   // cached client
	liveClient        ctrlclient.Client // direct API client
}

// NewClientWrapper creates a new ClientWrapper instance.
func NewClientWrapper(cached, live ctrlclient.Client) *ClientWrapper {
	return &ClientWrapper{
		Client:     cached,
		liveClient: live,
	}
}

// Get first tries to get from the cached client, and only falls back to live GET
// if the cached object appears stripped or is missing required labels.
// After a live GET, it also ensures the object has the tracking label (best-effort).
func (cw *ClientWrapper) Get(ctx context.Context, key types.NamespacedName, obj ctrlclient.Object, opts ...ctrlclient.GetOption) error {
	if err := cw.Client.Get(ctx, key, obj, opts...); err != nil {
		return err
	}

	switch o := obj.(type) {
	case *corev1.Secret:
		if secretNeedsLiveRefresh(o) {
			if err := cw.liveClient.Get(ctx, key, obj, opts...); err != nil {
				return err
			}

			err := cw.ensureTrackedLabel(ctx, obj)
			if err != nil {
				// non-fatal: a later reconcile will reattempt if this fails
				log.Error(err, "failed to add operator tracking label to object", "name", obj.GetName(), "namespace", obj.GetNamespace())
			}
		}
	case *corev1.ConfigMap:
		if configmapNeedsLiveRefresh(o) {
			if err := cw.liveClient.Get(ctx, key, obj, opts...); err != nil {
				return err
			}

			err := cw.ensureTrackedLabel(ctx, obj)
			if err != nil {
				// non-fatal: a later reconcile will reattempt if this fails
				log.Error(err, "failed to add operator tracking label to object", "name", obj.GetName(), "namespace", obj.GetNamespace())
			}
		}
		// add more kinds here (e.g., *corev1.Deployment) if applying similar striping
	}

	return nil
}

// secretNeedsLiveRefresh returns true if the cached secret looks stripped or untracked.
func secretNeedsLiveRefresh(s *corev1.Secret) bool {
	if !cacheutils.IsTrackedByOperator(s) {
		return true
	}

	// Heuristic: a "stripped" Secret from our transform has nil Data/StringData.
	// A truly empty Secret may also match, but that only triggers an extra live GET,
	// which is rare and acceptable.
	if s.Data == nil && s.StringData == nil {
		return true
	}
	return false
}

// configmapNeedsLiveRefresh returns true if the cached cm looks stripped or untracked.
func configmapNeedsLiveRefresh(cm *corev1.ConfigMap) bool {
	if !cacheutils.IsTrackedByOperator(cm) {
		return true
	}

	// Heuristic: a "stripped" ConfigMap from our transform has nil Data/BinaryData.
	// A truly empty ConfigMap may also match, but that only triggers an extra live GET,
	// which is rare and acceptable.
	if cm.Data == nil && cm.BinaryData == nil {
		return true
	}
	return false
}

// ensureTrackedLabel adds the operator tracking label.
func (cw *ClientWrapper) ensureTrackedLabel(ctx context.Context, obj ctrlclient.Object) error {
	if cacheutils.IsTrackedByOperator(obj) {
		return nil
	}
	orig := obj.DeepCopyObject().(ctrlclient.Object)

	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string, 1)
	}
	labels[common.ArgoCDTrackedByOperatorLabel] = common.ArgoCDAppName
	obj.SetLabels(labels)

	// Best-effort patch to add the operator tracking label.
	// Non-fatal: a later reconcile will reattempt if this fails.
	return cw.liveClient.Patch(ctx, obj, ctrlclient.MergeFrom(orig))
}
